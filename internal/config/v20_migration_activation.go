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

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

const V20ConfigSchemaVersion = 20

type v20MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v20RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v20RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v20MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV20 lifts every task document to schema 18, which admits the
// optional per-task checklist origin. No existing project carries one yet, so the
// boundary rewrites every document into bytes identical to its schema-17 source
// apart from the marker line; what it buys is that the marker and the reader
// agree from this point on.
//
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV20() error {
	return m.migrateWorkspaceToV20(nil)
}

func (m *Manager) migrateWorkspaceToV20(hooks v20MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-20 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V19ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV19(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 19: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-19 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V20ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, SupportedConfigVersion)
	}
	if schema != V19ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V20ConfigSchemaVersion)
	}

	_, err = buildV20MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v20 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreChecklistOriginV20Snapshot(dataDir)
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
		if compareErr := compareV20ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV20MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v20 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V20ConfigSchemaVersion {
		return fmt.Errorf("v20 migration did not publish schema %d after rollback snapshot %s: %w", V20ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV20ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV20ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v20 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v20 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV20MigrationPlan reads and validates every project document as strict
// schema 17 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 18 and the catalog advanced beside it.
//
// This boundary stages no lens artifact: origin is placement metadata for a
// later reverse conversion, and no query axis narrows by it.
func buildV20MigrationPlan(dataDir string) (v20MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V19ConfigSchemaVersion)
	if err != nil {
		return v20MigrationPlan{}, err
	}
	plan := v20MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v20MigrationPlan{}, fmt.Errorf("read schema-17 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV17ProjectDocument(string(source))
		if parseErr != nil {
			return v20MigrationPlan{}, fmt.Errorf("parse schema-17 project document %s: %w", path, parseErr)
		}
		lifted := storage.V18ProjectDocumentFromV17(document)
		published, writeErr := storage.WriteV18ProjectDocument(lifted)
		if writeErr != nil {
			return v20MigrationPlan{}, fmt.Errorf("encode schema-18 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V20ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V20ConfigSchemaVersion); validationErr != nil {
		return v20MigrationPlan{}, fmt.Errorf("validate schema-20 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V20ConfigSchemaVersion)
	if err != nil {
		return v20MigrationPlan{}, fmt.Errorf("encode schema-20 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V20ConfigSchemaVersion)
	if err != nil {
		return v20MigrationPlan{}, err
	}
	return plan, nil
}

func createPreChecklistOriginV20Snapshot(dataDir string) (v20RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v20RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preChecklistOriginV20SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v20RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v20ChecklistOriginAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preChecklistOriginV20SnapshotPrefix+"audit.md")
	if err != nil {
		return v20RollbackSnapshot{}, err
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
		return v20RollbackSnapshot{}, fmt.Errorf("pre-checklist-origins v20 snapshot failed: %w", err)
	}
	return v20RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v20ChecklistOriginAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-checklist-origin schema-20 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary advances task Markdown to schema 18. It adds an optional, strict " +
		"checklist-origin line to individual task records. Existing tasks have no origin, so task values " +
		"are preserved and project documents change only at their schema marker.\n\n")
	report.WriteString("Saved lens definitions are not rewritten because checklist origin does not change query matching.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 17 -->`; the schema-18 project-document scanner rejects that marker outright, " +
		"so replaying an entry would restore a document the new loader cannot read. This report is the only " +
		"place that can explain the empty history.\n\n")
	report.WriteString("Conflict bundles under `.tt/conflicts/` are not rewritten either, here or at any " +
		"boundary. A bundle recorded before this migration keeps its schema-17 documents, so `tt conflict " +
		"show`, `--use remote`, and `--use file` keep working from its metadata while `--use local` and a " +
		"forced re-merge refuse by name: those two need the stored document to parse under the current reader, " +
		"and it no longer does. Nothing removes such a bundle for you.\n")
	return report.String()
}
