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
	"github.com/sglavoie/docket/internal/util"
)

// V26ConfigSchemaVersion is the catalog schema whose task documents are schema
// 21: the someday lifecycle state is retired and the work it held is Backlog.
// The catalog itself changes at its number alone; advancing it is what makes an
// older Docket refuse the workspace as newer than it supports instead of
// refusing each document as malformed.
const V26ConfigSchemaVersion = 26

// v26ActivationLockName is the lock one activating process holds from before
// its snapshot until the schema-26 catalog is published.
const v26ActivationLockName = "v26-activation.lock"

type v26MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
	retired   int
}

// v26RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v26RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v26MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV26 lifts every task document to schema 21, which retires
// the someday state: each task schema 20 stored as someday is Backlog, and no
// other value moves. Catalog and project documents publish in a single
// transaction after a byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV26() error {
	return m.migrateWorkspaceToV26(nil)
}

func (m *Manager) migrateWorkspaceToV26(hooks v26MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-26 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V25ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV25(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 25: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-25 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V26ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade Docket", schema, SupportedConfigVersion)
	}
	if schema != V25ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V26ConfigSchemaVersion)
	}

	plan, err := buildV26MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v26 migration validation failed; workspace was not changed: %w", err)
	}
	return withV26ActivationLock(dataDir, func() error {
		// A concurrent opener may have activated the boundary while this one
		// waited for the lock. Its archive and catalog are this one's too, and a
		// second archive minted in the same second would collide with its name.
		if current, _, schemaErr := workspaceCatalogPayloadSchema(dataDir); schemaErr == nil && current == V26ConfigSchemaVersion {
			return nil
		}
		snapshot, snapshotErr := createPreRetireSomedayV26Snapshot(dataDir, plan.retired)
		if snapshotErr != nil {
			return snapshotErr
		}
		if hooks != nil {
			hooks.didPublishSnapshot(snapshot.archivePath, snapshot.auditPath)
		}
		if commitErr := commit.CommitPlannedOpsWithArtifacts(func() ([]transaction.Op, []storage.ArtifactChange, error) {
			live, inventoryErr := inventoryPreScopeV7(dataDir, false)
			if inventoryErr != nil {
				return nil, nil, inventoryErr
			}
			if compareErr := compareV26ArchivedInventory(snapshot.inventory, live); compareErr != nil {
				return nil, nil, compareErr
			}
			current, planErr := buildV26MigrationPlan(dataDir)
			if planErr != nil {
				return nil, nil, planErr
			}
			return current.cleanup, current.artifacts, nil
		}); commitErr != nil {
			return fmt.Errorf("v26 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
		}
		published, publishedExists, publishedErr := workspaceCatalogPayloadSchema(dataDir)
		if publishedErr != nil || !publishedExists || published != V26ConfigSchemaVersion {
			return fmt.Errorf("v26 migration did not publish schema %d after rollback snapshot %s: %w", V26ConfigSchemaVersion, snapshot.archivePath, publishedErr)
		}
		return nil
	})
}

// withV26ActivationLock serializes the schema-26 activation across processes
// from before its snapshot until its catalog is published, as the schema-25
// boundary's lock does.
func withV26ActivationLock(dataDir string, fn func() error) error {
	if err := os.MkdirAll(TransactionsDir(dataDir), DirPermissions); err != nil {
		return fmt.Errorf("prepare %s: %w", TransactionsDir(dataDir), err)
	}
	return util.WithFileLock(filepath.Join(TransactionsDir(dataDir), v26ActivationLockName), true, fn)
}

// compareV26ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds.
func compareV26ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v26 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v26 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV26MigrationPlan reads and validates every project document as strict
// schema 20 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 21 — someday becoming Backlog — and the catalog advanced beside it.
//
// The loop walks catalog.Projects with no filter, which is what carries parked
// and archived projects across: a document left behind is one the new reader
// rejects.
//
// Planning is not touched. A someday task holds no Today or Next entry in the
// ordinary course, since leaving Doing for any other state clears both; one a
// merge or a hand edit left planned reads afterwards as a planned Backlog task,
// which is exactly what `wt` or `wn` make. Saved lenses are not rewritten either:
// a query naming `$someday` is refused with guidance naming `$backlog` when the
// lens is opened, rather than silently widened to the work Backlog already held.
func buildV26MigrationPlan(dataDir string) (v26MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V25ConfigSchemaVersion)
	if err != nil {
		return v26MigrationPlan{}, err
	}
	if validationErr := catalog.validateAtSchema(V25ConfigSchemaVersion); validationErr != nil {
		return v26MigrationPlan{}, fmt.Errorf("validate schema-25 catalog: %w", validationErr)
	}
	plan := v26MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		sourceBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return v26MigrationPlan{}, fmt.Errorf("read schema-20 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV20ProjectDocument(string(sourceBytes))
		if parseErr != nil {
			return v26MigrationPlan{}, fmt.Errorf("parse schema-20 project document %s: %w", path, parseErr)
		}
		lifted, retired := storage.V21ProjectDocumentFromV20(document)
		plan.retired += retired
		published, writeErr := storage.WriteV21ProjectDocument(lifted)
		if writeErr != nil {
			return v26MigrationPlan{}, fmt.Errorf("encode schema-21 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(sourceBytes)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V26ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V26ConfigSchemaVersion); validationErr != nil {
		return v26MigrationPlan{}, fmt.Errorf("validate schema-26 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V26ConfigSchemaVersion)
	if err != nil {
		return v26MigrationPlan{}, fmt.Errorf("encode schema-26 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V26ConfigSchemaVersion)
	if err != nil {
		return v26MigrationPlan{}, err
	}
	return plan, nil
}

func createPreRetireSomedayV26Snapshot(dataDir string, retired int) (v26RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v26RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preRetireSomedayV26SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v26RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v26RetireSomedayAuditReport(createdAt, len(inventory.files), retired))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preRetireSomedayV26SnapshotPrefix+"audit.md")
	if err != nil {
		return v26RollbackSnapshot{}, err
	}
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
		return v26RollbackSnapshot{}, fmt.Errorf("pre-retire-someday v26 snapshot failed: %w", err)
	}
	return v26RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v26RetireSomedayAuditReport(createdAt time.Time, files, retired int) string {
	var report strings.Builder
	report.WriteString("# Pre-retire-someday schema-26 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Source catalog schema: %d\n", V25ConfigSchemaVersion)
	fmt.Fprintf(&report, "- Snapshotted files: %d\n", files)
	fmt.Fprintf(&report, "- Tasks moved from someday to backlog: %d\n\n", retired)
	report.WriteString("This boundary advances task Markdown to schema 21, which retires the `someday` " +
		"lifecycle state. Every task stored as `state: someday` is rewritten as `state: backlog`; its " +
		"`state_changed`, `updated`, and every other value are carried byte for byte, and planning is not " +
		"touched. The catalog changes at its schema number alone.\n\n")
	report.WriteString("The `.tt/` tombstone keeps declaring schema 24. It only has to be newer than every " +
		"Docket that still reads `.tt/`, and those all stop at schema 23.\n\n")
	report.WriteString("Saved lens definitions are not rewritten. A lens whose query names `$someday` opens " +
		"with guidance naming `$backlog` instead of silently widening to work Backlog already held.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary. Ledger entries store raw schema-20 " +
		"Markdown, which the schema-21 reader rejects by its marker, so replaying one would restore a document " +
		"the new loader cannot read. This report is the only place that can explain the empty history.\n\n")
	report.WriteString("Conflict bundles under `.docket/conflicts/` are not rewritten, here or at any " +
		"boundary. A bundle recorded before this migration keeps its schema-20 documents, so `--use local` " +
		"and a forced re-merge refuse by name while `show`, `--use remote`, and `--use file` keep working.\n")
	return report.String()
}
