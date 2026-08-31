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

const V15ConfigSchemaVersion = 15

type v15MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v15RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v15RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v15MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV15 lifts every task document to schema 14, making the
// optional soft-dependency list (depends) available to every record. No existing
// dependency data survives from any retired field, so each migrated task starts
// with an empty list, which is exactly an omitted depends key. Catalog and
// project documents publish in a single transaction after a byte-exact rollback
// snapshot is durable.
func (m *Manager) MigrateWorkspaceToV15() error {
	return m.migrateWorkspaceToV15(nil)
}

func (m *Manager) migrateWorkspaceToV15(hooks v15MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-15 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V14ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV14(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 14: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-14 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V15ConfigSchemaVersion {
		if schema <= CurrentConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, CurrentConfigVersion)
	}
	if schema != V14ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V15ConfigSchemaVersion)
	}

	_, err = buildV15MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v15 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreDependenciesV15Snapshot(dataDir)
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
		if compareErr := compareV15ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV15MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v15 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V15ConfigSchemaVersion {
		return fmt.Errorf("v15 migration did not publish schema %d after rollback snapshot %s: %w", V15ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV15ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV15ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v15 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v15 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

func buildV15MigrationPlan(dataDir string) (v15MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V14ConfigSchemaVersion)
	if err != nil {
		return v15MigrationPlan{}, err
	}
	plan := v15MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v15MigrationPlan{}, fmt.Errorf("read schema-13 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV13ProjectDocument(string(source))
		if parseErr != nil {
			return v15MigrationPlan{}, fmt.Errorf("parse schema-13 project document %s: %w", path, parseErr)
		}
		lifted := storage.V14ProjectDocumentFromV13(document)
		published, writeErr := storage.WriteV14ProjectDocument(lifted)
		if writeErr != nil {
			return v15MigrationPlan{}, fmt.Errorf("encode schema-14 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V15ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V15ConfigSchemaVersion); validationErr != nil {
		return v15MigrationPlan{}, fmt.Errorf("validate schema-15 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V15ConfigSchemaVersion)
	if err != nil {
		return v15MigrationPlan{}, fmt.Errorf("encode schema-15 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V15ConfigSchemaVersion)
	if err != nil {
		return v15MigrationPlan{}, err
	}
	return plan, nil
}

func createPreDependenciesV15Snapshot(dataDir string) (v15RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v15RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preDependenciesV15SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v15RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v15DependenciesAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preDependenciesV15SnapshotPrefix+"audit.md")
	if err != nil {
		return v15RollbackSnapshot{}, err
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
		return v15RollbackSnapshot{}, fmt.Errorf("pre-dependencies v15 snapshot failed: %w", err)
	}
	return v15RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v15DependenciesAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-dependencies schema-15 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary adds the optional task-level `depends` field: an ordered list of soft, " +
		"project-qualified task references. Every migrated task receives an empty list, represented " +
		"canonically by omitting the field, so the rewrite preserves all existing task values while making " +
		"the signal available to schema-14 task documents. Dependencies retired before this boundary are not " +
		"recovered; no earlier snapshot is scanned for them.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 13 -->`; the schema-14 project-document scanner rejects that marker " +
		"outright, so replaying an entry would restore a document the new loader cannot read. This report " +
		"is the only place that can explain the empty history.\n")
	return report.String()
}
