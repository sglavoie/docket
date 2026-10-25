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

const V18ConfigSchemaVersion = 18

type v18MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v18RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v18RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v18MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV18 lifts every task document to schema 16, which admits the
// optional retained resume note. No task carries one yet, so the boundary rewrites
// every document into bytes identical to its schema-15 source apart from the
// marker line; what it buys is that the marker and the reader agree from this
// point on.
//
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV18() error {
	return m.migrateWorkspaceToV18(nil)
}

func (m *Manager) migrateWorkspaceToV18(hooks v18MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-18 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V17ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV17(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 17: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-17 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V18ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, SupportedConfigVersion)
	}
	if schema != V17ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V18ConfigSchemaVersion)
	}

	_, err = buildV18MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v18 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreResumeNoteV18Snapshot(dataDir)
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
		if compareErr := compareV18ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV18MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v18 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V18ConfigSchemaVersion {
		return fmt.Errorf("v18 migration did not publish schema %d after rollback snapshot %s: %w", V18ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV18ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV18ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v18 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v18 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV18MigrationPlan reads and validates every project document as strict
// schema 15 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 16 and the catalog advanced beside it.
//
// Unlike the schema-17 boundary, this one stages no lens artifact. Schema 16 adds
// a field no lens axis can name — the note is prose on one task, not a value a
// query narrows by — so every saved lens keeps selecting exactly the rows it
// selected before, and rewriting the collection would churn bytes to say nothing.
func buildV18MigrationPlan(dataDir string) (v18MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V17ConfigSchemaVersion)
	if err != nil {
		return v18MigrationPlan{}, err
	}
	plan := v18MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v18MigrationPlan{}, fmt.Errorf("read schema-15 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV15ProjectDocument(string(source))
		if parseErr != nil {
			return v18MigrationPlan{}, fmt.Errorf("parse schema-15 project document %s: %w", path, parseErr)
		}
		lifted := storage.V16ProjectDocumentFromV15(document)
		published, writeErr := storage.WriteV16ProjectDocument(lifted)
		if writeErr != nil {
			return v18MigrationPlan{}, fmt.Errorf("encode schema-16 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V18ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V18ConfigSchemaVersion); validationErr != nil {
		return v18MigrationPlan{}, fmt.Errorf("validate schema-18 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V18ConfigSchemaVersion)
	if err != nil {
		return v18MigrationPlan{}, fmt.Errorf("encode schema-18 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V18ConfigSchemaVersion)
	if err != nil {
		return v18MigrationPlan{}, err
	}
	return plan, nil
}

func createPreResumeNoteV18Snapshot(dataDir string) (v18RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v18RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preResumeNoteV18SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v18RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v18ResumeNoteAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preResumeNoteV18SnapshotPrefix+"audit.md")
	if err != nil {
		return v18RollbackSnapshot{}, err
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
		return v18RollbackSnapshot{}, fmt.Errorf("pre-resume-note v18 snapshot failed: %w", err)
	}
	return v18RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v18ResumeNoteAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-resume-note schema-18 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary advances task Markdown to schema 16, which admits one optional retained " +
		"resume note per task: a single line of at most 240 characters answering \"where was I\", written as an " +
		"escaped `RESUME` block ahead of the description and kept until it is explicitly cleared. No task carries " +
		"a note at this boundary, so every document is rewritten byte-identical to its schema-15 source apart " +
		"from the marker line. Nothing is dropped, renamed, or reinterpreted here.\n\n")
	report.WriteString("Saved lens definitions are not rewritten. The note is prose on one task rather than an " +
		"axis a query narrows by, so every lens keeps selecting exactly the rows it selected before, and " +
		"republishing the collection would change bytes to say nothing.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 15 -->`; the schema-16 project-document scanner rejects that marker outright, " +
		"so replaying an entry would restore a document the new loader cannot read. This report is the only " +
		"place that can explain the empty history.\n")
	return report.String()
}
