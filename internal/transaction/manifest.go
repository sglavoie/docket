// Package transaction implements the durable transaction journal that backs
// tt's crash-safe workspace mutations. A transaction stages the replacement
// bytes for every file it intends to write (beside the target so the eventual
// commit is an atomic same-directory rename), fsyncs that staged data, then
// records a versioned manifest in the durable workspace transactions directory.
// The manifest is the recovery record: it names every file operation and the
// staged artifact backing each write, so any machine that opens the workspace
// can inspect in-flight transactions and roll them forward or clean them up.
//
// This package deliberately operates on raw file paths and bytes only. It has no
// knowledge of Markdown, projects, or config schemas, so it can serve as the
// shared commit primitive for the higher-level ChangeSet/WorkspaceRepository
// layers built on top of it. Preparation is the boundary this session covers;
// crash recovery and commit application build on the prepared manifest.
package transaction

import (
	"errors"
	"fmt"
	"os"
)

// SchemaVersion is the on-disk version stamped into every transaction manifest.
// A manifest with a newer version than the running binary understands is
// rejected rather than misinterpreted.
const SchemaVersion = 1

// Manifest validation and journal sentinel errors. Callers classify a failure
// with errors.Is rather than matching message text.
var (
	// ErrInvalidManifest is returned for a structurally invalid manifest
	// (missing id, unknown state, no operations, or a malformed operation).
	ErrInvalidManifest = errors.New("transaction: invalid manifest")
	// ErrUnsupportedSchema is returned for a manifest whose schema version is
	// newer than this binary supports.
	ErrUnsupportedSchema = errors.New("transaction: unsupported manifest schema version")
	// ErrUnknownOpKind is returned for a file operation with an unrecognized kind.
	ErrUnknownOpKind = errors.New("transaction: unknown file operation kind")
	// ErrInvalidOp is returned for a requested operation that is missing a
	// required field (an empty target, or a move without a source).
	ErrInvalidOp = errors.New("transaction: invalid file operation")
	// ErrTargetOutsideRoot is returned when an operation's target or source
	// resolves outside every approved workspace root.
	ErrTargetOutsideRoot = errors.New("transaction: path escapes approved workspace roots")
	// ErrEmptyTransaction is returned when Prepare is called with no operations.
	ErrEmptyTransaction = errors.New("transaction: cannot prepare an empty transaction")
)

// OpKind enumerates the file operations a transaction can describe.
type OpKind string

const (
	// OpWrite creates or replaces a target file with staged replacement bytes.
	OpWrite OpKind = "write"
	// OpMove renames an existing source path onto a target path.
	OpMove OpKind = "move"
	// OpDelete removes a target path.
	OpDelete OpKind = "delete"
)

// State is the lifecycle state of a transaction manifest. Preparation writes a
// manifest in StatePrepared; the commit and recovery layers advance it through
// StateCommitting to StateCommitted (or clean it up) in later sessions.
type State string

const (
	// StatePreparing marks a manifest whose staging is not yet durable. It is
	// reserved for future two-phase preparation; Prepare currently writes a
	// manifest only once staging is fsynced, so it lands directly in
	// StatePrepared.
	StatePreparing State = "preparing"
	// StatePrepared marks a fully staged transaction whose replacement bytes are
	// durably on disk. It is safe to roll forward or discard.
	StatePrepared State = "prepared"
	// StateCommitting marks a transaction whose file operations are being applied.
	StateCommitting State = "committing"
	// StateCommitted marks a transaction whose file operations have all been
	// applied; its staged artifacts may be reclaimed.
	StateCommitted State = "committed"
)

// FileOp records a single file operation within a manifest. Fields are populated
// per kind: a write carries Staged (the absolute path of the fsynced staging
// file), Hash (the hex SHA-256 of the staged replacement bytes), and Mode; a
// move carries Source; a delete uses Target alone. Hash lets crash recovery roll
// a write forward deterministically — it verifies the staged bytes are intact
// before renaming them into place rather than trusting file timestamps.
type FileOp struct {
	Kind   OpKind      `json:"kind"`
	Target string      `json:"target"`
	Source string      `json:"source,omitempty"`
	Staged string      `json:"staged,omitempty"`
	Hash   string      `json:"hash,omitempty"`
	Mode   os.FileMode `json:"mode,omitempty"`
}

func (op FileOp) validate() error {
	if op.Target == "" {
		return fmt.Errorf("missing target: %w", ErrInvalidManifest)
	}
	switch op.Kind {
	case OpWrite:
		if op.Staged == "" {
			return fmt.Errorf("write op for %q missing staged path: %w", op.Target, ErrInvalidManifest)
		}
	case OpMove:
		if op.Source == "" {
			return fmt.Errorf("move op for %q missing source: %w", op.Target, ErrInvalidManifest)
		}
	case OpDelete:
		// Delete needs only a target.
	default:
		return fmt.Errorf("%q: %w", op.Kind, ErrUnknownOpKind)
	}
	return nil
}

// Manifest is the durable, versioned record of a transaction's intended file
// operations and current lifecycle state.
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	ID            string   `json:"id"`
	State         State    `json:"state"`
	CreatedAt     string   `json:"created_at"`
	Ops           []FileOp `json:"ops"`
}

// Validate reports whether a manifest is structurally sound and its schema
// version is understood by this binary. It is applied on load so a corrupt or
// forward-version manifest is rejected rather than acted on.
func (m *Manifest) Validate() error {
	if m.SchemaVersion == 0 {
		return fmt.Errorf("missing schema version: %w", ErrInvalidManifest)
	}
	if m.SchemaVersion > SchemaVersion {
		return fmt.Errorf("manifest schema version %d newer than supported %d: %w", m.SchemaVersion, SchemaVersion, ErrUnsupportedSchema)
	}
	if m.ID == "" {
		return fmt.Errorf("missing id: %w", ErrInvalidManifest)
	}
	switch m.State {
	case StatePreparing, StatePrepared, StateCommitting, StateCommitted:
	default:
		return fmt.Errorf("unknown state %q: %w", m.State, ErrInvalidManifest)
	}
	if len(m.Ops) == 0 {
		return fmt.Errorf("manifest has no operations: %w", ErrInvalidManifest)
	}
	for i, op := range m.Ops {
		if err := op.validate(); err != nil {
			return fmt.Errorf("op %d: %w", i, err)
		}
	}
	return nil
}
