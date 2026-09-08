package storage

import (
	"os"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
)

// ArtifactChange describes a single full-file artifact written or deleted in the
// same transaction as project or lifecycle effects. Path is the absolute target;
// a write carries Data (and an optional Mode, defaulting to the standard private
// file permission), while Delete removes the target instead.
type ArtifactChange struct {
	Path   string
	Data   []byte
	Mode   os.FileMode
	Delete bool
	// BackupPath, when non-empty, receives the current Path bytes as a private
	// 0600 write immediately before the primary write or delete in the same
	// journal manifest. An absent primary produces no backup operation, matching
	// first-save behavior. Backup writes are transaction-internal maintenance and
	// are deliberately excluded from user-visible undo ledger images.
	BackupPath string
	// ExpectedRevision applies optimistic concurrency to this artifact, validated
	// under the commit's deterministic artifact locks before any bytes are staged.
	// Its three states are distinct on purpose:
	//   - nil: unchecked. The write or delete is last-writer-wins.
	//   - non-nil pointing to "": the artifact must be ABSENT. A create that must
	//     not clobber an existing file; an existing target aborts the commit.
	//   - non-nil pointing to a non-empty value: the current raw file bytes must
	//     hash (hex SHA-256, matching RawArtifactRevision) to exactly that value.
	// Any divergence aborts the whole commit with an *ArtifactConflictError before
	// a single byte is written, so a concurrent config writer's change is never
	// silently overwritten.
	ExpectedRevision *string
	// ExpectedRevisionPath optionally names the source whose raw bytes must match
	// ExpectedRevision when it differs from Path. Project rename uses this to
	// validate the old slug's tasks.md under its canonical project lock while the
	// transaction writes the replacement below the new slug after moving the
	// directory. Empty means validate Path itself.
	ExpectedRevisionPath string
}

// ArtifactPrecondition is a read-only optimistic-concurrency guard for an
// artifact that is not itself part of a transaction's writes or deletes. Path
// is locked with the transaction's ArtifactChanges and its raw bytes must match
// ExpectedRevision before ledger images are captured or journal operations are
// staged. An empty ExpectedRevision requires Path to be absent.
//
// Preconditions never become journal operations or workspace-ledger images;
// they only close races between an earlier application-level validation and
// the commit boundary.
type ArtifactPrecondition struct {
	Path             string
	ExpectedRevision string
}

// ProjectRevisionPrecondition is a read-only optimistic-concurrency guard for
// a project that an application transaction consulted but may not rewrite.
// Path is the project directory containing tasks.md. The current canonical
// project revision must equal ExpectedRevision while the project's advisory
// lock is held; any divergence aborts the whole transaction with a
// RevisionConflictError before normal stale-revision merging, conflict
// recording, undo capture, or journal staging begins.
//
// Preconditions never become project writes, journal operations, conflict
// bundles, or workspace-ledger images. ProjectID and ProjectName are retained
// only to identify a failed guard precisely to callers.
type ProjectRevisionPrecondition struct {
	ProjectID        string
	ProjectName      string
	Path             string
	ExpectedRevision string
}

// ArtifactCommitRequest describes an artifact-only user mutation. Artifacts are
// committed through the same workspace lock, recovery, optimistic-revision, and
// journal path as configuration-only artifact commits, while ProjectIDs and the
// undo description let a configured workspace ledger expose the transaction to
// global and current-project undo selection without requiring a tasks.md rewrite.
//
// Preconditions are read-only guards and never become journal or ledger
// artifacts. ProjectIDs may be empty for a workspace-global mutation. A
// transaction that affects one or more projects lists their durable IDs; the
// ledger de-duplicates them before persistence. UndoAction and UndoContext are
// presentation metadata only and do not affect transaction semantics.
type ArtifactCommitRequest struct {
	Artifacts     []ArtifactChange
	Preconditions []ArtifactPrecondition
	ProjectIDs    []string
	UndoAction    string
	UndoContext   string
}

// OpsCommitRequest describes an undoable user mutation that combines raw
// filesystem choreography with revision-checked artifacts. Project lifecycle
// operations use it when a directory move and its catalog/tasks rewrites must
// be one journal transaction and one workspace-ledger entry.
type OpsCommitRequest struct {
	Ops         []transaction.Op
	Artifacts   []ArtifactChange
	ProjectIDs  []string
	UndoAction  string
	UndoContext string
}

// ChangeSet is the unit of a single project's mutation committed atomically: the
// project whose tasks.md is rewritten, the exact document and revision the caller
// loaded (for lossless preservation and optimistic concurrency), and any
// additional artifacts to write or delete alongside it in one transaction. A
// leaf commit with no artifacts still commits the project write through the
// journal.
//
// Artifacts are arbitrary files: configuration stores, notes, blocker records,
// archive files, and derived indexes all flow through the same field. CommitMulti
// commits several ChangeSets — spanning multiple projects and their artifacts —
// through one journal transaction, so a whole cross-project mutation is
// all-or-nothing.
type ChangeSet struct {
	// Project is the single project whose tasks.md this changeset rewrites.
	Project *models.Project
	// ExpectedRevision is the canonical revision the caller observed when it
	// loaded Project. Commit re-reads the current on-disk revision under the lock
	// and aborts with a RevisionConflictError if it no longer matches. An empty
	// value opts out of the check (e.g. legacy callers that do not track a load
	// revision, or a brand-new project with no prior content).
	ExpectedRevision string
	// Base is the exact lossless document retained when Project was loaded. Normal
	// mutations overlay Project's structured fields onto Base while preserving its
	// schema-v2 envelope and opaque header blocks. It is also the future three-way
	// merge base when ExpectedRevision is stale. Nil is accepted only for legacy,
	// repair, and initial-create callers that have no loaded document.
	Base *ProjectDocument
	// ProjectPreconditions guard every additional project state this mutation
	// consulted without turning those projects into writes. They participate in
	// the same early path validation and deterministic project-lock set as Project,
	// then validate before the ordinary expected-revision merge path begins.
	ProjectPreconditions []ProjectRevisionPrecondition
	// ArtifactPreconditions guard every non-project artifact this mutation read
	// but does not rewrite — the workspace catalog a whole-workspace census was
	// derived from, for instance. They are containment-checked and locked with the
	// changeset's own artifacts and validate in the same first phase as
	// ProjectPreconditions, so a concurrent catalog write expires the mutation
	// before any merge, conflict bundle, ledger image, or staged byte.
	ArtifactPreconditions []ArtifactPrecondition
	// Artifacts are additional files committed atomically with the project write.
	Artifacts []ArtifactChange
	// AttachmentOps are attachment file side effects (copy, move, delete) resolved
	// to concrete transaction operations and committed in the same transaction as
	// the project write, so a task's attachment bytes and the metadata that
	// references them are staged, committed, and recovered together.
	AttachmentOps []AttachmentOp
	// UndoAction and UndoContext describe this mutation for the workspace undo
	// ledger. When the commit service carries a ledger, the committed transaction
	// is recorded under this description so a later undo can name what it reverses.
	// They are ignored when no ledger is configured.
	UndoAction  string
	UndoContext string
}

// AttachmentOpKind enumerates the attachment file side effects a ChangeSet can
// carry so they are committed and crash-recovered in the same transaction as the
// task metadata that references them.
type AttachmentOpKind int

const (
	// AttachmentCopy ingests an external file into a task's attachments directory.
	AttachmentCopy AttachmentOpKind = iota
	// AttachmentMove relocates a task's whole attachments directory from one
	// project to another. The task UUID — and therefore every referenced filename —
	// is preserved across a cross-project move, so the destination project takes
	// ownership of the same on-disk files.
	AttachmentMove
	// AttachmentDelete removes a task's attachments directory and every file in it.
	AttachmentDelete
)

// AttachmentOp describes one attachment file side effect committed atomically with
// a ChangeSet's project write. It is resolved to concrete transaction operations
// under the commit locks: a copy stages the source bytes for the destination path,
// a move renames the task's whole attachments directory, and a delete removes every
// file and the directory itself. Any resolution failure — a missing copy source, an
// occupied move destination, an unexpected nested directory — aborts the whole
// commit before any file is written, and crash recovery restores the attachment
// bytes together with the task metadata because both flow through one manifest.
type AttachmentOp struct {
	// Kind selects the side effect.
	Kind AttachmentOpKind
	// Task owns the attachments; its stable UUID addresses the on-disk directory.
	Task *models.Task
	// SourceProject holds the attachments before a move and owns them for a delete.
	SourceProject *models.Project
	// DestProject receives the attachments for a copy or a move.
	DestProject *models.Project
	// SourcePath is the external file ingested by a copy.
	SourcePath string
	// Filename is the destination name for a copy; it defaults to the base name of
	// SourcePath when empty.
	Filename string
}

// CommitResult reports the outcome of a successful commit: the new canonical
// revision of the committed project and the id of the transaction that applied
// it.
type CommitResult struct {
	Revision      string
	TransactionID string
}
