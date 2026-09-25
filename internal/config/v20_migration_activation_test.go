package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

func TestV20MigrationPreservesTasksAndClearsOldUndo(t *testing.T) {
	root := copyMigratedFixture(t, 19)
	manager := NewManager(ConfigPathForDataDir(root))
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	path := storage.TasksFilePath(catalog.Projects[0].CatalogPath(root))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(UndoLedgerDir(root), "ledger.json")
	if mkdirErr := os.MkdirAll(filepath.Dir(ledger), DirPermissions); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(ledger, []byte("old history"), FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}
	if migrateErr := manager.MigrateWorkspaceToV20(); migrateErr != nil {
		t.Fatal(migrateErr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.Replace(after, []byte("<!-- tt-schema: 18 -->"), []byte("<!-- tt-schema: 17 -->"), 1), before) {
		t.Fatal("migration changed task values")
	}
	if _, err := storage.ParseV18ProjectDocument(string(after)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ledger); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old undo survived: %v", err)
	}
	if err := manager.MigrateWorkspaceToV20(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
}
