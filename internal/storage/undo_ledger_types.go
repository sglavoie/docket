package storage

import (
	"errors"
	"time"

	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/transaction"
)

// Empty history is an ordinary outcome, distinct from an unreadable ledger.
var (
	ErrNoUndoHistory = errors.New("no undo history available")
	ErrNoRedoHistory = errors.New("no redo history available")
)

// ErrUndoSuperseded refuses an undo naming a transaction that is no longer the
// newest undo entry.
var ErrUndoSuperseded = errors.New("the named transaction is no longer the newest undo entry")

// UndoLedgerSchemaVersion is stamped into every persisted ledger file. A file
// carrying a newer version than the running binary understands is rejected
// rather than misinterpreted, matching the transaction manifest contract.
const UndoLedgerSchemaVersion = 2

const undoLedgerSchemaVersionV1 = 1

const (
	undoLedgerFileName = "ledger.json"
	undoLedgerBlobsDir = "blobs"
	WorkspaceUndoLimit = 50
)

// ArtifactImageKind distinguishes retained file contents from directory
// existence. Schema-v1 images have no kind and are interpreted as files.
type ArtifactImageKind string

const (
	ArtifactImageKindFile      ArtifactImageKind = "file"
	ArtifactImageKindDirectory ArtifactImageKind = "directory"
)

// ArtifactImage is the before/after image of one filesystem artifact touched by
// a committed transaction. File images retain byte hashes; directory images
// retain only type and existence. Undo restores the "before" state and redo
// restores the "after" state.
//
// This uniform model reverses every kind of committed side effect: a project
// tasks.md rewrite is one artifact (before-bytes → after-bytes); an artifact
// delete is one artifact (before-bytes → absent); an attachment copy is one
// artifact (absent → after-bytes); an attachment delete is one artifact per
// file (before-bytes → absent); and an attachment move across projects is two
// artifacts per file (the source path loses its bytes, the destination path
// gains them), so undo and redo relocate the bytes by writing each side to its
// recorded state.
type ArtifactImage struct {
	// Kind is file or directory. It was absent from schema-v1 file images.
	Kind ArtifactImageKind `json:"kind"`
	// Path is the absolute filesystem path the image restores.
	Path string `json:"path"`
	// BeforeExists reports whether the artifact existed before the transaction.
	BeforeExists bool `json:"before_exists"`
	// BeforeHash references the retained blob holding the pre-transaction bytes.
	// Empty when BeforeExists is false.
	BeforeHash string `json:"before_hash,omitempty"`
	// AfterExists reports whether the artifact existed after the transaction.
	AfterExists bool `json:"after_exists"`
	// AfterHash references the retained blob holding the post-transaction bytes.
	// Empty when AfterExists is false.
	AfterHash string `json:"after_hash,omitempty"`
}

// LedgerEntry is one append-only record of a committed transaction: the manifest
// id it corresponds to, the projects it affected (for current-project
// selection), and the before/after artifact images needed to reverse (undo) or
// replay (redo) it.
type LedgerEntry struct {
	// Seq is the monotonically increasing sequence assigned at record time. It
	// orders the undo and redo stacks even after entries move between them, so
	// "newest" selection is a max-Seq scan rather than slice position.
	Seq int `json:"seq"`
	// TransactionID is the committed transaction manifest id this entry reverses.
	TransactionID string `json:"transaction_id"`
	// Action and Context describe the mutation for user-facing undo messages.
	Action  string `json:"action,omitempty"`
	Context string `json:"context,omitempty"`
	// Timestamp records when the entry was appended.
	Timestamp time.Time `json:"timestamp"`
	// ProjectIDs lists the logical ids of every project the transaction touched,
	// so current-project undo can select the newest entry affecting a project.
	ProjectIDs []string `json:"project_ids,omitempty"`
	// Artifacts carries every affected file or directory's before/after image.
	Artifacts []ArtifactImage `json:"artifacts,omitempty"`
	// LegacySchemaVersion preserves the provenance of entries upgraded from v1.
	// Their file-only images need conservative attachment-directory cleanup on
	// every undo/redo, including after the containing ledger is rewritten as v2.
	LegacySchemaVersion int `json:"legacy_schema_version,omitempty"`
	// Device is the sync device that committed the transaction, stamped when
	// the ledger records beside a sync history. Undo is device-local: a device
	// restores only its own entries, so a ledger carried to another device in a
	// copied workspace can never replay one device's history on another. An
	// entry without one predates sync and stays restorable, revision checks
	// included.
	Device string `json:"device,omitempty"`
}

func (a ArtifactImage) imageKind() ArtifactImageKind {
	if a.Kind == "" {
		return ArtifactImageKindFile
	}
	return a.Kind
}

// ownedBy reports whether device may restore the entry: any entry when the
// ledger records beside no sync history (device is empty), otherwise its own
// entries and those recorded before sync stamped a device.
func (e LedgerEntry) ownedBy(device string) bool {
	return device == "" || e.Device == "" || e.Device == device
}

// device is the sync identity entries are stamped with and restored by, or
// empty without a sync history.
func (l *WorkspaceLedger) device() string {
	if l.history == nil {
		return ""
	}
	return l.history.DeviceID()
}

func (e LedgerEntry) touchesProject(projectID string) bool {
	for _, id := range e.ProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

// blobHashes returns every non-empty blob hash referenced by this entry, used by
// blob garbage collection to compute the live set.
func (e LedgerEntry) blobHashes() []string {
	out := make([]string, 0, len(e.Artifacts)*2)
	for _, a := range e.Artifacts {
		if a.BeforeExists && a.BeforeHash != "" {
			out = append(out, a.BeforeHash)
		}
		if a.AfterExists && a.AfterHash != "" {
			out = append(out, a.AfterHash)
		}
	}
	return out
}

// ledgerFile is the persisted, schema-versioned workspace ledger: the undo stack
// (Entries, most recent has the highest Seq) and the redo stack (RedoEntries),
// plus the monotonic sequence counter.
type ledgerFile struct {
	SchemaVersion int           `json:"schema_version"`
	NextSeq       int           `json:"next_seq"`
	Entries       []LedgerEntry `json:"entries"`
	RedoEntries   []LedgerEntry `json:"redo_entries"`
}

// LedgerArtifact is a caller-supplied before/after image. File images carry
// bytes; directory images carry only existence. The commit boundary captures
// these under the project locks and hands them to Record for persistence.
type LedgerArtifact struct {
	Kind         ArtifactImageKind
	Path         string
	BeforeExists bool
	Before       []byte
	AfterExists  bool
	After        []byte
}

// LedgerRecord is the request to append one committed transaction to the ledger.
type LedgerRecord struct {
	TransactionID string
	Action        string
	Context       string
	ProjectIDs    []string
	Artifacts     []LedgerArtifact
}

// preparedLedgerRecord is a side-effect-free ledger append plan. Ops write any
// missing content-addressed blobs first and the new ledger file last, so a
// CommitService can append them to the same transaction manifest as the user
// artifacts. file is the resulting in-memory ledger state used for post-commit
// best-effort blob garbage collection.
type preparedLedgerRecord struct {
	ops  []transaction.Op
	file *ledgerFile
}

// WorkspaceLedger is the single workspace-wide undo/redo history. It is an
// append-only ledger of committed transactions: each successful commit appends
// one entry referencing the transaction manifest and retaining the before/after
// image of every file the transaction touched. Undo and redo move entries
// between the undo and redo stacks while restoring the recorded images to disk.
//
// The commit boundary wires an optional ledger so callers cannot forget history.
// Project ChangeSets and ledger-aware artifact-only transactions use
// prepareRecord to put the blobs and ledger JSON in the same durable manifest as
// the user artifacts. A nil ledger disables history entirely, preserving behavior
// for callers and tests that do not opt in.
type WorkspaceLedger struct {
	dir             string
	transactionsDir string
	roots           []string
	now             func() time.Time
	// machineID owns the transactions the ledger's own standalone record and
	// restore paths prepare. It is bound by CommitService.OnMachine alongside
	// transactionsDir, so an undo runs as the same machine as the commit it
	// reverses. Empty means the running process's machine.
	machineID string
	// history, when non-nil, records each undo and redo as a new outgoing sync
	// transaction staged in the restore's own journal manifest. The commit
	// service that attaches the history propagates it here.
	history *synclog.History
	// ownWrites, when non-nil, records what each undo and redo wrote. The
	// commit service that attaches the registry propagates it here.
	ownWrites *OwnWrites
}

// WorkspaceLedgerValidation summarizes a successful read-only validation.
type WorkspaceLedgerValidation struct {
	UndoEntries int
	RedoEntries int
}
