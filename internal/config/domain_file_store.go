package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// This file owns the store's identity and addressing: the workspace ID it is
// bound to and the path each domain artifact resolves to. The envelope those
// artifacts carry lives in domain_file_store_envelope.go, the locked atomic read
// and write paths in domain_file_store_io.go, and the per-domain load and save
// entry points in domain_file_store_domains.go.

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

// ErrNoDataDir reports a synced-domain write attempted by a store rooted at no
// data directory. An empty root would otherwise resolve every synced artifact
// beneath the process working directory, so the write is refused by name; reads
// stay permitted and simply observe whatever that relative path holds.
var ErrNoDataDir = errors.New("config: no workspace data directory is set")

// domainKind identifies one of the runtime configuration domains.
type domainKind int

const (
	domainCatalog domainKind = iota
	domainPlanning
	domainMachine
	domainLenses
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
	case domainLenses:
		return "lenses"
	default:
		return "unknown"
	}
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
//   - <data>/.docket/workspace.json   (synced)  — WorkspaceCatalog
//   - <data>/.docket/planning.json    (synced)  — PlanningState
//   - ~/.docket/runtime/<workspace-id>/machine.json (machine-local) — MachineState
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
	// machineHome names the machine identity this store resolves machine.json
	// beneath. The empty string is the default and means this process's own
	// home, so nothing but an explicitly named machine leaves $HOME. It is the
	// only field a second machine over the same data directory differs in.
	machineHome string
	// unrooted records that the store was built over an empty data directory,
	// which filepath.Clean has already turned into ".". It is what refuses a
	// synced write rather than landing one in the working directory.
	unrooted bool
}

// NewDomainFileStore roots a store at a data directory with no known workspace
// ID; the ID is adopted from the first artifact read, or minted via
// EnsureWorkspaceID. Machine-local state resolves beneath this process's home.
func NewDomainFileStore(dataDir string) *DomainFileStore {
	return NewDomainFileStoreOnMachine(dataDir, "")
}

// NewDomainFileStoreOnMachine roots a store at a data directory and resolves its
// machine-local artifact beneath a named machine home. Two stores over one data
// directory with distinct homes are two machines sharing one workspace, and
// because neither reads $HOME they may be driven concurrently.
func NewDomainFileStoreOnMachine(dataDir, machineHome string) *DomainFileStore {
	return &DomainFileStore{
		dataDir:     filepath.Clean(dataDir),
		machineHome: machineHome,
		unrooted:    strings.TrimSpace(dataDir) == "",
	}
}

// requireDataDir refuses a synced write from a store rooted at no data
// directory. Machine-local state resolves beneath a home rather than the data
// directory, so it is the one domain this does not gate.
func (s *DomainFileStore) requireDataDir() error {
	if s.unrooted {
		return ErrNoDataDir
	}
	return nil
}

// DataDir returns the store's workspace data directory.
func (s *DomainFileStore) DataDir() string { return s.dataDir }

// MachineHome returns the machine identity this store resolves machine.json
// beneath, or the empty string when it follows this process's home.
func (s *DomainFileStore) MachineHome() string { return s.machineHome }

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
	case domainLenses:
		return LensDefinitionsPath(s.dataDir), nil
	case domainMachine:
		workspaceID, err := s.validatedWorkspaceID()
		if err != nil {
			return "", err
		}
		return machineStatePathOn(s.machineHome, workspaceID)
	default:
		return filepath.Join(InternalDir(s.dataDir), kind.name()+".json"), nil
	}
}

// Compile-time checks that the file store satisfies every domain store
// interface. It is the canonical implementation, persisting each domain to its
// own synced file.
var (
	_ WorkspaceCatalogStore = (*DomainFileStore)(nil)
	_ PlanningStore         = (*DomainFileStore)(nil)
	_ MachineStore          = (*DomainFileStore)(nil)
)
