package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

type v9MigrationActivationTestHooks struct {
	afterSnapshotPublished func(archivePath, auditPath string)
}

func (h *v9MigrationActivationTestHooks) didPublishSnapshot(archivePath, auditPath string) {
	if h != nil && h.afterSnapshotPublished != nil {
		h.afterSnapshotPublished(archivePath, auditPath)
	}
}

// TestManagerV9MigrationAbortsOnWorkspaceMutationAfterSnapshot proves the
// activation guard refuses a candidate the rollback archive cannot restore: a
// task document written after snapshot publication leaves the live schema-8
// workspace exactly as the mutation left it.
func TestManagerV9MigrationAbortsOnWorkspaceMutationAfterSnapshot(t *testing.T) {
	root := copyMigratedFixture(t, 8)
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
	migrateErr := manager.migrateWorkspaceToV9(&v9MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			mutated = rewriteV9TaskTitle(t, projectPath, "Concurrent edit after snapshot")
		},
	})
	if migrateErr == nil {
		t.Fatal("migration activated a candidate the rollback archive does not describe")
	}
	if !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("migration error does not name the inventory divergence: %v", migrateErr)
	}

	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V8ConfigSchemaVersion {
		t.Fatalf("live catalog schema = %d exists=%v err=%v, want %d", schema, exists, err, V8ConfigSchemaVersion)
	}
	if current, readErr := os.ReadFile(WorkspaceCatalogPath(root)); readErr != nil {
		t.Fatal(readErr)
	} else if !bytes.Equal(current, catalogBytes) {
		t.Fatal("aborted v9 migration rewrote the schema-8 catalog")
	}
	current, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, mutated) {
		t.Fatalf("aborted v9 migration rewrote live task bytes:\n%s", current)
	}
	if !strings.Contains(string(current), "<!-- tt-schema: 7 -->") {
		t.Fatalf("live project document left schema 7: %s", current)
	}
}

func rewriteV9TaskTitle(t *testing.T, path, title string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseV7ProjectDocument(string(source))
	if err != nil || len(document.Tasks) == 0 {
		t.Fatalf("parse schema-7 project: tasks=%d err=%v", len(document.Tasks), err)
	}
	document.Tasks[0].Title = title
	published, err := storage.WriteV7ProjectDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(published), FilePermissions); err != nil {
		t.Fatal(err)
	}
	return []byte(published)
}

//nolint:gocyclo // One end-to-end migration rehearsal keeps the guarded contract together.
func TestManagerSnapshotFirstV9MigrationBackfillsStateClockAndClearsHistory(t *testing.T) {
	root := copyMigratedFixture(t, 8)
	manager := NewManager(ConfigPathForDataDir(root))

	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("fixture has no projects")
	}
	projectPath := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	sourceBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	source, err := storage.ParseV7ProjectDocument(string(sourceBytes))
	if err != nil || len(source.Tasks) == 0 {
		t.Fatalf("parse schema-7 project: tasks=%d err=%v", len(source.Tasks), err)
	}

	ledgerPath := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledgerPath), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledgerPath, []byte("schema-7 history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	if migrateErr := manager.MigrateWorkspaceToV9(); migrateErr != nil {
		t.Fatalf("migrate schema 9: %v", migrateErr)
	}

	migrated, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if migrated.SchemaVersion != V9ConfigSchemaVersion {
		t.Fatalf("catalog schema = %d, want %d", migrated.SchemaVersion, V9ConfigSchemaVersion)
	}
	migratedBytes, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedBytes), "<!-- tt-schema: 8 -->") {
		t.Fatalf("project did not publish schema 8: %s", migratedBytes)
	}
	document, err := storage.ParseV8ProjectDocument(string(migratedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Tasks) != len(source.Tasks) {
		t.Fatalf("migrated tasks = %d, want %d", len(document.Tasks), len(source.Tasks))
	}
	for i, task := range document.Tasks {
		if !task.StateChangedAt.Equal(source.Tasks[i].UpdatedAt) {
			t.Fatalf("task %q state clock = %v, want backfill from updated %v", task.ID, task.StateChangedAt, source.Tasks[i].UpdatedAt)
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
		if !strings.HasPrefix(entry.Name(), "pre-state-clock-v9-") {
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
	if !strings.Contains(string(auditBytes), "approximation") ||
		!strings.Contains(string(auditBytes), "self-corrects on the task's next state transition") {
		t.Fatalf("audit does not disclose the backfill approximation:\n%s", auditBytes)
	}

	if migrateErr := manager.MigrateWorkspaceToV9(); migrateErr != nil {
		t.Fatalf("idempotent migration: %v", migrateErr)
	}
}
