package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/util"
)

// CurrentDomainSchemaVersion is the on-disk schema version stamped into every
// decomposed-config artifact (workspace/planning/settings/automation/machine
// .json). It is independent of the legacy single-file CurrentConfigVersion so
// the two formats can evolve separately during the migration.
const CurrentDomainSchemaVersion = 1

// ErrStaleObject is returned by the *Checked save methods when the on-disk
// artifact advanced since it was loaded. It is the optimistic-concurrency
// signal a caller uses to reload, re-merge, and retry rather than clobbering a
// concurrent writer's changes.
var ErrStaleObject = errors.New("config: object changed on disk since it was loaded")

// domainKind identifies one of the five decomposed configuration domains.
type domainKind int

const (
	domainCatalog domainKind = iota
	domainPlanning
	domainSettings
	domainAutomation
	domainMachine
)

// name returns the stable kind label written into each artifact's envelope so a
// misplaced or hand-edited file can be recognized.
func (k domainKind) name() string {
	switch k {
	case domainCatalog:
		return "workspace"
	case domainPlanning:
		return "planning"
	case domainSettings:
		return "settings"
	case domainAutomation:
		return "automation"
	case domainMachine:
		return "machine"
	default:
		return "unknown"
	}
}

// domainEnvelope wraps a domain payload on disk with the schema version and
// workspace ID required of every artifact. The payload itself is stored under
// Data so the envelope metadata stays uniform across the five files.
type domainEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
}

// NewWorkspaceID returns a fresh workspace identifier.
func NewWorkspaceID() string {
	return uuid.New().String()
}

// DomainFileStore persists the five decomposed configuration domains to their
// own JSON files, mirroring the durable layout in context.md:
//
//   - <data>/.tt/workspace.json   (synced)  — WorkspaceCatalog
//   - <data>/.tt/planning.json    (synced)  — PlanningState
//   - <data>/.tt/settings.json    (synced)  — Settings
//   - <data>/.tt/automation.json  (synced)  — AutomationConfig
//   - ~/.tt/machine.json for the default local workspace, otherwise
//     ~/.tt/runtime/<data-dir-hash>/machine.json (machine-local) — MachineState
//
// Every file is wrapped in a versioned envelope carrying a schema version and
// the workspace ID, written atomically under a per-file lock with a .bak backup
// and private (0600) permissions. The plain Save* methods (which satisfy the
// narrow domain store interfaces) are last-writer-wins but still locked, atomic,
// and backed up; the Save*Checked variants add optimistic concurrency by
// failing with ErrStaleObject when the on-disk revision no longer matches the
// one observed at load time.
//
// This is additive persistence infrastructure: the legacy single-file Config
// remains canonical until later sessions route the application through these
// per-domain files. Because Decompose/ApplyTo cover only each domain's owned
// subset of Config, flipping the canonical format is intentionally out of scope
// here.
type DomainFileStore struct {
	dataDir     string
	workspaceID string
}

// NewDomainFileStore roots a store at a data directory with no known workspace
// ID; the ID is adopted from the first artifact read, or minted via
// EnsureWorkspaceID.
func NewDomainFileStore(dataDir string) *DomainFileStore {
	return &DomainFileStore{dataDir: filepath.Clean(dataDir)}
}

// NewDomainFileStoreWithWorkspace roots a store at a data directory with a known
// workspace ID that is stamped into every artifact it writes.
func NewDomainFileStoreWithWorkspace(dataDir, workspaceID string) *DomainFileStore {
	return &DomainFileStore{dataDir: filepath.Clean(dataDir), workspaceID: workspaceID}
}

// DataDir returns the store's workspace data directory.
func (s *DomainFileStore) DataDir() string { return s.dataDir }

// WorkspaceID returns the workspace ID the store stamps into artifacts. It may
// be empty until an artifact is read or EnsureWorkspaceID is called.
func (s *DomainFileStore) WorkspaceID() string { return s.workspaceID }

// EnsureWorkspaceID mints a workspace ID when the store has none yet and returns
// the effective ID. It does not persist anything on its own; the ID is written
// the next time an artifact is saved.
func (s *DomainFileStore) EnsureWorkspaceID() string {
	if s.workspaceID == "" {
		s.workspaceID = NewWorkspaceID()
	}
	return s.workspaceID
}

// pathFor resolves the on-disk path of a domain artifact.
func (s *DomainFileStore) pathFor(kind domainKind) string {
	switch kind {
	case domainCatalog:
		return WorkspaceCatalogPath(s.dataDir)
	case domainPlanning:
		return PlanningStatePath(s.dataDir)
	case domainSettings:
		return SettingsPath(s.dataDir)
	case domainAutomation:
		return AutomationPath(s.dataDir)
	case domainMachine:
		return MachineStatePath(s.dataDir)
	default:
		return filepath.Join(InternalDir(s.dataDir), kind.name()+".json")
	}
}

// revisionOf returns the content revision (hex SHA-256) of raw artifact bytes.
// An empty revision denotes "no artifact observed" and disables the concurrency
// check on save.
func revisionOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// load reads an artifact into out and returns its content revision. A missing
// file yields ("", nil) so the caller keeps its seeded defaults; corruption
// falls back to the .bak sidecar before failing.
func (s *DomainFileStore) load(kind domainKind, out any) (string, error) {
	path := s.pathFor(kind)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", filepath.Base(path), err)
	}

	rev := revisionOf(data)
	env, decErr := decodeEnvelope(data)
	if decErr != nil {
		recovered, recErr := s.recoverEnvelopeFromBackup(path, decErr)
		if recErr != nil {
			return rev, recErr
		}
		env = recovered
	}
	if env.SchemaVersion > CurrentDomainSchemaVersion {
		return rev, fmt.Errorf("%s schema version %d not supported, please upgrade tt", filepath.Base(path), env.SchemaVersion)
	}
	if s.workspaceID == "" && env.WorkspaceID != "" {
		s.workspaceID = env.WorkspaceID
	}
	if len(env.Data) == 0 {
		return rev, nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return rev, fmt.Errorf("failed to parse %s payload: %w", filepath.Base(path), err)
	}
	return rev, nil
}

// encodeDomainEnvelope renders a domain payload into its on-disk envelope bytes,
// exactly as DomainFileStore.save writes them. It is the single encode point
// shared by the store's own writes and the transactional artifact encoder so a
// domain file produced by a transaction is byte-compatible with one produced by
// Manager.Save, and a reload observes the same content either way.
func encodeDomainEnvelope(kind domainKind, workspaceID string, payload any) ([]byte, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s payload: %w", kind.name(), err)
	}
	env := domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          kind.name(),
		Data:          rawPayload,
	}
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s: %w", kind.name(), err)
	}
	return out, nil
}

func decodeEnvelope(data []byte) (domainEnvelope, error) {
	var env domainEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return domainEnvelope{}, err
	}
	return env, nil
}

// recoverEnvelopeFromBackup restores an artifact from its .bak sidecar when the
// primary is corrupted, re-persisting the recovered bytes so later loads do not
// hit the corruption again.
func (s *DomainFileStore) recoverEnvelopeFromBackup(path string, primaryErr error) (domainEnvelope, error) {
	backupData, err := os.ReadFile(path + ".bak")
	if err != nil {
		return domainEnvelope{}, fmt.Errorf("failed to parse %s (no backup available): %w", filepath.Base(path), primaryErr)
	}
	env, err := decodeEnvelope(backupData)
	if err != nil {
		return domainEnvelope{}, fmt.Errorf("failed to parse %s and backup is also corrupted: %w", filepath.Base(path), primaryErr)
	}
	if writeErr := util.AtomicWriteFile(path, backupData, FilePermissions); writeErr != nil {
		return domainEnvelope{}, fmt.Errorf("recovered %s from backup but failed to persist it: %w", filepath.Base(path), writeErr)
	}
	return env, nil
}

// save writes an artifact atomically under a per-file lock with a backup and
// private permissions. When expectedRev is non-empty the on-disk revision is
// verified inside the lock and a mismatch (including a vanished file) returns
// ErrStaleObject without writing.
func (s *DomainFileStore) save(kind domainKind, payload any, expectedRev string) error {
	path := s.pathFor(kind)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// A workspace-catalog save is the authoritative source of the workspace ID;
	// adopt it so subsequent artifacts written by this store agree.
	if wc, ok := payload.(*WorkspaceCatalog); ok && wc.WorkspaceID != "" {
		s.workspaceID = wc.WorkspaceID
	}

	lockPath := path + ".lock"
	return util.WithFileLock(lockPath, true, func() error {
		if expectedRev != "" {
			current, err := os.ReadFile(path)
			switch {
			case errors.Is(err, os.ErrNotExist):
				return ErrStaleObject
			case err != nil:
				return fmt.Errorf("failed to re-read %s for concurrency check: %w", filepath.Base(path), err)
			default:
				if revisionOf(current) != expectedRev {
					return ErrStaleObject
				}
			}
		}

		backupDomainArtifact(path)

		out, err := encodeDomainEnvelope(kind, s.workspaceID, payload)
		if err != nil {
			return err
		}
		if err := util.AtomicWriteFile(path, out, FilePermissions); err != nil {
			return fmt.Errorf("failed to write %s: %w", filepath.Base(path), err)
		}
		return nil
	})
}

// backupDomainArtifact best-effort copies an existing artifact to its .bak
// sidecar before it is overwritten, mirroring the single-file manager's backup.
func backupDomainArtifact(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return // nothing to back up (first write) or unreadable; overwrite proceeds
	}
	if err := util.AtomicWriteFile(path+".bak", content, FilePermissions); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to back up %s: %v\n", filepath.Base(path), err)
	}
}

// LoadCatalog implements WorkspaceCatalogStore.
func (s *DomainFileStore) LoadCatalog() (*WorkspaceCatalog, error) {
	c, _, err := s.LoadCatalogChecked()
	return c, err
}

// LoadCatalogChecked loads the catalog and returns the revision to pass back to
// SaveCatalogChecked for optimistic concurrency.
func (s *DomainFileStore) LoadCatalogChecked() (*WorkspaceCatalog, string, error) {
	out := NewWorkspaceCatalog()
	rev, err := s.load(domainCatalog, out)
	if err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// SaveCatalog implements WorkspaceCatalogStore (last-writer-wins).
func (s *DomainFileStore) SaveCatalog(catalog *WorkspaceCatalog) error {
	return s.SaveCatalogChecked(catalog, "")
}

// SaveCatalogChecked saves the catalog, failing with ErrStaleObject when the
// on-disk revision no longer matches rev.
func (s *DomainFileStore) SaveCatalogChecked(catalog *WorkspaceCatalog, rev string) error {
	if catalog == nil {
		return fmt.Errorf("config: nil workspace catalog")
	}
	return s.save(domainCatalog, catalog, rev)
}

// LoadPlanning implements PlanningStore.
func (s *DomainFileStore) LoadPlanning() (*PlanningState, error) {
	p, _, err := s.LoadPlanningChecked()
	return p, err
}

// LoadPlanningChecked loads planning state and its revision.
func (s *DomainFileStore) LoadPlanningChecked() (*PlanningState, string, error) {
	out := NewPlanningState()
	rev, err := s.load(domainPlanning, out)
	if err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// SavePlanning implements PlanningStore (last-writer-wins).
func (s *DomainFileStore) SavePlanning(planning *PlanningState) error {
	return s.SavePlanningChecked(planning, "")
}

// SavePlanningChecked saves planning state with an optimistic concurrency check.
func (s *DomainFileStore) SavePlanningChecked(planning *PlanningState, rev string) error {
	if planning == nil {
		return fmt.Errorf("config: nil planning state")
	}
	return s.save(domainPlanning, planning, rev)
}

// LoadSettings implements SettingsStore.
func (s *DomainFileStore) LoadSettings() (*Settings, error) {
	st, _, err := s.LoadSettingsChecked()
	return st, err
}

// LoadSettingsChecked loads settings and their revision.
func (s *DomainFileStore) LoadSettingsChecked() (*Settings, string, error) {
	out := NewSettings()
	rev, err := s.load(domainSettings, out)
	if err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// SaveSettings implements SettingsStore (last-writer-wins).
func (s *DomainFileStore) SaveSettings(settings *Settings) error {
	return s.SaveSettingsChecked(settings, "")
}

// SaveSettingsChecked saves settings with an optimistic concurrency check.
func (s *DomainFileStore) SaveSettingsChecked(settings *Settings, rev string) error {
	if settings == nil {
		return fmt.Errorf("config: nil settings")
	}
	return s.save(domainSettings, settings, rev)
}

// LoadAutomation implements AutomationStore.
func (s *DomainFileStore) LoadAutomation() (*AutomationConfig, error) {
	a, _, err := s.LoadAutomationChecked()
	return a, err
}

// LoadAutomationChecked loads automation config and its revision.
func (s *DomainFileStore) LoadAutomationChecked() (*AutomationConfig, string, error) {
	out := NewAutomationConfig()
	rev, err := s.load(domainAutomation, out)
	if err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// SaveAutomation implements AutomationStore (last-writer-wins).
func (s *DomainFileStore) SaveAutomation(automation *AutomationConfig) error {
	return s.SaveAutomationChecked(automation, "")
}

// SaveAutomationChecked saves automation config with an optimistic concurrency
// check.
func (s *DomainFileStore) SaveAutomationChecked(automation *AutomationConfig, rev string) error {
	if automation == nil {
		return fmt.Errorf("config: nil automation config")
	}
	return s.save(domainAutomation, automation, rev)
}

// LoadMachine implements MachineStore.
func (s *DomainFileStore) LoadMachine() (*MachineState, error) {
	m, _, err := s.LoadMachineChecked()
	return m, err
}

// LoadMachineChecked loads machine state and its revision.
func (s *DomainFileStore) LoadMachineChecked() (*MachineState, string, error) {
	out := NewMachineState()
	rev, err := s.load(domainMachine, out)
	if err != nil {
		return nil, "", err
	}
	return out, rev, nil
}

// SaveMachine implements MachineStore (last-writer-wins).
func (s *DomainFileStore) SaveMachine(machine *MachineState) error {
	return s.SaveMachineChecked(machine, "")
}

// SaveMachineChecked saves machine state with an optimistic concurrency check.
func (s *DomainFileStore) SaveMachineChecked(machine *MachineState, rev string) error {
	if machine == nil {
		return fmt.Errorf("config: nil machine state")
	}
	return s.save(domainMachine, machine, rev)
}

// Compile-time checks that the file store satisfies every domain store
// interface. It is the canonical implementation, persisting each domain to its
// own synced file.
var (
	_ WorkspaceCatalogStore = (*DomainFileStore)(nil)
	_ PlanningStore         = (*DomainFileStore)(nil)
	_ SettingsStore         = (*DomainFileStore)(nil)
	_ AutomationStore       = (*DomainFileStore)(nil)
	_ MachineStore          = (*DomainFileStore)(nil)
)
