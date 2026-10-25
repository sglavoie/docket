package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
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

// ErrCorruptWorkspaceCatalog is returned when the synced workspace catalog
// exists but its bytes do not decode as a domain envelope.
var ErrCorruptWorkspaceCatalog = errors.New("config: workspace catalog is not valid JSON")

// CorruptWorkspaceCatalogError names the catalog that failed to decode and
// where a good copy may be found. It is reported before the migration chain
// runs, so the failure names the file rather than whichever schema boundary
// happened to read it first.
type CorruptWorkspaceCatalogError struct {
	Path string
	// BackupPath is the catalog's .bak sidecar when one exists, else empty.
	BackupPath string
	// BackupsDir holds the workspace's migration snapshots.
	BackupsDir string
	Err        error
}

func (e *CorruptWorkspaceCatalogError) Error() string {
	restore := fmt.Sprintf("restore it from a snapshot under %s", DisplayPath(e.BackupsDir))
	if e.BackupPath != "" {
		restore = fmt.Sprintf("restore it from its last good copy %s or from a snapshot under %s",
			DisplayPath(e.BackupPath), DisplayPath(e.BackupsDir))
	}
	return fmt.Sprintf("workspace catalog %s is not valid JSON (%v); %s, then run 'docket doctor'",
		DisplayPath(e.Path), e.Err, restore)
}

func (e *CorruptWorkspaceCatalogError) Unwrap() []error {
	return []error{ErrCorruptWorkspaceCatalog, e.Err}
}

// checkWorkspaceCatalogDecodes refuses a catalog whose bytes are not a domain
// envelope. An absent or unreadable file is left to the paths that already
// report it, and nothing is read from or written to the .bak sidecar: choosing
// which copy is the truth stays the operator's decision.
func checkWorkspaceCatalogDecodes(dataDir string) error {
	path := WorkspaceCatalogPath(dataDir)
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil //nolint:nilerr // an unreadable catalog is reported by the load path that needs it
	}
	if _, decErr := decodeEnvelope(data); decErr != nil {
		corrupt := &CorruptWorkspaceCatalogError{
			Path:       path,
			BackupsDir: PreScopeV7BackupsDir(dataDir),
			Err:        decErr,
		}
		if info, statErr := os.Stat(path + ".bak"); statErr == nil && info.Mode().IsRegular() {
			corrupt.BackupPath = path + ".bak"
		}
		return corrupt
	}
	return nil
}

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
	// machineHome names the machine identity this manager resolves machine-local
	// state beneath. Empty means this process's own home, which is what every
	// production caller uses; only a caller that explicitly names a home departs
	// from $HOME.
	machineHome string
	// deviceID, when set, is the transaction owner this manager stamps instead
	// of the identity derived from machineHome and the hostname. A frontend that
	// names its own stable device identity sets it, so recovery and every commit
	// it opens agree on one owner without consulting the host environment.
	deviceID string
}

// NewManager creates a workspace configuration manager. The path identifies
// the workspace root for compatibility with the existing --config resolver;
// durable configuration lives in the split domain files under that root.
// Machine-local state resolves beneath this process's home.
func NewManager(customPath string) *Manager {
	return NewManagerOnMachine(customPath, "")
}

// NewManagerOnMachine creates a workspace configuration manager that resolves
// machine-local state beneath a named machine home instead of $HOME. Two
// managers over one data directory with distinct homes are two machines sharing
// one workspace; because neither reads the environment, they may be driven
// concurrently, which a process-global $HOME cannot express.
func NewManagerOnMachine(customPath, machineHome string) *Manager {
	if customPath != "" {
		return &Manager{configPath: customPath, machineHome: machineHome}
	}
	return &Manager{configPath: defaultConfigPathOn(machineHome), fromDefaultPath: true, machineHome: machineHome}
}

// NewManagerForDevice creates a workspace configuration manager whose
// machine-local state resolves beneath machineHome and whose transactions are
// owned by deviceID. Both are explicit, so neither $HOME nor the hostname is
// read: this is the constructor a frontend without desktop home-directory
// conventions uses.
func NewManagerForDevice(customPath, machineHome, deviceID string) *Manager {
	manager := NewManagerOnMachine(customPath, machineHome)
	manager.deviceID = deviceID
	return manager
}

func (m *Manager) CreatedFromDefaultPath() bool { return m.fromDefaultPath }

// MachineHome returns the machine identity this manager resolves machine-local
// state beneath, or the empty string when it follows this process's home.
func (m *Manager) MachineHome() string { return m.machineHome }

func (m *Manager) workspaceDataDir() string {
	return WorkspaceRootFromConfigPath(m.configPath)
}

// localDataRoot is this manager's machine-local ~/.docket root. Every approved
// commit root and runtime path must come through here rather than the package
// LocalDataRoot(), or a named machine would write its transactions beneath the
// process home instead of its own.
func (m *Manager) localDataRoot() string {
	return localDataRootOn(m.machineHome)
}

// machineID is the transaction-ownership identity of the machine this manager
// speaks for. Every commit service the manager builds is bound to it, so a
// manager named after another machine's home stamps that machine onto the
// manifests it writes and does not claim the real machine's live transactions.
func (m *Manager) machineID() string {
	if m.deviceID != "" {
		return m.deviceID
	}
	return transaction.MachineIDForHome(m.machineHome)
}

// TransactionOwner is the machine identity bound to this manager, or the empty
// string when it follows the running process. The empty answer is deliberate: a
// commit service handed it recomputes this machine on every journal, so a
// process whose home changes is a different machine, while a manager that named
// a home or a device keeps that one owner.
func (m *Manager) TransactionOwner() string {
	if m.deviceID == "" && m.machineHome == "" {
		return ""
	}
	return m.machineID()
}

// LocalDataRoot is this manager's machine-local ~/.docket root, the approved commit
// root beside the workspace that machine.json writes resolve beneath.
func (m *Manager) LocalDataRoot() string {
	return m.localDataRoot()
}

func (m *Manager) domainStore() *DomainFileStore {
	return NewDomainFileStoreOnMachine(m.workspaceDataDir(), m.machineHome)
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
	// A data directory that is a file makes every path beneath it fail with
	// ENOTDIR; name the directory itself instead of the first child probed.
	if info, err := os.Stat(m.workspaceDataDir()); err == nil && !info.IsDir() {
		return fmt.Errorf("workspace data directory %s is not a directory", absoluteArtifactPath(m.workspaceDataDir()))
	}
	if err := pendingV24LayoutError(m.workspaceDataDir()); err != nil {
		return err
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
	// The layout activation comes first: every reader below looks under
	// .docket/, so a pre-v24 .tt/ must have moved before anything is read.
	if err := m.ActivateV24Layout(); err != nil {
		return fmt.Errorf("failed to activate the schema-24 workspace layout: %w", err)
	}
	store := m.domainStore()
	if err := m.rejectLegacyOnlyState(store); err != nil {
		return err
	}
	dataDir := m.workspaceDataDir()
	commit := storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir, m.localDataRoot()}).OnMachine(m.machineID())
	if _, err := commit.RecoverWorkspace(); err != nil {
		return fmt.Errorf("failed to recover workspace before loading split configuration domains: %w", err)
	}
	// Recovery must run before migration inspection: an interrupted activation
	// transaction is roll-forward state, not a fresh migration candidate. Only a
	// complete split-domain workspace is eligible; partial state remains the
	// load path's explicit corruption error.
	if state, _ := m.classifyDomainState(store); state == domainStateComplete {
		if err := checkWorkspaceCatalogDecodes(dataDir); err != nil {
			return err
		}
		if err := m.MigrateWorkspaceToV12(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v12: %w", err)
		}
		if err := m.MigrateWorkspaceToV13(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v13: %w", err)
		}
		if err := m.MigrateWorkspaceToV14(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v14: %w", err)
		}
		if err := m.MigrateWorkspaceToV15(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v15: %w", err)
		}
		if err := m.MigrateWorkspaceToV16(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v16: %w", err)
		}
		if err := m.MigrateWorkspaceToV17(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v17: %w", err)
		}
		if err := m.MigrateWorkspaceToV18(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v18: %w", err)
		}
		if err := m.MigrateWorkspaceToV19(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v19: %w", err)
		}
		if err := m.MigrateWorkspaceToV20(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v20: %w", err)
		}
		if err := m.MigrateWorkspaceToV21(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v21: %w", err)
		}
		if err := m.MigrateWorkspaceToV23(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v23: %w", err)
		}
		if err := m.MigrateWorkspaceToV24(); err != nil {
			return fmt.Errorf("failed to activate workspace schema v24: %w", err)
		}
		// Adoption is deliberately outside the chain above: it is keyed off
		// machine-local state rather than the synced catalog schema, so it runs
		// once per machine after the schema is already current instead of once
		// per workspace at the boundary.
		if err := m.AdoptPortableLenses(); err != nil {
			return fmt.Errorf("failed to adopt portable lens definitions: %w", err)
		}
	}
	return nil
}
