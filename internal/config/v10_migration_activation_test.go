package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

type v10MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v10MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

// TestManagerV10MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore: a
// task document written after snapshot publication leaves the live schema-9
// workspace exactly as the mutation left it, still carrying the schema-8
// Markdown marker and the pre-rename `snoozed` key.
func TestManagerV10MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyMigratedFixture(t, 9)
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
	deferPreRenameTask(t, projectPath, "2030-01-02")

	var mutated []byte
	migrateErr := manager.migrateWorkspaceToV10(&v10MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV10TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V9ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V9ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v10 migration rewrote the schema-9 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v10 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 8 -->") {
		t.Fatalf("live project document left schema 8: %s", current)
	}
	if !strings.Contains(string(current), "snoozed: 2030-01-02") {
		t.Fatalf("aborted v10 migration renamed the deferral key: %s", current)
	}
}

// deferPreRenameTask gives the first task a schema-8 `snoozed` date so the
// rename has something to report and the abort has something to preserve.
func deferPreRenameTask(t *testing.T, path, date string) {
	t.Helper()
	document := readV8TaskDocument(t, path)
	document.Tasks[0].SnoozedUntil = date
	writeV8TaskDocument(t, path, document)
}

func rewriteV10TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	document := readV8TaskDocument(t, path)
	document.Tasks[0].Title = title
	return writeV8TaskDocument(t, path, document)
}

func readV8TaskDocument(t *testing.T, path string) storage.V8ProjectDocument {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV8ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-8 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	return document
}

func writeV8TaskDocument(t *testing.T, path string, document storage.V8ProjectDocument) []byte {
	t.Helper()
	published, err := storage.WriteV8ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV10MigrationRenamesDeferralKeyAndClearsHistory(t *testing.T) {
	root := copyMigratedFixture(t, 9)
	manager := NewManager(ConfigPathForDataDir(root))

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	deferPreRenameTask(t, projectPath, "2030-01-02")
	source := readV8TaskDocument(t, projectPath)

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-8 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV10(); migrateErr != nil {
		t.Fatalf("migrate schema 10: %v", migrateErr)
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V10ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V10ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 9 -->") {
		t.Fatalf("project did not publish schema 9: %s", migratedBytes)
	}
	if !strings.Contains(string(migratedBytes), "deferred: 2030-01-02") {
		t.Fatalf("project did not publish the deferred key: %s", migratedBytes)
	}
	if strings.Contains(string(migratedBytes), "snoozed:") {
		t.Fatalf("project retained the pre-rename snoozed key: %s", migratedBytes)
	}
	document, err := storage.ParseV9ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Tasks) != len(source.Tasks) {
		t.Fatalf("migrated tasks = %d, want %d", len(document.Tasks), len(source.Tasks))
	}
	for i, task := range document.Tasks {
		if task.SnoozedUntil != source.Tasks[i].SnoozedUntil {
			t.Fatalf("task %q deferral date = %q, want %q", task.ID, task.SnoozedUntil, source.Tasks[i].SnoozedUntil)
		}
	}
	if _, statErr := os.Stat(UndoLedgerDir(root)); !os.IsNotExist(statErr) {
		t.Fatalf("undo history survived migration: %v", statErr)
	}

	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	var auditName string
	foundArchive := false
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "pre-defer-rename-v10-") {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".tar.gz") {
			foundArchive = true
		}
		if strings.HasSuffix(entry.Name(), ".audit.md") {
			auditName = entry.Name()
		}
	}
	if !foundArchive || auditName == "" {
		t.Fatalf("rollback snapshot pair missing: %+v", entries)
	}
	auditBytes, err := os.ReadFile(filepath.Join(PreScopeV7BackupsDir(root), auditName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"`snoozed` is renamed to `deferred` on 2 tasks across 2 projects",
		"No value is reinterpreted",
		"Undo/redo history is dropped at this boundary",
		"| `project-archive` |",
	} {
		if !strings.Contains(string(auditBytes), want) {
			t.Fatalf("audit does not state %q:\n%s", want, auditBytes)
		}
	}
}

// TestManagerV10MigrationIsIdempotent proves a second activation over an
// already-migrated workspace writes nothing: neither the task document nor the
// catalog moves, and no second rollback snapshot pair is published.
func TestManagerV10MigrationIsIdempotent(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV10(); err != nil {
		t.Fatalf("migrate schema 10: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
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
	firstSnapshots := deferRenameSnapshotNames(t, root)

	if migrateErr := manager.MigrateWorkspaceToV10(); migrateErr != nil {
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
	if second := deferRenameSnapshotNames(t, root); len(second) != len(firstSnapshots) {
		t.Fatalf("second migration published snapshots %v, want %v", second, firstSnapshots)
	}
}

func deferRenameSnapshotNames(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(PreScopeV7BackupsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "pre-defer-rename-v10-") {
			names = append(names, entry.Name())
		}
	}
	return names
}
