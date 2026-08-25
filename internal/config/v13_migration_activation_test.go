package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

type v13MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v13MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

// TestManagerV13MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore.
func TestManagerV13MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV12(); err != nil {
		t.Fatalf("publish schema 12: %v", err)
	}
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
	migrateErr := manager.migrateWorkspaceToV13(&v13MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV13TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V12ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V12ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v13 migration rewrote the schema-12 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v13 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 11 -->") {
		t.Fatalf("live project document left schema 11: %s", current)
	}
}

func rewriteV13TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	document := readV11TaskDocument(t, path)
	document.Tasks[0].Title = title
	return writeV11TaskDocument(t, path, document)
}

func readV11TaskDocument(t *testing.T, path string) storage.V11ProjectDocument {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV11ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-11 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	return document
}

func writeV11TaskDocument(t *testing.T, path string, document storage.V11ProjectDocument) []byte {
	t.Helper()
	published, err := storage.WriteV11ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV13MigrationAddsZeroDeferCountAndClearsHistory(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV12(); err != nil {
		t.Fatalf("publish schema 12: %v", err)
	}

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	source := readV11TaskDocument(t, projectPath)
	beforeProject, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-11 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	snapshotObserved := false
	if migrateErr := manager.migrateWorkspaceToV13(&v13MigrationActivationTestHooks{
		afterSnapshotPublished: func(archivePath, auditPath string) {
			snapshotObserved = true
			for _, path := range []string{archivePath, auditPath} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Fatalf("published rollback artifact %s: %v", path, statErr)
				}
			}
			entries := readPreSimplificationV5Archive(t, archivePath)
			projectRel, relErr := filepath.Rel(root, projectPath)
			if relErr != nil {
				t.Fatal(relErr)
			}
			if archived := entries["workspace/"+filepath.ToSlash(projectRel)]; !bytes.Equal(archived, beforeProject) {
				t.Fatal("rollback archive project bytes differ from the live pre-migration document")
			}
			if archived := entries["workspace/.tt/workspace.json"]; !bytes.Equal(archived, beforeCatalog) {
				t.Fatal("rollback archive catalog bytes differ from the live pre-migration catalog")
			}
			schema, exists, schemaErr := workspaceCatalogPayloadSchema(root)
			if schemaErr != nil || !exists || schema != V12ConfigSchemaVersion {
				t.Fatalf("snapshot hook catalog schema = %d exists=%v err=%v, want %d", schema, exists, schemaErr, V12ConfigSchemaVersion)
			}
			current, readErr := os.ReadFile(projectPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(current, beforeProject) {
				t.Fatal("schema-11 task bytes changed before rollback snapshot publication")
			}
		},
	}); migrateErr != nil {
		t.Fatalf("migrate schema 13: %v", migrateErr)
	}
	if !snapshotObserved {
		t.Fatal("migration activated without observing rollback snapshot publication")
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V13ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V13ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 12 -->") {
		t.Fatalf("project did not publish schema 12: %s", migratedBytes)
	}
	document, err := storage.ParseV12ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Tasks) != len(source.Tasks) {
		t.Fatalf("migrated tasks = %d, want %d", len(document.Tasks), len(source.Tasks))
	}
	for index, task := range document.Tasks {
		if task.DeferCount != 0 {
			t.Fatalf("migrated task %q defer count = %d, want zero", task.ID, task.DeferCount)
		}
		if task.ID != source.Tasks[index].ID || task.Title != source.Tasks[index].Title {
			t.Fatalf("migrated task %d changed identity or title", index)
		}
	}
	if strings.Contains(string(migratedBytes), "defer_count:") {
		t.Fatal("zero defer count was not canonically omitted")
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	var auditName string
	for _, name := range deferCountV13SnapshotNames(t, root) {
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
		"optional task-level `defer_count` field",
		"zero deferral count",
		"Undo/redo history is dropped at this boundary",
		"`<!-- tt-schema: 11 -->`",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

func TestManagerV13MigrationValidationFailurePublishesNothing(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV12(); err != nil {
		t.Fatalf("publish schema 12: %v", err)
	}
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
	beforeSnapshots := deferCountV13SnapshotNames(t, root)

	migrateErr := manager.MigrateWorkspaceToV13()
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "workspace was not changed") {
		t.Fatalf("invalid migration error = %v, want unchanged refusal", migrateErr)
	}
	if after := deferCountV13SnapshotNames(t, root); len(after) != len(beforeSnapshots) {
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

//nolint:govet // Scoped setup and recovery errors keep the interrupted transaction stages readable.
func TestManagerV13MigrationRecoversPreparedActivationOnStartup(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV12(); err != nil {
		t.Fatalf("publish schema 12: %v", err)
	}
	plan, err := buildV13MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	if _, err := createPreDeferCountV13Snapshot(root); err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}

	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanup))
	for _, artifact := range plan.artifacts {
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode,
		})
	}
	ops = append(ops, plan.cleanup...)
	journal := transaction.NewJournal(TransactionsDir(root), []string{root, LocalDataRoot()})
	manifest, err := journal.Prepare(ops)
	if err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}

	if err := manager.RecoverWorkspace(); err != nil {
		t.Fatalf("recover prepared schema-13 activation on startup: %v", err)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema < V13ConfigSchemaVersion {
		t.Fatalf("recovered catalog schema = %d exists=%v err=%v, want >= %d", schema, exists, err, V13ConfigSchemaVersion)
	}
	if _, statErr := os.Stat(journal.ManifestPath(manifest.ID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered activation manifest still exists: %v", statErr)
	}
	if snapshots := deferCountV13SnapshotNames(t, root); len(snapshots) != 2 {
		t.Fatalf("recovered activation snapshot artifacts = %v, want the one published pair", snapshots)
	}
}

// TestManagerV13MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing and publishes no second snapshot.
func TestManagerV13MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV13(); err != nil {
		t.Fatalf("migrate schema 13: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	firstTasks, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	firstCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshots := deferCountV13SnapshotNames(t, root)

	if migrateErr := manager.MigrateWorkspaceToV13(); migrateErr != nil {
		t.Fatalf("idempotent migration: %v", migrateErr)
	}

	secondTasks, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstTasks, secondTasks) {
		t.Fatalf("second migration rewrote the task document:\n%s", secondTasks)
	}
	secondCatalog, err := os.ReadFile(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstCatalog, secondCatalog) {
		t.Fatal("second migration rewrote the catalog")
	}
	if second := deferCountV13SnapshotNames(t, root); len(second) != len(firstSnapshots) {
		t.Fatalf("second migration published snapshots %v, want %v", second, firstSnapshots)
	}
}

func deferCountV13SnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pre-defer-count-v13-") &&
			(strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".audit.md")) {
			names = append(names, name)
		}
	}
	return names
}
