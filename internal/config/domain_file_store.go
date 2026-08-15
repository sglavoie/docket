package config

import (
	"bytes"
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
// runtime configuration artifact. It is independent of the legacy single-file
// CurrentConfigVersion so the two formats can evolve separately.
const CurrentDomainSchemaVersion = 1

// ErrStaleObject is returned by the *Checked save methods when the on-disk
// artifact advanced since it was loaded. It is the optimistic-concurrency
// signal a caller uses to reload, re-merge, and retry rather than clobbering a
// concurrent writer's changes.
var ErrStaleObject = errors.New("config: object changed on disk since it was loaded")

// ErrInvalidWorkspaceID reports an identity that is not a canonical UUID.
// Workspace IDs are path components for machine-local state, so accepting an
// arbitrary string would permit aliasing or path traversal.
var ErrInvalidWorkspaceID = errors.New("config: workspace ID must be a canonical UUID")

// ErrWorkspaceIDMismatch reports two artifacts (or an artifact and an already
// bound store) that claim different workspace identities. The catalog identity
// is immutable for a workspace and may never be silently replaced.
var ErrWorkspaceIDMismatch = errors.New("config: workspace ID mismatch")

// ErrWorkspaceIDRequired reports machine-state access attempted before the
// synced workspace catalog established the workspace identity.
var ErrWorkspaceIDRequired = errors.New("config: workspace ID is required")

// domainKind identifies one of the three runtime configuration domains.
type domainKind int

const (
	domainCatalog domainKind = iota
	domainPlanning
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
	case domainMachine:
		return "machine"
	default:
		return "unknown"
	}
}

// domainEnvelope wraps a domain payload on disk with the schema version and
// workspace ID required of every artifact. The payload itself is stored under
// Data so the envelope metadata stays uniform across the three runtime files.
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

// validateWorkspaceID accepts only the canonical lower-case hyphenated UUID
// representation produced by NewWorkspaceID. Besides keeping one spelling per
// identity, this makes the value safe to use as a single path component.
func validateWorkspaceID(workspaceID string) error {
	parsed, err := uuid.Parse(workspaceID)
	if err != nil || parsed.String() != workspaceID {
		return fmt.Errorf("%w: %q", ErrInvalidWorkspaceID, workspaceID)
	}
	return nil
}

// DomainFileStore persists the three runtime configuration domains to their
// own JSON files, mirroring the durable layout in context.md:
//
//   - <data>/.tt/workspace.json   (synced)  — WorkspaceCatalog
//   - <data>/.tt/planning.json    (synced)  — PlanningState
//   - ~/.tt/runtime/<workspace-id>/machine.json (machine-local) — MachineState
//
// Every file is wrapped in a versioned envelope carrying a schema version and
// the workspace ID, written atomically under a per-file lock with a .bak backup
// and private (0600) permissions. The plain Save* methods (which satisfy the
// narrow domain store interfaces) are last-writer-wins but still locked, atomic,
// and backed up; the Save*Checked variants add optimistic concurrency by
// failing with ErrStaleObject when the on-disk revision no longer matches the
// one observed at load time.
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

// DataDir returns the store's workspace data directory.
func (s *DomainFileStore) DataDir() string { return s.dataDir }

// WorkspaceID returns the workspace ID the store stamps into artifacts. It may
// be empty until an artifact is read or EnsureWorkspaceID is called.
func (s *DomainFileStore) WorkspaceID() string { return s.workspaceID }

// BindWorkspaceID validates and binds the store to one immutable workspace
// identity. Rebinding to the same UUID is idempotent; attempting to adopt a
// different identity fails explicitly.
func (s *DomainFileStore) BindWorkspaceID(workspaceID string) error {
	if err := validateWorkspaceID(workspaceID); err != nil {
		return err
	}
	if s.workspaceID != "" && s.workspaceID != workspaceID {
		return fmt.Errorf("%w: store is bound to %q, artifact has %q", ErrWorkspaceIDMismatch, s.workspaceID, workspaceID)
	}
	s.workspaceID = workspaceID
	return nil
}

// EnsureWorkspaceID mints a workspace ID when the store has none yet and returns
// the effective ID. It does not persist anything on its own; the ID is written
// the next time an artifact is saved.
func (s *DomainFileStore) EnsureWorkspaceID() string {
	if s.workspaceID == "" {
		s.workspaceID = NewWorkspaceID()
	}
	return s.workspaceID
}

func (s *DomainFileStore) validatedWorkspaceID() (string, error) {
	workspaceID := s.WorkspaceID()
	if workspaceID == "" {
		return "", ErrWorkspaceIDRequired
	}
	if err := validateWorkspaceID(workspaceID); err != nil {
		return "", err
	}
	return workspaceID, nil
}

// bindCatalogPayload makes catalog identity authoritative for this store. A
// fresh catalog receives the store's one minted identity; an existing identity
// must match the identity already bound from disk or from the caller.
func (s *DomainFileStore) bindCatalogPayload(catalog *WorkspaceCatalog) (string, error) {
	if catalog == nil {
		return "", fmt.Errorf("config: nil workspace catalog")
	}
	if catalog.WorkspaceID == "" {
		catalog.WorkspaceID = s.EnsureWorkspaceID()
	}
	if err := s.BindWorkspaceID(catalog.WorkspaceID); err != nil {
		return "", err
	}
	return s.validatedWorkspaceID()
}

// pathFor resolves the on-disk path of a domain artifact.
func (s *DomainFileStore) pathFor(kind domainKind) (string, error) {
	switch kind {
	case domainCatalog:
		return WorkspaceCatalogPath(s.dataDir), nil
	case domainPlanning:
		return PlanningStatePath(s.dataDir), nil
	case domainMachine:
		workspaceID, err := s.validatedWorkspaceID()
		if err != nil {
			return "", err
		}
		return MachineStatePath(workspaceID)
	default:
		return filepath.Join(InternalDir(s.dataDir), kind.name()+".json"), nil
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
	path, err := s.pathFor(kind)
	if err != nil {
		return "", err
	}
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
		recovered, recoveredData, recErr := s.recoverEnvelopeFromBackup(path, decErr)
		if recErr != nil {
			return rev, recErr
		}
		env = recovered
		rev = revisionOf(recoveredData)
	}
	if env.SchemaVersion != CurrentDomainSchemaVersion {
		return rev, fmt.Errorf("%s envelope schema version %d is not supported; want %d", filepath.Base(path), env.SchemaVersion, CurrentDomainSchemaVersion)
	}
	if env.Kind != kind.name() {
		return rev, fmt.Errorf("%s contains %q domain data; want %q", filepath.Base(path), env.Kind, kind.name())
	}
	if err := s.BindWorkspaceID(env.WorkspaceID); err != nil {
		return rev, fmt.Errorf("invalid workspace identity in %s: %w", filepath.Base(path), err)
	}
	if len(env.Data) == 0 {
		return rev, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(env.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return rev, fmt.Errorf("failed to parse %s payload: %w", filepath.Base(path), err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return rev, fmt.Errorf("failed to parse %s payload: %w", filepath.Base(path), err)
	}
	if catalog, ok := out.(*WorkspaceCatalog); ok {
		if catalog.WorkspaceID == "" {
			return rev, fmt.Errorf("invalid workspace identity in %s payload: %w", filepath.Base(path), ErrWorkspaceIDRequired)
		}
		if err := s.BindWorkspaceID(catalog.WorkspaceID); err != nil {
			return rev, fmt.Errorf("invalid workspace identity in %s payload: %w", filepath.Base(path), err)
		}
	}
	return rev, nil
}

// encodeDomainEnvelope renders a domain payload into its on-disk envelope bytes,
// exactly as DomainFileStore.save writes them. It is the single encode point
// shared by the store's own writes and the transactional artifact encoder so a
// domain file produced by a transaction is byte-compatible with one produced by
// Manager.Save, and a reload observes the same content either way.
func encodeDomainEnvelope(kind domainKind, workspaceID string, payload any) ([]byte, error) {
	if err := validateWorkspaceID(workspaceID); err != nil {
		return nil, err
	}
	if catalog, ok := payload.(*WorkspaceCatalog); ok {
		if err := catalog.Validate(); err != nil {
			return nil, fmt.Errorf("invalid workspace catalog: %w", err)
		}
	}
	if planning, ok := payload.(*PlanningState); ok {
		if err := planning.Validate(); err != nil {
			return nil, fmt.Errorf("invalid planning state: %w", err)
		}
	}
	if machine, ok := payload.(*MachineState); ok {
		if err := machine.Validate(); err != nil {
			return nil, fmt.Errorf("invalid machine state: %w", err)
		}
	}
	return marshalDomainEnvelope(kind, workspaceID, payload)
}

// encodeMigrationCatalogEnvelope renders the catalog a schema migration is about
// to activate. It validates against the schema the migration targets rather than
// the schema this build loads, which is the one thing encodeDomainEnvelope
// cannot do: an activation publishes its target schema before that target is
// loadable. It shares marshalDomainEnvelope so the bytes stay identical to an
// ordinary save.
func encodeMigrationCatalogEnvelope(catalog *WorkspaceCatalog, targetSchema int) ([]byte, error) {
	if err := validateWorkspaceID(catalog.WorkspaceID); err != nil {
		return nil, err
	}
	if err := catalog.validateAtSchema(targetSchema); err != nil {
		return nil, fmt.Errorf("invalid workspace catalog: %w", err)
	}
	return marshalDomainEnvelope(domainCatalog, catalog.WorkspaceID, catalog)
}

func marshalDomainEnvelope(kind domainKind, workspaceID string, payload any) ([]byte, error) {
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
func (s *DomainFileStore) recoverEnvelopeFromBackup(path string, primaryErr error) (domainEnvelope, []byte, error) {
	backupData, err := os.ReadFile(path + ".bak")
	if err != nil {
		return domainEnvelope{}, nil, fmt.Errorf("failed to parse %s (no backup available): %w", filepath.Base(path), primaryErr)
	}
	env, err := decodeEnvelope(backupData)
	if err != nil {
		return domainEnvelope{}, nil, fmt.Errorf("failed to parse %s and backup is also corrupted: %w", filepath.Base(path), primaryErr)
	}
	if writeErr := util.AtomicWriteFile(path, backupData, FilePermissions); writeErr != nil {
		return domainEnvelope{}, nil, fmt.Errorf("recovered %s from backup but failed to persist it: %w", filepath.Base(path), writeErr)
	}
	return env, backupData, nil
}

// save writes an artifact atomically under a per-file lock with a backup and
// private permissions. When expectedRev is non-empty the on-disk revision is
// verified inside the lock and a mismatch (including a vanished file) returns
// ErrStaleObject without writing.
func (s *DomainFileStore) save(kind domainKind, payload any, expectedRev string) error {
	if catalog, ok := payload.(*WorkspaceCatalog); ok {
		if _, err := s.bindCatalogPayload(catalog); err != nil {
			return err
		}
	} else if _, err := s.validatedWorkspaceID(); err != nil {
		return err
	}

	path, err := s.pathFor(kind)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	lockPath := path + ".lock"
	return util.WithFileLock(lockPath, true, func() error {
		// Workspace identity is immutable even for last-writer-wins saves. Check
		// the current envelope while holding the same lock as the write so a
		// concurrent catalog create cannot be replaced with another identity.
		current, currentErr := os.ReadFile(path)
		if currentErr == nil {
			env, decErr := decodeEnvelope(current)
			if decErr != nil {
				return fmt.Errorf("failed to validate workspace identity in %s: %w", filepath.Base(path), decErr)
			}
			if bindErr := s.BindWorkspaceID(env.WorkspaceID); bindErr != nil {
				return fmt.Errorf("invalid workspace identity in %s: %w", filepath.Base(path), bindErr)
			}
		} else if !errors.Is(currentErr, os.ErrNotExist) {
			return fmt.Errorf("failed to read %s for workspace identity check: %w", filepath.Base(path), currentErr)
		}

		if expectedRev != "" {
			switch {
			case errors.Is(currentErr, os.ErrNotExist):
				return ErrStaleObject
			case currentErr != nil:
				return fmt.Errorf("failed to re-read %s for concurrency check: %w", filepath.Base(path), currentErr)
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
	// Color slots are cosmetic and explicitly self-healing. Validate every
	// structural field while allowing EnsureProjectColorSlots at startup or the
	// next catalog mutation to repair and persist an out-of-range slot.
	if err := out.validateForLoad(); err != nil {
		return nil, rev, fmt.Errorf("invalid workspace catalog: %w", err)
	}
	return out, rev, nil
}

// bindWorkspaceIDFromCatalog resolves the synced catalog before any machine
// path is requested. A machine-only artifact cannot establish workspace
// identity because doing so would let a stale or foreign local file select its
// own namespace.
func (s *DomainFileStore) bindWorkspaceIDFromCatalog() error {
	if s.workspaceID != "" {
		if err := validateWorkspaceID(s.workspaceID); err != nil {
			return err
		}
	}
	_, rev, err := s.LoadCatalogChecked()
	if err != nil {
		return err
	}
	if rev == "" {
		return fmt.Errorf("%w: workspace catalog must exist before machine-state access", ErrWorkspaceIDRequired)
	}
	_, err = s.validatedWorkspaceID()
	return err
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
	if err := catalog.Validate(); err != nil {
		return fmt.Errorf("invalid workspace catalog: %w", err)
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
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return err
	}
	return s.save(domainPlanning, planning, rev)
}

// LoadMachine implements MachineStore.
func (s *DomainFileStore) LoadMachine() (*MachineState, error) {
	m, _, err := s.LoadMachineChecked()
	return m, err
}

// LoadMachineChecked loads machine state and its revision.
func (s *DomainFileStore) LoadMachineChecked() (*MachineState, string, error) {
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return nil, "", err
	}
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
	if err := s.bindWorkspaceIDFromCatalog(); err != nil {
		return err
	}
	return s.save(domainMachine, machine, rev)
}

// Compile-time checks that the file store satisfies every domain store
// interface. It is the canonical implementation, persisting each domain to its
// own synced file.
var (
	_ WorkspaceCatalogStore = (*DomainFileStore)(nil)
	_ PlanningStore         = (*DomainFileStore)(nil)
	_ MachineStore          = (*DomainFileStore)(nil)
)
