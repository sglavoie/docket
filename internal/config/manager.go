package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// ErrPartialDomainState is returned when only some synced domain artifacts are
// present. Missing domains are never defaulted because doing so could silently
// discard state after an interrupted write.
var ErrPartialDomainState = errors.New("config: incomplete split-domain configuration; recover the missing domain files before continuing")

// ErrLegacyConfigRequiresMigration is returned when a legacy aggregate config
// exists without any split-domain state. Startup must stop before recovery or
// bootstrap creates files so an explicit migration can choose the source of
// truth safely.
var ErrLegacyConfigRequiresMigration = errors.New("config: legacy configuration requires migration")

// LegacyConfigRequiresMigrationError identifies the aggregate config that must
// be migrated before this workspace can use split-domain persistence.
type LegacyConfigRequiresMigrationError struct {
	Path string
}

func (e *LegacyConfigRequiresMigrationError) Error() string {
	return fmt.Sprintf("config: legacy configuration at %q requires migration before continuing", e.Path)
}

func (e *LegacyConfigRequiresMigrationError) Unwrap() error {
	return ErrLegacyConfigRequiresMigration
}

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

var allDomainKinds = []domainKind{domainCatalog, domainPlanning, domainMachine}
var syncedDomainKinds = []domainKind{domainCatalog, domainPlanning}

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

// rejectLegacyOnlyState refuses an aggregate config only when no synced domain
// marker exists. Complete and partial split workspaces take precedence over a
// stale aggregate file left beside them.
func (m *Manager) rejectLegacyOnlyState(store *DomainFileStore) error {
	state, _ := m.classifyDomainState(store)
	if state != domainStateAbsent {
		return nil
	}
	if _, err := os.Stat(m.configPath); err == nil {
		return &LegacyConfigRequiresMigrationError{Path: m.configPath}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect legacy configuration at %q: %w", m.configPath, err)
	}
	return nil
}

// RecoverWorkspace completes interrupted split-domain transactions after first
// rejecting legacy-only state. Frontends use this guarded entry point so crash
// recovery cannot create transaction directories or locks beside an aggregate
// config that still requires migration.
func (m *Manager) RecoverWorkspace() error {
	store := m.domainStore()
	if err := m.rejectLegacyOnlyState(store); err != nil {
		return err
	}
	dataDir := m.workspaceDataDir()
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, LocalDataRoot()})
	if _, err := commit.RecoverWorkspace(); err != nil {
		return fmt.Errorf("failed to recover workspace before loading split configuration domains: %w", err)
	}
	// Recovery must run before migration inspection: an interrupted activation
	// transaction is roll-forward state, not a fresh migration candidate. Only a
	// complete split-domain workspace is eligible; partial state remains the
	// load path's explicit corruption error.
	if state, _ := m.classifyDomainState(store); state == domainStateComplete {
		if err := m.MigrateWorkspaceToV7(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v7: %w", err)
		}
	}
	return nil
}

// LoadDomains loads the five explicit domains. An uninitialized workspace
// yields defaults; a partial synced set is rejected.
func (m *Manager) LoadDomains() (*Domains, error) {
	store := m.domainStore()
	switch state, partialErr := m.classifyDomainState(store); state {
	case domainStatePartial:
		return nil, partialErr
	case domainStateAbsent:
		if err := m.rejectLegacyOnlyState(store); err != nil {
			return nil, err
		}
		return NewDomains(), nil
	case domainStateComplete:
		// Load the complete split-domain set below.
	}
	return loadDomainBundle(store)
}

func loadDomainBundle(store *DomainFileStore) (*Domains, error) {
	domains, _, err := loadDomainBundleWithRevisions(store)
	return domains, err
}

// LoadOrCreateDomains loads explicit domains and persists their defaults when
// the workspace has not yet been initialized.
func (m *Manager) LoadOrCreateDomains() (*Domains, error) {
	dataDir := m.workspaceDataDir()
	if err := m.RecoverWorkspace(); err != nil {
		return nil, err
	}
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir})

	store := m.domainStore()
	state, partialErr := m.classifyDomainState(store)
	if state == domainStatePartial {
		return nil, partialErr
	}
	if state == domainStateComplete {
		return loadDomainBundle(store)
	}

	domains := NewDomains()
	unfiledArtifact, err := prepareFreshSystemUnfiled(dataDir, domains.Catalog)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare protected system Unfiled: %w", err)
	}
	if err := createDomainBundle(commit, store, domains, unfiledArtifact); err != nil {
		if errors.Is(err, storage.ErrStaleArtifact) {
			return m.loadAfterBootstrapRace(err)
		}
		return nil, fmt.Errorf("failed to create split configuration domains: %w", err)
	}
	return domains, nil
}

// createDomainBundle validates and encodes the complete default bundle before
// the synced transaction creates anything. Machine state lives outside the
// workspace root and is intentionally saved only after all four synced domains
// have committed successfully.
func createDomainBundle(commit *storage.CommitService, store *DomainFileStore, domains *Domains, additional ...storage.ArtifactChange) error {
	if commit == nil {
		return fmt.Errorf("configuration commit service is required")
	}
	if domains == nil || domains.Catalog == nil || domains.Planning == nil || domains.Machine == nil {
		return fmt.Errorf("all configuration domains are required")
	}
	workspaceID, err := store.bindCatalogPayload(domains.Catalog)
	if err != nil {
		return err
	}
	if err := validateDomainBundle(domains); err != nil {
		return err
	}

	type syncedDomain struct {
		kind    domainKind
		payload any
	}
	synced := []syncedDomain{
		{kind: domainCatalog, payload: domains.Catalog},
		{kind: domainPlanning, payload: domains.Planning},
	}
	absentRevision := ""
	changes := make([]storage.ArtifactChange, 0, len(synced)+len(additional))
	for _, domain := range synced {
		path, pathErr := store.pathFor(domain.kind)
		if pathErr != nil {
			return pathErr
		}
		data, encodeErr := encodeDomainEnvelope(domain.kind, workspaceID, domain.payload)
		if encodeErr != nil {
			return encodeErr
		}
		changes = append(changes, DomainArtifact{
			Path:             path,
			Data:             data,
			ExpectedRevision: &absentRevision,
		}.StorageChange())
	}
	changes = append(changes, additional...)
	if _, err := commit.CommitArtifacts(changes); err != nil {
		return err
	}
	if err := store.SaveMachine(domains.Machine); err != nil {
		return fmt.Errorf("failed to save machine-local configuration after synced bootstrap: %w", err)
	}
	return nil
}

// prepareFreshSystemUnfiled installs the protected Unfiled into a brand-new
// catalog and renders its empty task document. The caller commits this artifact
// together with every synced domain, so a clean workspace can never expose a
// catalog entry without its project file (or vice versa).
//
// This helper is intentionally limited to the absent-workspace bootstrap path.
// Established workspaces are loaded strictly and must surface a missing or
// inconsistent Unfiled to diagnostics instead of silently repairing it.
func prepareFreshSystemUnfiled(dataDir string, catalog *WorkspaceCatalog) (storage.ArtifactChange, error) {
	if catalog == nil {
		return storage.ArtifactChange{}, fmt.Errorf("workspace catalog is required")
	}
	project, err := models.NewProject(
		"system-unfiled-"+uuid.NewString(),
		SystemUnfiledName,
		filepath.Join(dataDir, "projects", SystemUnfiledSlug),
	)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	if repairErr := catalog.RepairSystemUnfiledProject(project); repairErr != nil {
		return storage.ArtifactChange{}, repairErr
	}
	document, err := storage.WriteV6ProjectDocument(storage.V6ProjectDocument{
		ProjectName: project.Name,
	})
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	absentRevision := ""
	return storage.ArtifactChange{
		Path:             storage.TasksFilePath(project.Path),
		Data:             []byte(document),
		Mode:             storage.FilePermissions,
		ExpectedRevision: &absentRevision,
	}, nil
}

func validateDomainBundle(domains *Domains) error {
	return validateDomainBundleWithCatalogPolicy(domains, (*WorkspaceCatalog).Validate)
}

func validateDomainBundleForLoad(domains *Domains) error {
	return validateDomainBundleWithCatalogPolicy(domains, (*WorkspaceCatalog).validateForLoad)
}

func validateDomainBundleWithCatalogPolicy(domains *Domains, validateCatalog func(*WorkspaceCatalog) error) error {
	if domains == nil || domains.Catalog == nil || domains.Planning == nil || domains.Machine == nil {
		return fmt.Errorf("all configuration domains are required")
	}
	checks := []struct {
		name     string
		validate func() error
	}{
		{name: "catalog", validate: func() error { return validateCatalog(domains.Catalog) }},
		{name: "planning", validate: domains.Planning.Validate},
		{name: "machine", validate: domains.Machine.Validate},
	}
	for _, check := range checks {
		if err := check.validate(); err != nil {
			return fmt.Errorf("invalid %s domain: %w", check.name, err)
		}
	}
	return nil
}

// loadAfterBootstrapRace resolves the only expected create conflict: another
// initializer committed first. A fresh store must see either the winner's whole
// bundle or an explicit partial state; an absent result preserves the original
// stale-artifact error rather than silently retrying with a new identity.
func (m *Manager) loadAfterBootstrapRace(staleErr error) (*Domains, error) {
	store := m.domainStore()
	switch state, partialErr := m.classifyDomainState(store); state {
	case domainStateComplete:
		return loadDomainBundle(store)
	case domainStatePartial:
		return nil, partialErr
	default:
		return nil, staleErr
	}
}

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
