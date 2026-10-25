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

const V19ConfigSchemaVersion = 19

type v19MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v19RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v19RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v19MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV19 lifts every task document to schema 17, which admits the
// optional project-comment collection. No project carries one yet, so the
// boundary rewrites every document into bytes identical to its schema-16 source
// apart from the marker line; what it buys is that the marker and the reader
// agree from this point on.
//
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV19() error {
	return m.migrateWorkspaceToV19(nil)
}

func (m *Manager) migrateWorkspaceToV19(hooks v19MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-19 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V18ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV18(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 18: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-18 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V19ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, SupportedConfigVersion)
	}
	if schema != V18ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V19ConfigSchemaVersion)
	}

	_, err = buildV19MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v19 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreProjectCommentsV19Snapshot(dataDir)
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
		if compareErr := compareV19ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV19MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v19 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V19ConfigSchemaVersion {
		return fmt.Errorf("v19 migration did not publish schema %d after rollback snapshot %s: %w", V19ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV19ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV19ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v19 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v19 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV19MigrationPlan reads and validates every project document as strict
// schema 16 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 17 and the catalog advanced beside it.
//
// Like the schema-18 boundary, this one stages no lens artifact. Schema 17 adds
// a collection that belongs to the project rather than to any task, and no lens
// axis narrows by it, so every saved lens keeps selecting exactly the rows it
// selected before.
func buildV19MigrationPlan(dataDir string) (v19MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V18ConfigSchemaVersion)
	if err != nil {
		return v19MigrationPlan{}, err
	}
	plan := v19MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v19MigrationPlan{}, fmt.Errorf("read schema-16 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV16ProjectDocument(string(source))
		if parseErr != nil {
			return v19MigrationPlan{}, fmt.Errorf("parse schema-16 project document %s: %w", path, parseErr)
		}
		lifted := storage.V17ProjectDocumentFromV16(document)
		published, writeErr := storage.WriteV17ProjectDocument(lifted)
		if writeErr != nil {
			return v19MigrationPlan{}, fmt.Errorf("encode schema-17 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V19ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V19ConfigSchemaVersion); validationErr != nil {
		return v19MigrationPlan{}, fmt.Errorf("validate schema-19 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V19ConfigSchemaVersion)
	if err != nil {
		return v19MigrationPlan{}, fmt.Errorf("encode schema-19 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V19ConfigSchemaVersion)
	if err != nil {
		return v19MigrationPlan{}, err
	}
	return plan, nil
}

func createPreProjectCommentsV19Snapshot(dataDir string) (v19RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v19RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preProjectCommentsV19SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v19RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v19ProjectCommentsAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preProjectCommentsV19SnapshotPrefix+"audit.md")
	if err != nil {
		return v19RollbackSnapshot{}, err
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
		return v19RollbackSnapshot{}, fmt.Errorf("pre-project-comments v19 snapshot failed: %w", err)
	}
	return v19RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v19ProjectCommentsAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-project-comments schema-19 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary advances task Markdown to schema 17, which admits an optional collection " +
		"of project comments: prose that belongs to the project rather than to any one task, stored in an " +
		"escaped `PROJECT-COMMENTS` block between the schema marker and the first task heading. No project " +
		"carries a comment at this boundary, so every document is rewritten byte-identical to its schema-16 " +
		"source apart from the marker line. No task value is dropped, renamed, or reinterpreted here.\n\n")
	report.WriteString("Saved lens definitions are not rewritten. A project comment is prose on a project " +
		"rather than an axis a query narrows by, so every lens keeps selecting exactly the rows it selected " +
		"before, and republishing the collection would change bytes to say nothing.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 16 -->`; the schema-17 project-document scanner rejects that marker outright, " +
		"so replaying an entry would restore a document the new loader cannot read. This report is the only " +
		"place that can explain the empty history.\n\n")
	report.WriteString("Conflict bundles under `.docket/conflicts/` are not rewritten either, here or at any " +
		"boundary. A bundle recorded before this migration keeps its schema-16 documents, so `docket conflict " +
		"show`, `--use remote`, and `--use file` keep working from its metadata while `--use local` and a " +
		"forced re-merge refuse by name: those two need the stored document to parse under the current reader, " +
		"and it no longer does. Nothing removes such a bundle for you.\n")
	return report.String()
}
