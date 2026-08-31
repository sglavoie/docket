package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

const V12ConfigSchemaVersion = 12

type v12MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v12RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v12RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v12MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV12 lifts every task document to schema 11, adding an empty
// task-level context to every existing record. Catalog and project documents
// publish in a single transaction after a byte-exact rollback snapshot is
// durable.
func (m *Manager) MigrateWorkspaceToV12() error {
	return m.migrateWorkspaceToV12(nil)
}

func (m *Manager) migrateWorkspaceToV12(hooks v12MigrationActivationHooks) error {
	if m == nil {
		return fmt.Errorf("configuration manager is required")
	}
	dataDir, err := validateV5DryRunRoot(m.workspaceDataDir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, m.localDataRoot()}).OnMachine(m.machineID())
	if _, recoverErr := commit.RecoverWorkspace(); recoverErr != nil {
		return fmt.Errorf("recover workspace before schema-12 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V11ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV11(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 11: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-11 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V12ConfigSchemaVersion {
		if schema <= CurrentConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, CurrentConfigVersion)
	}
	if schema != V11ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V12ConfigSchemaVersion)
	}

	_, err = buildV12MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v12 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreContextV12Snapshot(dataDir)
	if err != nil {
		return err
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot.archivePath, snapshot.auditPath)
	}
	if commitErr := commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		live, inventoryErr := inventoryPreScopeV7(dataDir, false)
		if inventoryErr != nil {
			return nil, nil, inventoryErr
		}
		if compareErr := compareV12ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV12MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v12 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V12ConfigSchemaVersion {
		return fmt.Errorf("v12 migration did not publish schema %d after rollback snapshot %s: %w", V12ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV12ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV12ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v12 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v12 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

func buildV12MigrationPlan(dataDir string) (v12MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V11ConfigSchemaVersion)
	if err != nil {
		return v12MigrationPlan{}, err
	}
	plan := v12MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v12MigrationPlan{}, fmt.Errorf("read schema-10 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV10ProjectDocument(string(source))
		if parseErr != nil {
			return v12MigrationPlan{}, fmt.Errorf("parse schema-10 project document %s: %w", path, parseErr)
		}
		lifted := storage.V11ProjectDocumentFromV10(document)
		published, writeErr := storage.WriteV11ProjectDocument(lifted)
		if writeErr != nil {
			return v12MigrationPlan{}, fmt.Errorf("encode schema-11 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V12ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V12ConfigSchemaVersion); validationErr != nil {
		return v12MigrationPlan{}, fmt.Errorf("validate schema-12 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V12ConfigSchemaVersion)
	if err != nil {
		return v12MigrationPlan{}, fmt.Errorf("encode schema-12 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V12ConfigSchemaVersion)
	if err != nil {
		return v12MigrationPlan{}, err
	}
	return plan, nil
}

func createPreContextV12Snapshot(dataDir string) (v12RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v12RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preContextV12SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v12RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v12ContextAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preContextV12SnapshotPrefix+"audit.md")
	if err != nil {
		return v12RollbackSnapshot{}, err
	}
	// The archive owns the bytes now; the activation guard needs only the
	// fingerprints, so releasing them keeps the whole workspace from being held
	// in memory across the commit.
	for index := range inventory.files {
		inventory.files[index].data = nil
	}
	absent := ""
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir})
	_, err = commit.CommitArtifacts([]storage.ArtifactChange{
		{Path: archivePath, Data: archive, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
		{Path: auditPath, Data: audit, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
	})
	if err != nil {
		return v12RollbackSnapshot{}, fmt.Errorf("pre-context v12 snapshot failed: %w", err)
	}
	return v12RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v12ContextAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-context schema-12 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary adds the task-level `context` field. Every migrated task receives " +
		"an empty context, so the rewrite preserves all existing task values while making the new field " +
		"available to schema-11 task documents.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 10 -->`; the schema-11 project-document scanner rejects that marker " +
		"outright, so replaying an entry would restore a document the new loader cannot read. This report " +
		"is the only place that can explain the empty history.\n")
	return report.String()
}
