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

const V17ConfigSchemaVersion = 17

type v17MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v17RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v17RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v17MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV17 lifts every task document to schema 15, moving review
// off the tag list and onto a stored flag of its own. Review was never a
// lifecycle state, but spelling it as an ordinary task tag made a promoted lane
// depend on a value any user could mint, misspell, or clear by rewriting an
// unrelated tag. Schema 15 gives it a boolean beside the pin.
//
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV17() error {
	return m.migrateWorkspaceToV17(nil)
}

func (m *Manager) migrateWorkspaceToV17(hooks v17MigrationActivationHooks) error {
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
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, m.localDataRoot()})
	if _, recoverErr := commit.RecoverWorkspace(); recoverErr != nil {
		return fmt.Errorf("recover workspace before schema-17 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V16ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV16(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 16: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-16 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V17ConfigSchemaVersion {
		if schema <= CurrentConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, CurrentConfigVersion)
	}
	if schema != V16ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V17ConfigSchemaVersion)
	}

	_, err = buildV17MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v17 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreStoredReviewV17Snapshot(dataDir)
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
		if compareErr := compareV17ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV17MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v17 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V17ConfigSchemaVersion {
		return fmt.Errorf("v17 migration did not publish schema %d after rollback snapshot %s: %w", V17ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV17ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV17ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v17 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v17 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV17MigrationPlan reads and validates every project document as strict
// schema 14 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 15 and the catalog advanced beside it.
func buildV17MigrationPlan(dataDir string) (v17MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V16ConfigSchemaVersion)
	if err != nil {
		return v17MigrationPlan{}, err
	}
	plan := v17MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v17MigrationPlan{}, fmt.Errorf("read schema-14 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV14ProjectDocument(string(source))
		if parseErr != nil {
			return v17MigrationPlan{}, fmt.Errorf("parse schema-14 project document %s: %w", path, parseErr)
		}
		lifted := storage.V15ProjectDocumentFromV14(document)
		published, writeErr := storage.WriteV15ProjectDocument(lifted)
		if writeErr != nil {
			return v17MigrationPlan{}, fmt.Errorf("encode schema-15 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V17ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V17ConfigSchemaVersion); validationErr != nil {
		return v17MigrationPlan{}, fmt.Errorf("validate schema-17 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V17ConfigSchemaVersion)
	if err != nil {
		return v17MigrationPlan{}, fmt.Errorf("encode schema-17 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V17ConfigSchemaVersion)
	if err != nil {
		return v17MigrationPlan{}, err
	}
	return plan, nil
}

func createPreStoredReviewV17Snapshot(dataDir string) (v17RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v17RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := "pre-stored-review-v17-" + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v17RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v17StoredReviewAuditReport(createdAt, len(inventory.files)))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, "pre-stored-review-v17-audit.md")
	if err != nil {
		return v17RollbackSnapshot{}, err
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
		return v17RollbackSnapshot{}, fmt.Errorf("pre-stored-review v17 snapshot failed: %w", err)
	}
	return v17RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v17StoredReviewAuditReport(createdAt time.Time, files int) string {
	var report strings.Builder
	report.WriteString("# Pre-stored-review schema-17 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary advances task Markdown to schema 15, which lifts review off the tag list " +
		"and onto a stored boolean of its own. A task carrying the `review` tag before this migration carries " +
		"`review: true` after it, and that tag is removed as it is lifted: leaving both would make the workspace " +
		"hold two answers to one question, and the next edit that rewrote the tag list would silently disagree " +
		"with the lane. Every other tag is preserved in place, and a task that was not up for review is written " +
		"back byte-identical, because the new key is omitted when the flag is off.\n\n")
	report.WriteString("Review remains what it always was: not a lifecycle state. It is now an independent axis " +
		"beside the pin, spelled `=review` in the capture, edit, outline, and search grammars and reached by the " +
		"`f=` filter facet.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 14 -->`; the schema-15 project-document scanner rejects that marker outright, " +
		"so replaying an entry would restore a document the new loader cannot read. This report is the only " +
		"place that can explain the empty history.\n")
	return report.String()
}
