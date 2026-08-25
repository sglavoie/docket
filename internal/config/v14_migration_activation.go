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

const V14ConfigSchemaVersion = 14

type v14MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v14RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v14RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v14MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV14 lifts every task document to schema 13, making the
// optional due-date clock time (due_time) available to every record. Existing
// due dates are date-only, so each migrated task starts with no due_time.
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV14() error {
	return m.migrateWorkspaceToV14(nil)
}

func (m *Manager) migrateWorkspaceToV14(hooks v14MigrationActivationHooks) error {
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
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, LocalDataRoot()})
	if _, recoverErr := commit.RecoverWorkspace(); recoverErr != nil {
		return fmt.Errorf("recover workspace before schema-14 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V13ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV13(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 13: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-13 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema == V14ConfigSchemaVersion {
		return nil
	}
	if schema > V14ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, V14ConfigSchemaVersion)
	}
	if schema != V13ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V14ConfigSchemaVersion)
	}

	_, err = buildV14MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v14 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreDueTimeV14Snapshot(dataDir)
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
		if compareErr := compareV14ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV14MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v14 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V14ConfigSchemaVersion {
		return fmt.Errorf("v14 migration did not publish schema %d after rollback snapshot %s: %w", V14ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV14ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV14ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v14 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v14 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

func buildV14MigrationPlan(dataDir string) (v14MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V13ConfigSchemaVersion)
	if err != nil {
		return v14MigrationPlan{}, err
	}
	plan := v14MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v14MigrationPlan{}, fmt.Errorf("read schema-12 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV12ProjectDocument(string(source))
		if parseErr != nil {
			return v14MigrationPlan{}, fmt.Errorf("parse schema-12 project document %s: %w", path, parseErr)
		}
		lifted := storage.V13ProjectDocumentFromV12(document)
		published, writeErr := storage.WriteV13ProjectDocument(lifted)
		if writeErr != nil {
			return v14MigrationPlan{}, fmt.Errorf("encode schema-13 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V14ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V14ConfigSchemaVersion); validationErr != nil {
		return v14MigrationPlan{}, fmt.Errorf("validate schema-14 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V14ConfigSchemaVersion)
	if err != nil {
		return v14MigrationPlan{}, fmt.Errorf("encode schema-14 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V14ConfigSchemaVersion)
	if err != nil {
		return v14MigrationPlan{}, err
	}
	return plan, nil
}

func createPreDueTimeV14Snapshot(dataDir string) (v14RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v14RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := "pre-due-time-v14-" + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v14RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v14DueTimeAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, "pre-due-time-v14-audit.md")
	if err != nil {
		return v14RollbackSnapshot{}, err
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
		return v14RollbackSnapshot{}, fmt.Errorf("pre-due-time v14 snapshot failed: %w", err)
	}
	return v14RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v14DueTimeAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-due-time schema-14 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary adds the optional task-level `due_time` field. Every migrated task " +
		"keeps its date-only due date and receives no `due_time`, so the rewrite preserves all existing " +
		"task values while making an explicit hh:mm due time available to schema-13 task documents.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 12 -->`; the schema-13 project-document scanner rejects that marker " +
		"outright, so replaying an entry would restore a document the new loader cannot read. This report " +
		"is the only place that can explain the empty history.\n")
	return report.String()
}
