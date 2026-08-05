package config

import (
	"os"
	"path/filepath"
	"testing"
)

func newDomainManager(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	return NewManager(filepath.Join(root, ".tt.json"))
}

func TestManagerBackupConfig(t *testing.T) {
	manager := newDomainManager(t)
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatal(err)
	}
	domains.Catalog.Projects = append(domains.Catalog.Projects, CatalogProject{ID: "p1", Slug: "one", Name: "One"})
	if commitErr := manager.CommitCatalog(domains.Catalog); commitErr != nil {
		t.Fatal(commitErr)
	}
	if backupErr := manager.BackupConfig(); backupErr != nil {
		t.Fatal(backupErr)
	}
	if !manager.ConfigBackupExists() {
		t.Fatal("expected a split-domain backup")
	}
	if _, err := os.Stat(manager.GetBackupPath()); err != nil {
		t.Fatalf("workspace backup missing: %v", err)
	}
}

func TestManagerRestoreConfigFromBackupIsReversible(t *testing.T) {
	manager := newDomainManager(t)
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatal(err)
	}
	domains.Catalog.Projects = append(domains.Catalog.Projects, CatalogProject{ID: "p1", Slug: "one", Name: "One"})
	if commitErr := manager.CommitCatalog(domains.Catalog); commitErr != nil {
		t.Fatal(commitErr)
	}
	if backupErr := manager.BackupConfig(); backupErr != nil {
		t.Fatal(backupErr)
	}
	domains.Catalog.Projects = append(domains.Catalog.Projects, CatalogProject{ID: "p2", Slug: "two", Name: "Two"})
	if commitErr := manager.CommitCatalog(domains.Catalog); commitErr != nil {
		t.Fatal(commitErr)
	}

	if restoreErr := manager.RestoreConfigFromBackup(); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	restored, err := manager.LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(restored.Catalog.Projects); got != 2 {
		t.Fatalf("restored projects = %d, want 2", got)
	}

	if restoreErr := manager.RestoreConfigFromBackup(); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	restored, err = manager.LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(restored.Catalog.Projects); got != 3 {
		t.Fatalf("reversed restore projects = %d, want 3", got)
	}
}

func TestManagerRestoreConfigFromBackupRequiresBackup(t *testing.T) {
	manager := newDomainManager(t)
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatal(err)
	}
	if err := manager.RestoreConfigFromBackup(); err == nil {
		t.Fatal("expected missing-backup error")
	}
}
