package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

type v19MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v19MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

func preProjectCommentsV19SnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pre-project-comments-v19-") &&
			(strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".audit.md")) {
			names = append(names, name)
		}
	}
	return names
}

func rewriteV16TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV16ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-16 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	document.Tasks[0].Title = title
	published, err := storage.WriteV16ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

// TestManagerSnapshotFirstV19MigrationAdvancesTheMarkerAndClearsHistory is the
// whole claim of this boundary: schema 17 admits a collection nothing carries
// yet, so every document must come out byte-identical to its schema-16 source
// apart from the marker line, the catalog must advance beside it, and the undo
// ledger must be gone because its entries still hold schema-16 bytes.
//
//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV19MigrationAdvancesTheMarkerAndClearsHistory(t *testing.T) {
	root := copyMigratedFixture(t, 18)
	manager := NewManager(ConfigPathForDataDir(root))

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	beforeProjectBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLenses, lensesExisted := readLensDefinitionsIfPresent(t, root)

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-18 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	snapshotObserved := false
	if migrateErr := manager.migrateWorkspaceToV19(&v19MigrationActivationTestHooks{
		afterSnapshotPublished: func(archivePath, auditPath string) {
			snapshotObserved = true
			for _, path := range []string{archivePath, auditPath} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatalf("published rollback artifact %s: %v", path, statErr)
				}
			}
			schema, exists, schemaErr := workspaceCatalogPayloadSchema(root)
			if schemaErr != nil || !exists || schema != V18ConfigSchemaVersion {
				t.Fatalf("snapshot hook catalog schema = %d exists=%v err=%v, want %d", schema, exists, schemaErr, V18ConfigSchemaVersion)
			}
		},
	}); migrateErr != nil {
		t.Fatalf("migrate schema 19: %v", migrateErr)
	}
	if !snapshotObserved {
		t.Fatal("migration activated without observing rollback snapshot publication")
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V19ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V19ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 17 -->") {
		t.Fatalf("task Markdown did not reach schema 17: %s", migratedBytes)
	}
	// The whole rewrite must be the schema marker and nothing else.
	if restored := bytes.Replace(migratedBytes, []byte("<!-- tt-schema: 17 -->"), []byte("<!-- tt-schema: 16 -->"), 1); !bytes.Equal(restored, beforeProjectBytes) {
		t.Fatalf("migration changed task bytes beyond the schema marker\nbefore: %s\nafter:  %s", beforeProjectBytes, migratedBytes)
	}
	if strings.Contains(string(migratedBytes), "PROJECT-COMMENTS") {
		t.Fatalf("migration minted a project-comment block:\n%s", migratedBytes)
	}
	document, err := storage.ParseV17ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatalf("parse migrated document: %v", err)
	}
	if len(document.Comments) != 0 {
		t.Fatalf("migrated project starts with %d comments, want none", len(document.Comments))
	}
	// This boundary owns no lens axis, so the portable collection must be
	// untouched rather than republished with equivalent contents.
	if lensesExisted {
		afterLenses, stillThere := readLensDefinitionsIfPresent(t, root)
		if !stillThere || !bytes.Equal(beforeLenses, afterLenses) {
			t.Fatal("v19 migration rewrote the portable lens definitions")
		}
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	var auditName string
	for _, name := range preProjectCommentsV19SnapshotNames(t, root) {
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
		"optional collection of project comments",
		"byte-identical to its schema-16",
		"Saved lens definitions are not rewritten",
		"Undo/redo history is dropped at this boundary",
		"Conflict bundles under `.docket/conflicts/` are not rewritten",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

// TestManagerV19MigrationValidationFailurePublishesNothing proves the boundary
// reads every project document strictly before it publishes one byte.
func TestManagerV19MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyMigratedFixture(t, 18)
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
	beforeSnapshots := preProjectCommentsV19SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV19()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := preProjectCommentsV19SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
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

// TestManagerV19MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore.
func TestManagerV19MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyMigratedFixture(t, 18)
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
	migrateErr := manager.migrateWorkspaceToV19(&v19MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV16TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V18ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V18ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v19 migration rewrote the schema-18 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v19 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 16 -->") {
		t.Fatalf("aborted v19 migration advanced the task schema marker:\n%s", current)
	}
}

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV19MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyMigratedFixture(t, 18)
	manager := NewManager(ConfigPathForDataDir(root))
	plan, err := buildV19MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreProjectCommentsV19Snapshot(root); err != nil {
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
		t.Fatalf("recover prepared schema-19 activation on startup: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != CurrentConfigVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, CurrentConfigVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := preProjectCommentsV19SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV19MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot.
func TestManagerV19MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV19(); err != nil {
		t.Fatalf("migrate schema 19: %v", err)
	}
	catalogBytes, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	snapshots := preProjectCommentsV19SnapshotNames(t, root)
	if len(snapshots) != 2 {
		t.Fatalf("published snapshot artifacts = %v, want one pair", snapshots)
	}

	if err := manager.MigrateWorkspaceToV19(); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("second v19 activation rewrote the catalog")
	}
	if after := preProjectCommentsV19SnapshotNames(t, root); len(after) != len(snapshots) {
		t.Fatalf("second v19 activation published snapshots: %v", after)
	}
}

// TestManagerV19MigrationRefusesANewerWorkspace keeps the guard honest about a
// catalog this build cannot read.
func TestManagerV19MigrationRefusesANewerWorkspace(t *testing.T) {
	root := copyMigratedFixture(t, 19)
	manager := NewManager(ConfigPathForDataDir(root))
	catalogPath := WorkspaceCatalogPath(root)
	source, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, newerThanSupportedCatalog(t, source, V19ConfigSchemaVersion), FilePermissions); err != nil {
		t.Fatal(err)
	}
	migrateErr := manager.MigrateWorkspaceToV19()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "newer than supported schema") {
		t.Fatalf("newer workspace error = %v, want an upgrade refusal", migrateErr)
	}
}

// TestOperationsAfterV19MigrationAreNormallyUndoable is the other half of the
// history-cleanup contract. Dropping the ledger is only acceptable because the
// next mutation rebuilds one, so this replays an ordinary post-migration edit —
// writing the collection the boundary exists for — and takes it back.
func TestOperationsAfterV19MigrationAreNormallyUndoable(t *testing.T) {
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
	project.Comments = []*models.ProjectComment{{
		ID:        "3f1c2a90-5b6d-4e21-9a77-0c8e4d5f6a1b",
		Text:      "noted after the upgrade",
		CreatedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
	}}
	if _, commitErr := commits.Commit(storage.ChangeSet{
		Project: project, ExpectedRevision: document.Revision, Base: document,
		UndoAction: "Add Project Comment", UndoContext: project.Name,
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
	if !strings.Contains(string(edited), "noted after the upgrade") {
		t.Fatalf("post-migration commit did not persist the project comment:\n%s", edited)
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

// TestV19MigrationCarriesArchivedAndParkedProjects keeps the boundary
// indifferent to a project's lifecycle: every cataloged project is migrated, not
// just the ones the Outliner would show.
func TestV19MigrationCarriesArchivedAndParkedProjects(t *testing.T) {
	root := copyMigratedFixture(t, 18)
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
	published, err := encodeMigrationCatalogEnvelope(&catalog, V18ConfigSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(WorkspaceCatalogPath(root), published, FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV19(); migrateErr != nil {
		t.Fatalf("migrate schema 19: %v", migrateErr)
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
		if !strings.Contains(string(source), "<!-- tt-schema: 17 -->") {
			t.Fatalf("project %s was left behind at an older schema:\n%s", entry.Slug, source)
		}
	}
}
