package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/transaction"
)

type v21MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v21MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

func preCheckInsV21SnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, preCheckInsV21SnapshotPrefix) &&
			(strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".audit.md")) {
			names = append(names, name)
		}
	}
	return names
}

// waitingV21FixtureDue is the deadline the seeding cases put on their Waiting
// task. It is a plain date in the past so the seeded check-in is one an ordinary
// reader would consider arrived, which is the state a migrated wait should come
// back in.
const waitingV21FixtureDue = "2026-07-15"

// rewriteV18TaskStates edits the schema-18 document in place so the first task
// is Waiting with a deadline and the second, when there is one, is Waiting with
// no deadline. It returns the published bytes so a caller can compare against
// them afterwards.
//
// It writes through the schema-18 codec rather than patching text, so the source
// the migration reads is a document the strict reader accepts.
func rewriteV18TaskStates(t *testing.T, path string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV18ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-18 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	document.Tasks[0].State = string(models.StateWaiting)
	document.Tasks[0].DueDate = waitingV21FixtureDue
	document.Tasks[0].CompletedAt = nil
	if len(document.Tasks) > 1 {
		document.Tasks[1].State = string(models.StateWaiting)
		document.Tasks[1].DueDate = ""
		document.Tasks[1].CompletedAt = nil
	}
	published, err := storage.WriteV18ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

func rewriteV18TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV18ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-18 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	document.Tasks[0].Title = title
	published, err := storage.WriteV18ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

// TestManagerSnapshotFirstV21MigrationSeedsWaitingCheckIns is the whole claim of
// this boundary: a Waiting task with a deadline gains a one-off check-in on that
// day, every other stored value is preserved, no review instant is invented, the
// catalog advances beside the documents, and the undo ledger is gone because its
// entries still hold schema-18 bytes.
//
//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV21MigrationSeedsWaitingCheckIns(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	beforeProjectBytes := rewriteV18TaskStates(t, projectPath)
	beforeDocument, err := storage.ParseV18ProjectDocument(string(beforeProjectBytes))
	if err != nil {
		t.Fatal(err)
	}
	beforeLenses, lensesExisted := readLensDefinitionsIfPresent(t, root)

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-20 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	snapshotObserved := false
	if migrateErr := manager.migrateWorkspaceToV21(&v21MigrationActivationTestHooks{
		afterSnapshotPublished: func(archivePath, auditPath string) {
			snapshotObserved = true
			for _, path := range []string{archivePath, auditPath} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatalf("published rollback artifact %s: %v", path, statErr)
				}
			}
			schema, exists, schemaErr := workspaceCatalogPayloadSchema(root)
			if schemaErr != nil || !exists || schema != V20ConfigSchemaVersion {
				t.Fatalf("snapshot hook catalog schema = %d exists=%v err=%v, want %d",
					schema, exists, schemaErr, V20ConfigSchemaVersion)
			}
		},
	}); migrateErr != nil {
		t.Fatalf("migrate schema 21: %v", migrateErr)
	}
	if !snapshotObserved {
		t.Fatal("migration activated without observing rollback snapshot publication")
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V21ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V21ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 19 -->") {
		t.Fatalf("task Markdown did not reach schema 19: %s", migratedBytes)
	}
	document, err := storage.ParseV19ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatalf("parse migrated document: %v", err)
	}
	if len(document.Tasks) != len(beforeDocument.Tasks) {
		t.Fatalf("migration changed the task count: %d, want %d", len(document.Tasks), len(beforeDocument.Tasks))
	}
	seeded := document.Tasks[0]
	if seeded.NextCheckIn != waitingV21FixtureDue {
		t.Fatalf("waiting task check-in = %q, want the stored deadline %q", seeded.NextCheckIn, waitingV21FixtureDue)
	}
	if seeded.CheckInRepeatDays != models.CheckInRepeatOff {
		t.Fatalf("seeded check-in repeats every %d days, want a one-off", seeded.CheckInRepeatDays)
	}
	if seeded.LastReviewedAt != nil {
		t.Fatalf("migration invented a review instant: %v", seeded.LastReviewedAt)
	}
	// Everything the task already said must be exactly what it still says: the
	// deadline, its companion clock, both timestamps, the body, and every other
	// inherited field. Comparing the embedded schema-18 record wholesale is what
	// keeps that claim from falling behind the struct.
	for index := range document.Tasks {
		got, want := document.Tasks[index], beforeDocument.Tasks[index]
		if !reflect.DeepEqual(got.V17TaskRecord, want) {
			t.Fatalf("migration changed task %d beyond its check-in\nbefore: %+v\nafter:  %+v", index, want, got.V17TaskRecord)
		}
	}
	if len(document.Tasks) > 1 && document.Tasks[1].NextCheckIn != "" {
		t.Fatalf("a Waiting task with no deadline was given a check-in: %q", document.Tasks[1].NextCheckIn)
	}
	// This boundary owns no lens axis, so the portable collection must be
	// untouched rather than republished with equivalent contents.
	if lensesExisted {
		afterLenses, stillThere := readLensDefinitionsIfPresent(t, root)
		if !stillThere || !bytes.Equal(beforeLenses, afterLenses) {
			t.Fatal("v21 migration rewrote the portable lens definitions")
		}
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	var auditName string
	for _, name := range preCheckInsV21SnapshotNames(t, root) {
		if strings.HasSuffix(name, ".audit.md") {
			auditName = name
		}
	}
	if auditName == "" {
		t.Fatal("rollback snapshot audit missing")
	}
	auditBytes, err := os.ReadFile(filepath.Join(PreScopeV7BackupsDir(root), auditName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Waiting tasks seeded with a check-in: 1",
		"No repeat and no review instant were invented",
		"including parked and archived ones",
		"Saved lens definitions are not rewritten",
		"Undo/redo history is dropped at this boundary",
		"Conflict bundles under `.tt/conflicts/` are not rewritten",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

// TestManagerV21MigrationPreservesProjectComments keeps the boundary honest about
// the collection the previous one introduced: comments are carried, not
// re-derived, so a project's stored notes survive a check-in migration untouched.
func TestManagerV21MigrationPreservesProjectComments(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	source, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV18ProjectDocument(string(source))
	if err != nil {
		t.Fatal(err)
	}
	document.Comments = []format.ProjectCommentRecord{
		{
			ID:        "3f1c2a90-5b6d-4e21-9a77-0c8e4d5f6a1b",
			Text:      "the earlier note\nwith a second line",
			CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        "8c2d4e61-7a90-4b33-9d55-1e6f7a8b9c0d",
			Text:      "the later note",
			CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
		},
	}
	published, err := storage.WriteV18ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(projectPath, []byte(published), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV21(); migrateErr != nil {
		t.Fatalf("migrate schema 21: %v", migrateErr)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := storage.ParseV19ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated.Comments) != len(document.Comments) {
		t.Fatalf("migrated project holds %d comments, want %d", len(migrated.Comments), len(document.Comments))
	}
	for i, want := range document.Comments {
		got := migrated.Comments[i]
		if got.ID != want.ID || got.Text != want.Text || !got.CreatedAt.Equal(want.CreatedAt) {
			t.Fatalf("comment %d = %+v, want %+v", i, got, want)
		}
	}
}

// TestManagerV21MigrationValidationFailurePublishesNothing proves the boundary
// reads every project document strictly before it publishes one byte.
func TestManagerV21MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	beforeCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte("<!-- tt-schema: 99 -->\n# Tasks\n")
	if err := os.WriteFile(projectPath, invalid, FilePermissions); err != nil {
		t.Fatal(err)
	}
	beforeSnapshots := preCheckInsV21SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV21()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := preCheckInsV21SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
		t.Fatalf("validation failure published snapshot artifacts: before=%v after=%v", beforeSnapshots, after)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, beforeCatalog) {
		t.Fatal("validation failure changed catalog bytes")
	}
	if current, readErr := os.ReadFile(projectPath); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, invalid) {
		t.Fatal("validation failure changed project bytes")
	}
}

// TestManagerV21MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore.
func TestManagerV21MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	catalogBytes, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}

	var mutated []byte
	migrateErr := manager.migrateWorkspaceToV21(&v21MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV18TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V20ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V20ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v21 migration rewrote the schema-20 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v21 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 18 -->") {
		t.Fatalf("aborted v21 migration advanced the task schema marker:\n%s", current)
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV21MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	plan, err := buildV21MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreCheckInsV21Snapshot(root, plan.seeded); err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanup))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanup...)
	journal := transaction.NewJournal(TransactionsDir(root), []string{root, LocalDataRoot()}, "")
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}

	if err := manager.RecoverWorkspace(); err != nil {
		t.Fatalf("recover prepared schema-21 activation on startup: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := preCheckInsV21SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV21MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot. It
// is also what makes the seeding pass safe to re-run: a task that already carries
// the check-in this boundary would mint is left exactly as it is.
func TestManagerV21MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV21(); err != nil {
		t.Fatalf("migrate schema 21: %v", err)
	}
	catalogBytes, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	snapshots := preCheckInsV21SnapshotNames(t, root)
	if len(snapshots) != 2 {
		t.Fatalf("published snapshot artifacts = %v, want one pair", snapshots)
	}

	if err := manager.MigrateWorkspaceToV21(); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("second v21 activation rewrote the catalog")
	}
	if after := preCheckInsV21SnapshotNames(t, root); len(after) != len(snapshots) {
		t.Fatalf("second v21 activation published snapshots: %v", after)
	}
}

// TestManagerV21MigrationRefusesANewerWorkspace keeps the guard honest about a
// catalog this build cannot read, which is the same refusal an old binary owes a
// schema-21 workspace.
func TestManagerV21MigrationRefusesANewerWorkspace(t *testing.T) {
	root := copyMigratedFixture(t, 21)
	manager := NewManager(ConfigPathForDataDir(root))
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, newerThanSupportedCatalog(t, source, V21ConfigSchemaVersion), FilePermissions); err != nil {
		t.Fatal(err)
	}
	migrateErr := manager.MigrateWorkspaceToV21()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "newer than supported schema") {
		t.Fatalf("newer workspace error = %v, want an upgrade refusal", migrateErr)
	}
}

// newerThanSupportedCatalog rewrites a catalog's schema marker to one version
// past what this build supports, so the caller can check that the guard refuses
// it.
//
// The replacement is derived from CurrentConfigVersion rather than spelled,
// because every one of these guards was written with a literal that was newer at
// the time and stopped being newer the moment that version was allocated. A
// guard that has quietly become a supported version tests nothing and still
// passes for the boundary that owns it, which is exactly how five of them failed
// together at this boundary instead of one of them failing at each.
func newerThanSupportedCatalog(t *testing.T, source []byte, currentSchema int) []byte {
	t.Helper()
	newer := bytes.Replace(source,
		fmt.Appendf(nil, "%q: %d", "schema_version", currentSchema),
		fmt.Appendf(nil, "%q: %d", "schema_version", SupportedConfigVersion+1), 1)
	if bytes.Equal(newer, source) {
		t.Fatalf("catalog does not carry the expected schema marker:\n%s", source)
	}
	return newer
}

// TestOperationsAfterV21MigrationAreNormallyUndoable is the other half of the
// history-cleanup contract. Dropping the ledger is only acceptable because the
// next mutation rebuilds one, so this replays an ordinary post-migration edit —
// scheduling the check-in the boundary exists for — and takes it back.
func TestOperationsAfterV21MigrationAreNormallyUndoable(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.RecoverWorkspace(); err != nil {
		t.Fatalf("recover workspace: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != CurrentConfigVersion {
		t.Fatalf("catalog schema = %d, want %d", catalog.SchemaVersion, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	project, tasksPath := loadMigratedProjectWithTasks(t, root, catalog)
	migratedBytes, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}

	ledger := storage.NewWorkspaceLedger(UndoLedgerDir(root), []string{root})
	commits := storage.NewCommitServiceWithLedger(TransactionsDir(root), []string{root}, ledger)
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	next, err := models.NextCheckInAfter(now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if setErr := project.Tasks[0].SetCheckIn(models.CheckInSchedule{Next: next, RepeatDays: 7}, now); setErr != nil {
		t.Fatal(setErr)
	}
	if _, commitErr := commits.Commit(storage.ChangeSet{
		Project: project, ExpectedRevision: document.Revision, Base: document,
		UndoAction: "Schedule Check-in", UndoContext: project.Name,
	}); commitErr != nil {
		t.Fatalf("commit after migration: %v", commitErr)
	}
	if count, countErr := ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 1 {
		t.Fatalf("undo entries after one post-migration commit = %d err=%v, want 1", count, countErr)
	}
	edited, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(edited), "check_in: "+next.Format("2006-01-02")) {
		t.Fatalf("post-migration commit did not persist the check-in:\n%s", edited)
	}

	if _, undoErr := ledger.Undo(storage.GlobalScope()); undoErr != nil {
		t.Fatalf("undo after migration: %v", undoErr)
	}
	restored, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, migratedBytes) {
		t.Fatalf("undo did not restore the migrated bytes\nwant: %s\ngot:  %s", migratedBytes, restored)
	}
}

// TestV21MigrationCarriesArchivedAndParkedProjects keeps the boundary
// indifferent to a project's lifecycle: every cataloged project is migrated, not
// just the ones the Outliner would show.
func TestV21MigrationCarriesArchivedAndParkedProjects(t *testing.T) {
	root := copyMigratedFixture(t, 20)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	markable := make([]*CatalogProject, 0, len(catalog.Projects))
	for index := range catalog.Projects {
		if catalog.Projects[index].ID == catalog.SystemUnfiledProjectID {
			continue
		}
		markable = append(markable, &catalog.Projects[index])
	}
	if len(markable) < 2 {
		t.Fatalf("fixture has %d markable projects; this case needs two", len(markable))
	}
	markable[0].Archived = true
	parked, err := models.NormalizeProjectTags(append(markable[1].Tags, "parked"))
	if err != nil {
		t.Fatal(err)
	}
	markable[1].Tags = parked
	// The archived project gets the Waiting deadline, so the case also proves the
	// seeding pass reaches a project no ordinary view would load.
	archivedPath := storage.TasksFilePath(markable[0].CatalogPath(root))
	rewriteV18TaskStates(t, archivedPath)
	published, err := encodeMigrationCatalogEnvelope(&catalog, V20ConfigSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(WorkspaceCatalogPath(root), published, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV21(); migrateErr != nil {
		t.Fatalf("migrate schema 21: %v", migrateErr)
	}
	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range migrated.Projects {
		source, readErr := os.ReadFile(storage.TasksFilePath(entry.CatalogPath(root)))
		if readErr != nil {
			t.Fatalf("read migrated project %s: %v", entry.Slug, readErr)
		}
		if !strings.Contains(string(source), "<!-- tt-schema: 19 -->") {
			t.Fatalf("project %s was left behind at an older schema:\n%s", entry.Slug, source)
		}
	}
	archived, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archived), "check_in: "+waitingV21FixtureDue) {
		t.Fatalf("the archived project's Waiting task was not seeded:\n%s", archived)
	}
}

// TestFreshWorkspaceStartsAtTheCheckInSchemaWithNoCheckIns proves the other end of
// the boundary: a workspace created by this build is already at schema 21 and
// crossed no migration, so it publishes no rollback snapshot and its tasks carry
// no check-in.
func TestFreshWorkspaceStartsAtTheCheckInSchemaWithNoCheckIns(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	if catalog.SchemaVersion != V21ConfigSchemaVersion {
		t.Fatalf("fresh catalog schema = %d, want %d", catalog.SchemaVersion, V21ConfigSchemaVersion)
	}
	if CurrentConfigVersion != V21ConfigSchemaVersion {
		t.Fatalf("CurrentConfigVersion = %d, want %d", CurrentConfigVersion, V21ConfigSchemaVersion)
	}

	// A workspace created here crosses no boundary, so the absent check-in is the
	// live writer's doing rather than a migration's: a task nobody scheduled must
	// render none of the three keys schema 19 added.
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	project := &models.Project{Name: "Fresh", Tasks: []*models.Task{{
		ID: "task-1", ShortID: 1, Title: "Untouched",
		WorkflowState: models.StateBacklog,
		CreatedAt:     now, UpdatedAt: now, StateChangedAt: now,
	}}}
	content, err := storage.WriteProjectMarkdown(project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "<!-- tt-schema: 19 -->") {
		t.Fatalf("fresh project is not at the current task schema:\n%s", content)
	}
	for _, key := range []string{"check_in:", "check_in_repeat:", "reviewed:"} {
		if strings.Contains(content, key) {
			t.Fatalf("fresh project carries %s:\n%s", key, content)
		}
	}
}
