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

const V21ConfigSchemaVersion = 21

type v21MigrationPlan struct {
	artifacts []storage.ArtifactChange
	cleanup   []transaction.Op
	// seeded counts the Waiting tasks that received a check-in, so the audit
	// report can state how much of the workspace this boundary actually changed
	// rather than claiming a rewrite it may not have performed.
	seeded int
}

// v21RollbackSnapshot is one published pre-migration archive together with the
// fingerprint of the exact workspace bytes it contains.
type v21RollbackSnapshot struct {
	archivePath string
	auditPath   string
	inventory   *preScopeV7Inventory
}

type v21MigrationActivationHooks interface {
	didPublishSnapshot(archivePath, auditPath string)
}

// MigrateWorkspaceToV21 lifts every task document to schema 19, which admits the
// per-task check-in schedule and the instant a task was last reviewed.
//
// Unlike the last several boundaries this one is not the marker line alone: a
// Waiting task that already carries a deadline is given a one-off check-in on
// that same day, because that deadline is the day the wait was always meant to
// be looked at again. Everything else about every task is preserved, and no
// review instant is invented.
//
// Catalog and project documents publish in a single transaction after a
// byte-exact rollback snapshot is durable.
func (m *Manager) MigrateWorkspaceToV21() error {
	return m.migrateWorkspaceToV21(nil)
}

func (m *Manager) migrateWorkspaceToV21(hooks v21MigrationActivationHooks) error {
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
		return fmt.Errorf("recover workspace before schema-21 activation: %w", recoverErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return err
	}
	if schema < V20ConfigSchemaVersion {
		if migrateErr := m.MigrateWorkspaceToV20(); migrateErr != nil {
			return fmt.Errorf("migrate supported workspace through schema 20: %w", migrateErr)
		}
		schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
		if err != nil || !exists {
			return fmt.Errorf("schema-20 migration did not publish a workspace catalog: %w", err)
		}
	}
	if schema >= V21ConfigSchemaVersion {
		if schema <= SupportedConfigVersion {
			return nil
		}
		return fmt.Errorf("workspace schema %d is newer than supported schema %d; upgrade tt", schema, SupportedConfigVersion)
	}
	if schema != V20ConfigSchemaVersion {
		return fmt.Errorf("workspace schema %d cannot be migrated directly to schema %d", schema, V21ConfigSchemaVersion)
	}

	dryRun, err := buildV21MigrationPlan(dataDir)
	if err != nil {
		return fmt.Errorf("v21 migration validation failed; workspace was not changed: %w", err)
	}
	snapshot, err := createPreCheckInsV21Snapshot(dataDir, dryRun.seeded)
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
		if compareErr := compareV21ArchivedInventory(snapshot.inventory, live); compareErr != nil {
			return nil, nil, compareErr
		}
		current, planErr := buildV21MigrationPlan(dataDir)
		if planErr != nil {
			return nil, nil, planErr
		}
		return current.cleanup, current.artifacts, nil
	}); commitErr != nil {
		return fmt.Errorf("v21 migration failed after creating rollback snapshot %s and audit %s: %w", snapshot.archivePath, snapshot.auditPath, commitErr)
	}
	schema, exists, err = workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists || schema != V21ConfigSchemaVersion {
		return fmt.Errorf("v21 migration did not publish schema %d after rollback snapshot %s: %w", V21ConfigSchemaVersion, snapshot.archivePath, err)
	}
	return nil
}

// compareV21ArchivedInventory refuses activation whenever the live workspace no
// longer matches the exact bytes the rollback archive holds. Without it a write
// landing after snapshot publication is migrated and activated with no
// recoverable pre-migration copy of itself.
func compareV21ArchivedInventory(archived, live *preScopeV7Inventory) error {
	if len(archived.files) != len(live.files) {
		return fmt.Errorf("v21 migration aborted: the workspace changed after the rollback snapshot was published")
	}
	for index := range archived.files {
		expected, found := archived.files[index], live.files[index]
		if expected.Path != found.Path || expected.sourcePath != found.sourcePath ||
			expected.Size != found.Size || expected.SHA256 != found.SHA256 {
			return fmt.Errorf(
				"v21 migration aborted: %s changed after the rollback snapshot was published",
				expected.sourcePath,
			)
		}
	}
	return nil
}

// buildV21MigrationPlan reads and validates every project document as strict
// schema 18 before it plans a single byte, so a workspace this build cannot read
// is refused rather than half-migrated. Only then is each document lifted to
// schema 19, seeded, and the catalog advanced beside it.
//
// The loop walks catalog.Projects with no filter, which is what carries parked
// and archived projects across: both are ordinary catalog entries that a read-time
// predicate hides, and a boundary that consulted that predicate would leave
// unreadable documents behind exactly where nobody would look for them.
//
// This boundary stages no lens artifact: a check-in is a stored schedule, and no
// saved query axis narrows by one.
func buildV21MigrationPlan(dataDir string) (v21MigrationPlan, error) {
	catalogPath := WorkspaceCatalogPath(dataDir)
	sourceCatalogBytes, catalog, err := readMigrationSourceCatalog(catalogPath, V20ConfigSchemaVersion)
	if err != nil {
		return v21MigrationPlan{}, err
	}
	plan := v21MigrationPlan{}
	for projectIndex := range catalog.Projects {
		project := &catalog.Projects[projectIndex]
		path := storage.TasksFilePath(project.CatalogPath(dataDir))
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return v21MigrationPlan{}, fmt.Errorf("read schema-18 project document %s: %w", path, readErr)
		}
		document, parseErr := storage.ParseV18ProjectDocument(string(source))
		if parseErr != nil {
			return v21MigrationPlan{}, fmt.Errorf("parse schema-18 project document %s: %w", path, parseErr)
		}
		lifted := storage.SeedWaitingCheckInsV19(storage.V19ProjectDocumentFromV18(document))
		plan.seeded += countV21SeededCheckIns(lifted)
		published, writeErr := storage.WriteV19ProjectDocument(lifted)
		if writeErr != nil {
			return v21MigrationPlan{}, fmt.Errorf("encode schema-19 project document %s: %w", path, writeErr)
		}
		revision := storage.RawArtifactRevision(source)
		plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
			Path: path, Data: []byte(published), Mode: FilePermissions, ExpectedRevision: &revision,
		})
	}
	catalog.SchemaVersion = V21ConfigSchemaVersion
	if validationErr := catalog.validateAtSchema(V21ConfigSchemaVersion); validationErr != nil {
		return v21MigrationPlan{}, fmt.Errorf("validate schema-21 catalog: %w", validationErr)
	}
	publishedCatalog, err := encodeMigrationCatalogEnvelope(&catalog, V21ConfigSchemaVersion)
	if err != nil {
		return v21MigrationPlan{}, fmt.Errorf("encode schema-21 catalog: %w", err)
	}
	catalogRevision := storage.RawArtifactRevision(sourceCatalogBytes)
	plan.artifacts = append(plan.artifacts, storage.ArtifactChange{
		Path: catalogPath, Data: publishedCatalog, Mode: FilePermissions, ExpectedRevision: &catalogRevision,
	})
	sort.Slice(plan.artifacts, func(i, j int) bool { return plan.artifacts[i].Path < plan.artifacts[j].Path })
	// Every schema boundary that changes retained artifact bytes drops the whole
	// undo ledger: replaying an entry would restore a document the new loader
	// rejects.
	plan.cleanup, err = undoHistoryCleanupOps(UndoLedgerDir(dataDir), V21ConfigSchemaVersion)
	if err != nil {
		return v21MigrationPlan{}, err
	}
	return plan, nil
}

// countV21SeededCheckIns counts the check-ins a migrated document carries. At
// this boundary every one of them was minted by the seeding pass, because the
// schema-18 source had no key to store one in.
func countV21SeededCheckIns(document storage.V19ProjectDocument) int {
	count := 0
	for _, task := range document.Tasks {
		if task.NextCheckIn != "" {
			count++
		}
	}
	return count
}

func createPreCheckInsV21Snapshot(dataDir string, seeded int) (v21RollbackSnapshot, error) {
	inventory, err := inventoryPreScopeV7(dataDir, true)
	if err != nil {
		return v21RollbackSnapshot{}, err
	}
	createdAt := time.Now().UTC().Truncate(time.Second)
	stem := preCheckInsV21SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(dataDir), stem+preScopeV7AuditSuffix)
	if precheckErr := precheckPreScopeV7Targets(archivePath, auditPath); precheckErr != nil {
		return v21RollbackSnapshot{}, precheckErr
	}
	audit := []byte(v21CheckInsAuditReport(createdAt, len(inventory.files), seeded))
	archive, err := buildWorkspaceSnapshotArchive(inventory.files, audit, preCheckInsV21SnapshotPrefix+"audit.md")
	if err != nil {
		return v21RollbackSnapshot{}, err
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
		return v21RollbackSnapshot{}, fmt.Errorf("pre-check-ins v21 snapshot failed: %w", err)
	}
	return v21RollbackSnapshot{archivePath: archivePath, auditPath: auditPath, inventory: inventory}, nil
}

func v21CheckInsAuditReport(createdAt time.Time, files, seeded int) string {
	var report strings.Builder
	report.WriteString("# Pre-check-ins schema-21 migration audit\n\n")
	fmt.Fprintf(&report, "- Created at: `%s`\n", createdAt.Format(time.RFC3339))
	fmt.Fprintf(&report, "- Snapshotted files: %d\n", files)
	fmt.Fprintf(&report, "- Waiting tasks seeded with a check-in: %d\n\n", seeded)
	report.WriteString("This boundary advances task Markdown to schema 19. It adds three optional task " +
		"metadata keys: `check_in`, a calendar date this task is next meant to be looked at; " +
		"`check_in_repeat`, the cadence in whole days that acknowledging moves that date forward by; and " +
		"`reviewed`, the instant the task was last answered. All three are omitted when absent, so a task " +
		"that gains none of them renders exactly the bytes it did at schema 18.\n\n")
	report.WriteString("Unlike the last several boundaries this one changes content as well as the marker. " +
		"Every task in state `waiting` that already carried a `due` date was given a one-off check-in on " +
		"that same calendar day: the deadline of a wait is the day the wait was always meant to be revisited, " +
		"so the check-in restates what the task already said. The seeded value is a copy of the stored " +
		"`due` string, which is why the deadline, its optional `due_time`, `created`, `updated`, " +
		"`state_changed`, the description, the resume note, the comments, the checklist, and every other " +
		"stored value are byte-identical to their pre-migration form.\n\n")
	report.WriteString("No repeat and no review instant were invented. A cadence is a claim about how a " +
		"task is worked and a review instant is a claim that somebody looked at it; the migration knows " +
		"neither, and a value it made up would be indistinguishable afterwards from one a person earned. " +
		"Every seeded check-in is therefore one-off, and `reviewed` is absent everywhere until a real " +
		"acknowledgement writes it.\n\n")
	report.WriteString("Every project the catalog owns was migrated, including parked and archived ones. " +
		"Parking and archiving are read-time predicates over ordinary catalog entries, so a boundary that " +
		"skipped them would leave documents the new reader rejects in exactly the places nobody looks.\n\n")
	report.WriteString("Saved lens definitions are not rewritten because no query axis narrows by a check-in.\n\n")
	report.WriteString("Undo/redo history is dropped at this boundary, as it is at every boundary that " +
		"changes retained artifact bytes. Ledger entries store raw Markdown carrying " +
		"`<!-- tt-schema: 18 -->`; the schema-19 project-document scanner rejects that marker outright, so " +
		"replaying an entry would restore a document the new loader cannot read. The ledger is removed " +
		"inside the same transaction that activates the new schema, so there is no window in which a live " +
		"workspace holds unreplayable history. This report is the only place that can explain the empty " +
		"history, and the seeded check-ins above are not undoable for the same reason: rolling back this " +
		"boundary means restoring the archive beside this report.\n\n")
	report.WriteString("Conflict bundles under `.tt/conflicts/` are not rewritten either, here or at any " +
		"boundary. A bundle recorded before this migration keeps its schema-18 documents, so `tt conflict " +
		"show`, `--use remote`, and `--use file` keep working from its metadata while `--use local` and a " +
		"forced re-merge refuse by name: those two need the stored document to parse under the current reader, " +
		"and it no longer does. Nothing removes such a bundle for you.\n")
	return report.String()
}
