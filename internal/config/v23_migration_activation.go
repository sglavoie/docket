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

// V22ConfigSchemaVersion is the catalog schema a workspace enrolled in device
// sync carried before schema 23: schema-21 contents plus the catalog's sync
// block. It was never a step in the migration chain, so nothing migrates a
// workspace to it any longer; an enrolled working copy still holding it is a
// source the schema-23 boundary reads, exactly as it reads schema 21.
const V22ConfigSchemaVersion = 22

const V23ConfigSchemaVersion = 23

type v23MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
}

// v23RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v23RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v23MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV23 lifts every task document to schema 20, which admits the
// stored hidden flag. No existing task carries one, so the boundary rewrites
// every document into bytes identical to its schema-19 source apart from the
// marker line.
//
// It is also the boundary that retires a separate enrolled schema. Both a
// schema-21 workspace and an enrolled schema-22 one activate schema 23, and an
// enrolled one keeps its sync block, which from here on is what alone marks
// enrollment: every Docket older than this one already refuses schema 23 as newer
// than it supports, so a number reserved for enrolled catalogs would buy
// nothing.
//
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV23() error {
	return m.migrateWorkspaceToV23(nil)
}

func (m *Manager) migrateWorkspaceToV23(hooks v23MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-23 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V21ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV21(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 21: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-21 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V23ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, SupportedConfigVersion)
	}
	if schema != V21ConfigSchemaVersion && schema != V22ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V23ConfigSchemaVersion)
	}

	if _, err = buildV23MigrationPlan(dataDir, schema); err != nil {
		return fmt.Errorf("v23 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreHiddenV23Snapshot(dataDir, schema)
	if err != nil {
		return err
	}
	if hooks != nil {
		hooks.didPublishSnapshot(snapshot.archivePath, snapshot.auditPath)
	}
	source := schema
	if commitErr := commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
		live, inventoryErr := inventoryPreScopeV7(dataDir, false)
		if inventoryErr != nil {
			return nil, nil, inventoryErr
		}
		if compareErr := compareV23ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV23MigrationPlan(dataDir, source)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v23 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V23ConfigSchemaVersion {
		return fmt.Errorf("v23 migration did not publish schema %d after rollback snapshot %s: %w", V23ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV23ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV23ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v23 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v23 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV23MigrationPlan reads and validates every project document as strict
// schema 19 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 20 and the catalog advanced beside it. source is the catalog schema the
// workspace holds, 21 or the enrolled 22; both carry schema-19 task Markdown, and
// the enrolled one's sync block is carried across untouched.
//
// The loop walks catalog.Projects with no filter, which is what carries parked
// and archived projects across: both are ordinary catalog entries that a
// read-time predicate hides, and a boundary that consulted that predicate would
// leave unreadable documents behind exactly where nobody would look for them.
//
// This boundary stages no lens artifact: no saved lens can name a hidden clause
// before this schema exists, so there is nothing to rewrite.
func buildV23MigrationPlan(dataDir string, source int) (v23MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, source)
	if err != nil {
		return v23MigrationPlan{}, err
	}
	if validationErr := catalog.validateAtSchema(source); validationErr != nil {
		return v23MigrationPlan{}, fmt.Errorf("validate schema-%d catalog: %w", source, validationErr)
	}
	plan := v23MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		sourceBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return v23MigrationPlan{}, fmt.Errorf("read schema-19 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV19ProjectDocument(string(sourceBytes))
		if parseErr != nil {
			return v23MigrationPlan{}, fmt.Errorf("parse schema-19 project document %s: %w", path, parseErr)
		}
		lifted := storage.V20ProjectDocumentFromV19(document)
		published, writeErr := storage.WriteV20ProjectDocument(lifted)
		if writeErr != nil {
			return v23MigrationPlan{}, fmt.Errorf("encode schema-20 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(sourceBytes)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V23ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V23ConfigSchemaVersion); validationErr != nil {
		return v23MigrationPlan{}, fmt.Errorf("validate schema-23 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V23ConfigSchemaVersion)
	if err != nil {
		return v23MigrationPlan{}, fmt.Errorf("encode schema-23 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V23ConfigSchemaVersion)
	if err != nil {
		return v23MigrationPlan{}, err
	}
	return plan, nil
}

func createPreHiddenV23Snapshot(dataDir string, source int) (v23RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v23RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preHiddenV23SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v23RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v23HiddenAuditReport(createdAt, len(inventory.files), source))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preHiddenV23SnapshotPrefix+"audit.md")
	if err != nil {
		return v23RollbackSnapshot{}, err
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
		return v23RollbackSnapshot{}, fmt.Errorf("pre-hidden v23 snapshot failed: %w", err)
	}
	return v23RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v23HiddenAuditReport(createdAt time.Time, files, source int) string {
	var report strings.Builder
	report.WriteString("# Pre-hidden schema-23 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Source catalog schema: %d\n", source)
	fmt.Fprintf(&report, "- Snapshotted files: %d\n\n", files)
	report.WriteString("This boundary advances task Markdown to schema 20. It adds an optional, strict " +
		"`hidden: true` metadata key to individual task records. Existing tasks are not hidden, so task values " +
		"are preserved and project documents change only at their schema marker.\n\n")
	if source == V22ConfigSchemaVersion {
		report.WriteString("This workspace was enrolled in sync at catalog schema 22. Its sync block is carried " +
			"across unchanged; from schema 23 the sync block alone marks enrollment.\n\n")
	}
	report.WriteString("Saved lens definitions are not rewritten because no lens could name the hidden flag " +
		"before this schema existed.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw Markdown " +
		"carrying `<!-- tt-schema: 19 -->`; the schema-20 project-document scanner rejects that marker outright, " +
		"so replaying an entry would restore a document the new loader cannot read. This report is the only " +
		"place that can explain the empty history.\n\n")
	report.WriteString("Conflict bundles under `.docket/conflicts/` are not rewritten either, here or at any " +
		"boundary. A bundle recorded before this migration keeps its schema-19 documents, so `docket conflict " +
		"show`, `--use remote`, and `--use file` keep working from its metadata while `--use local` and a " +
		"forced re-merge refuse by name: those two need the stored document to parse under the current reader, " +
		"and it no longer does. Nothing removes such a bundle for you.\n")
	return report.String()
}
