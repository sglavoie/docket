package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// DomainArtifact is one synced domain file's encoded bytes plus its optional
// optimistic-concurrency baseline.
type DomainArtifact struct {
	Path             string
	Data             []byte
	ExpectedRevision *string
}

// StorageChange converts a synced domain artifact into its canonical
// transaction representation. Every synced write keeps the preceding primary
// bytes in a private .bak sidecar inside the same journal transaction.
func (a DomainArtifact) StorageChange() storage.ArtifactChange {
	return storage.ArtifactChange{
		Path:             a.Path,
		Data:             a.Data,
		Mode:             FilePermissions,
		BackupPath:       a.Path + ".bak",
		ExpectedRevision: a.ExpectedRevision,
	}
}

func (m *Manager) GetConfigPath() string { return m.configPath }

// Exists reports whether any split-domain artifact exists.
func (m *Manager) Exists() bool { return m.domainFilesExist(m.domainStore()) }

func (m *Manager) domainFilePaths() ([]string, error) {
	store := m.domainStore()
	paths := make([]string, 0, len(allDomainKinds))
	for _, kind := range syncedDomainKinds {
		path, err := store.pathFor(kind)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	// Resolve workspace.json before asking for the workspace-ID machine path.
	// An absent catalog means there is no valid machine artifact to enumerate.
	if _, err := os.Stat(WorkspaceCatalogPath(m.workspaceDataDir())); errors.Is(err, os.ErrNotExist) {
		return paths, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to inspect workspace catalog: %w", err)
	}
	if _, err := store.LoadCatalog(); err != nil {
		return nil, err
	}
	machinePath, err := store.pathFor(domainMachine)
	if err != nil {
		return nil, err
	}
	return append(paths, machinePath), nil
}

func (m *Manager) GetBackupPath() string {
	return WorkspaceCatalogPath(m.workspaceDataDir()) + ".bak"
}

func (m *Manager) ConfigBackupExists() bool {
	paths, err := m.domainFilePaths()
	if err != nil {
		return false
	}
	for _, path := range paths {
		if _, err := os.Stat(path + ".bak"); err == nil {
			return true
		}
	}
	return false
}

func (m *Manager) BackupConfig() error {
	paths, err := m.domainFilePaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read %s for backup: %w", filepath.Base(path), err)
		}
		if err := util.AtomicWriteFile(path+".bak", content, FilePermissions); err != nil {
			return fmt.Errorf("failed to write backup for %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

func (m *Manager) RestoreConfigFromBackup() error {
	if !m.ConfigBackupExists() {
		return fmt.Errorf("no config backup available")
	}
	paths, err := m.domainFilePaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		backupPath := path + ".bak"
		backupContent, err := os.ReadFile(backupPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to read backup for %s: %w", filepath.Base(path), err)
		}
		currentContent, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			currentContent = []byte{}
		} else if err != nil {
			return fmt.Errorf("failed to read current %s: %w", filepath.Base(path), err)
		}
		if err := util.AtomicWriteFile(path, backupContent, FilePermissions); err != nil {
			return fmt.Errorf("failed to restore %s: %w", filepath.Base(path), err)
		}
		if err := util.AtomicWriteFile(backupPath, currentContent, FilePermissions); err != nil {
			return fmt.Errorf("failed to update backup for %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
