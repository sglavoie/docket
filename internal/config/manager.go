package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

// ErrPartialDomainState is returned when only some synced domain artifacts are
// present. Missing domains are never defaulted because doing so could silently
// discard state after an interrupted write.
var ErrPartialDomainState = errors.New("config: incomplete split-domain configuration; recover the missing domain files before continuing")

// PartialDomainStateError names the present and missing synced domains.
type PartialDomainStateError struct {
	Present []string
	Missing []string
}

func (e *PartialDomainStateError) Error() string {
	return fmt.Sprintf("config: incomplete split-domain configuration (present: %s; missing: %s); recover the missing files (e.g. from their .bak sidecars) before continuing", strings.Join(e.Present, ", "), strings.Join(e.Missing, ", "))
}

func (e *PartialDomainStateError) Unwrap() error { return ErrPartialDomainState }

type domainStateKind int

const (
	domainStateAbsent domainStateKind = iota
	domainStateComplete
	domainStatePartial
)

// Manager roots domain stores and other workspace-scoped configuration
// services. Runtime callers load and commit explicit domains; Manager never
// reconstructs or saves a whole-configuration aggregate.
type Manager struct {
	configPath      string
	fromDefaultPath bool
}

// NewManager creates a workspace configuration manager. The path identifies
// the workspace root for compatibility with the existing --config resolver;
// durable configuration lives in the split domain files under that root.
func NewManager(customPath string) *Manager {
	if customPath != "" {
		return &Manager{configPath: customPath}
	}
	return &Manager{configPath: DefaultConfigPath(), fromDefaultPath: true}
}

func (m *Manager) CreatedFromDefaultPath() bool { return m.fromDefaultPath }

func (m *Manager) workspaceDataDir() string {
	return WorkspaceRootFromConfigPath(m.configPath)
}

func (m *Manager) domainStore() *DomainFileStore {
	return NewDomainFileStore(m.workspaceDataDir())
}

var allDomainKinds = []domainKind{domainCatalog, domainPlanning, domainSettings, domainAutomation, domainMachine}
var syncedDomainKinds = []domainKind{domainCatalog, domainPlanning, domainSettings, domainAutomation}

func (m *Manager) domainFilesExist(store *DomainFileStore) bool {
	// Synced domains establish whether a workspace exists. machine.json cannot
	// be addressed until workspace.json has supplied a validated workspace ID.
	for _, kind := range syncedDomainKinds {
		path, err := store.pathFor(kind)
		if err == nil {
			if _, statErr := os.Stat(path); statErr == nil {
				return true
			}
		}
	}
	return false
}

func (m *Manager) classifyDomainState(store *DomainFileStore) (domainStateKind, *PartialDomainStateError) {
	present := make([]string, 0, len(syncedDomainKinds))
	missing := make([]string, 0, len(syncedDomainKinds))
	for _, kind := range syncedDomainKinds {
		path, err := store.pathFor(kind)
		if err == nil {
			_, err = os.Stat(path)
		}
		if err == nil {
			present = append(present, kind.name())
		} else {
			missing = append(missing, kind.name())
		}
	}
	switch len(present) {
	case 0:
		return domainStateAbsent, nil
	case len(syncedDomainKinds):
		return domainStateComplete, nil
	default:
		return domainStatePartial, &PartialDomainStateError{Present: present, Missing: missing}
	}
}

// LoadDomains loads the five explicit domains. An uninitialized workspace
// yields defaults; a partial synced set is rejected.
func (m *Manager) LoadDomains() (*Domains, error) {
	store := m.domainStore()
	switch state, partialErr := m.classifyDomainState(store); state {
	case domainStatePartial:
		return nil, partialErr
	case domainStateAbsent:
		return NewDomains(), nil
	case domainStateComplete:
		// Load the complete split-domain set below.
	}

	catalog, err := store.LoadCatalog()
	if err != nil {
		return nil, err
	}
	planning, err := store.LoadPlanning()
	if err != nil {
		return nil, err
	}
	settings, err := store.LoadSettings()
	if err != nil {
		return nil, err
	}
	automation, err := store.LoadAutomation()
	if err != nil {
		return nil, err
	}
	machine, err := store.LoadMachine()
	if err != nil {
		return nil, err
	}
	return &Domains{Catalog: catalog, Planning: planning, Settings: settings, Automation: automation, Machine: machine}, nil
}

// LoadOrCreateDomains loads explicit domains and persists their defaults when
// the workspace has not yet been initialized.
func (m *Manager) LoadOrCreateDomains() (*Domains, error) {
	store := m.domainStore()
	state, partialErr := m.classifyDomainState(store)
	if state == domainStatePartial {
		return nil, partialErr
	}
	domains, err := m.LoadDomains()
	if err != nil {
		return nil, err
	}
	if state == domainStateComplete {
		return domains, nil
	}
	if err := saveDomainBundle(store, domains); err != nil {
		return nil, fmt.Errorf("failed to create split configuration domains: %w", err)
	}
	return domains, nil
}

func saveDomainBundle(store *DomainFileStore, domains *Domains) error {
	if domains == nil || domains.Catalog == nil || domains.Planning == nil || domains.Settings == nil || domains.Automation == nil || domains.Machine == nil {
		return fmt.Errorf("all configuration domains are required")
	}
	if err := store.SaveCatalog(domains.Catalog); err != nil {
		return err
	}
	if err := store.SavePlanning(domains.Planning); err != nil {
		return err
	}
	if err := store.SaveSettings(domains.Settings); err != nil {
		return err
	}
	if err := store.SaveAutomation(domains.Automation); err != nil {
		return err
	}
	return store.SaveMachine(domains.Machine)
}

// DomainArtifact is one synced domain file's encoded bytes plus its optional
// optimistic-concurrency baseline.
type DomainArtifact struct {
	Path             string
	Data             []byte
	ExpectedRevision *string
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
