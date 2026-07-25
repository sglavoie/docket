package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/util"
)

// Typed conflict and recovery errors surfaced by the commit boundary. Callers
// classify a failure with errors.Is rather than matching message text.
var (
	// ErrStaleRevision indicates the on-disk project changed since the caller
	// loaded it: the expected revision no longer matches what is on disk, so the
	// changeset is aborted rather than clobbering the concurrent change. The
	// caller should reload and retry (or, later, three-way merge).
	ErrStaleRevision = errors.New("storage: project revision is stale; reload and retry")
	// ErrCommitBlocked indicates commit is refused because startup crash recovery
	// quarantined one or more irrecoverable transactions. The workspace stays
	// blocked until a human resolves the quarantine.
	ErrCommitBlocked = errors.New("storage: commit blocked by quarantined transactions")
	// ErrStaleArtifact indicates a configuration artifact changed on disk since the
	// caller loaded it: its ExpectedRevision no longer matches the current raw hash
	// (or an expected-absent artifact now exists), so the commit is aborted rather
	// than clobbering the concurrent write. It is the artifact-level counterpart to
	// ErrStaleRevision and the caller should reload the config domain and retry.
	ErrStaleArtifact = errors.New("storage: artifact revision is stale; reload and retry")
	// ErrMergeConflict indicates a stale-revision commit was three-way merged against
	// the current on-disk state and produced conflicts that cannot be resolved
	// automatically. The conflict bundle(s) have been durably recorded and marked
	// as requiring a fresh operation retry because they intentionally do not retain
	// the aborted transaction's companion projects, artifacts, attachment bytes, or
	// ledger intent. The whole commit was aborted with no project or artifact write.
	// Callers match this class with errors.Is and read the recorded ids from a
	// *CommitConflictError.
	ErrMergeConflict = errors.New("storage: commit produced unresolved merge conflicts recorded for resolution")
)

// RecordedConflict names one project whose stale-revision merge could not be
// resolved automatically and the durable conflict bundle recorded for it.
type RecordedConflict struct {
	ProjectID   string
	ProjectName string
	// ConflictID is the id of the durably recorded conflict bundle; resolve it
	// through the shared resolution service (tt conflict resolve / the TUI).
	ConflictID string
	// Conflicts carries the structured conflicts the merge produced, for callers
	// that want to summarize what diverged without reloading the bundle.
	Conflicts []MergeConflict
}

// CommitConflictError is the concrete unresolved-merge conflict returned when one
// or more stale-revision changesets three-way merged into true conflicts. Every
// listed conflict has already been durably recorded (so a later resolution can
// reconcile it) and the whole commit was aborted with no writes. It unwraps to
// ErrMergeConflict so callers can match the class while inspecting the recorded
// ids and structured conflicts.
type CommitConflictError struct {
	Conflicts []RecordedConflict
}

func (e *CommitConflictError) Error() string {
	if len(e.Conflicts) == 1 {
		c := e.Conflicts[0]
		name := c.ProjectName
		if name == "" {
			name = c.ProjectID
		}
		return fmt.Sprintf("storage: project %s has unresolved merge conflicts (recorded as %s); adopt remote, then retry the original operation", name, c.ConflictID)
	}
	ids := make([]string, 0, len(e.Conflicts))
	for _, c := range e.Conflicts {
		ids = append(ids, c.ConflictID)
	}
	return fmt.Sprintf("storage: %d projects have unresolved merge conflicts (recorded as %s); adopt remote, then retry the original operation", len(e.Conflicts), strings.Join(ids, ", "))
}

func (e *CommitConflictError) Unwrap() error { return ErrMergeConflict }

// ArtifactConflictError is the concrete stale-artifact conflict returned when an
// ArtifactChange's ExpectedRevision does not match the current on-disk raw hash
// (or an expected-absent artifact already exists). It unwraps to ErrStaleArtifact
// so callers can match the class while inspecting the specific path and the
// diverging revisions.
type ArtifactConflictError struct {
	Path     string
	Expected string
	Actual   string
}

func (e *ArtifactConflictError) Error() string {
	return fmt.Sprintf("storage: artifact %s changed on disk (expected revision %s, found %s): reload and retry", filepath.Base(e.Path), shortRevision(e.Expected), shortRevision(e.Actual))
}

func (e *ArtifactConflictError) Unwrap() error { return ErrStaleArtifact }

// RevisionConflictError is the concrete stale-revision conflict returned when a
// changeset's expected revision does not match the current on-disk revision. It
// unwraps to ErrStaleRevision so callers can match the class while still
// inspecting the specific project and the diverging revisions.
type RevisionConflictError struct {
	ProjectID   string
	ProjectName string
	Expected    string
	Actual      string
}

func (e *RevisionConflictError) Error() string {
	name := e.ProjectName
	if name == "" {
		name = e.ProjectID
	}
	if name == "" {
		name = "<unknown>"
	}
	return fmt.Sprintf("storage: project %s changed on disk (expected revision %s, found %s): reload and retry", name, shortRevision(e.Expected), shortRevision(e.Actual))
}

func (e *RevisionConflictError) Unwrap() error { return ErrStaleRevision }

// shortRevision trims a hex revision to a readable prefix for error messages,
// leaving short or empty revisions untouched.
func shortRevision(rev string) string {
	if rev == "" {
		return "<none>"
	}
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

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
// RevisionConflictError before normal stale-revision merging, activity diffs,
// conflict recording, undo capture, or journal staging begins.
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

// CommitService is the mutation commit boundary. It drives one or more
// changesets through the durable transaction journal: recover any interrupted
// transaction first, acquire every affected project's lock in deterministic
// order, verify each optimistic revision under the locks, then prepare and commit
// every project write plus its artifacts as one atomic transaction. Commit is the
// single-project convenience; CommitMulti spans multiple projects and their
// artifacts in one transaction. Direct raw-save paths
// (SaveProject/RestoreFromBackup) remain only as internal repair hooks.
type CommitService struct {
	transactionsDir string
	roots           []string
	// ledger, when non-nil, records undoable commits into the workspace ledger so
	// callers cannot forget to capture history. Project ChangeSets and ledger-aware
	// artifact-only transactions stage their retained blobs and ledger entry inside
	// the same manifest as the user artifacts. A nil ledger disables history.
	ledger *WorkspaceLedger
	// conflicts, when non-nil, turns a stale-revision commit into a three-way
	// merge: the loaded base, the caller's local intent, and the current on-disk
	// document are merged under the held locks. A clean non-overlapping merge is
	// committed against the current revision; a true conflict is recorded in this
	// store and aborts the whole commit. A nil store disables merging, so a stale
	// revision aborts with a RevisionConflictError exactly as before.
	conflicts *ConflictStore
}

// NewCommitService returns a CommitService that journals transactions under
// transactionsDir and only permits operations whose targets resolve inside one
// of roots. roots is typically the single workspace data directory. The returned
// service records no undo history; use NewCommitServiceWithLedger to attach the
// workspace undo ledger.
func NewCommitService(transactionsDir string, roots []string) *CommitService {
	return &CommitService{transactionsDir: transactionsDir, roots: roots}
}

// NewCommitServiceWithLedger returns a CommitService that, in addition to
// journaling every mutation, records undoable commits into ledger as one
// append-only workspace entry. Project ChangeSets and CommitArtifactTransaction
// put their retained blobs and ledger JSON in the user-artifact manifest itself
// so recovery cannot land one without the other. Both paths capture the exact
// before/after image of every user file the transaction touched.
func NewCommitServiceWithLedger(transactionsDir string, roots []string, ledger *WorkspaceLedger) *CommitService {
	if ledger != nil {
		// Restore must acquire the exact workspace lock used by the service that
		// records entries. The standard layout already makes these sibling paths;
		// binding explicitly also keeps custom/test layouts on the same lock.
		ledger.transactionsDir = filepath.Clean(transactionsDir)
	}
	return &CommitService{transactionsDir: transactionsDir, roots: roots, ledger: ledger}
}

// WithConflictStore attaches the durable conflict store that the commit boundary
// uses to three-way merge stale-revision commits and record irreconcilable
// conflicts. It returns the same service for fluent construction. Production
// mutation services (CLI and TUI) attach a store so a stale project revision is
// merged automatically or recorded as a conflict instead of aborting with a bare
// stale-revision error; the standalone conflict-resolution commit service is left
// storeless on purpose so its own re-commit races surface as RevisionConflictError
// rather than being re-merged against the wrong base.
func (s *CommitService) WithConflictStore(store *ConflictStore) *CommitService {
	s.conflicts = store
	return s
}

// RecoverWorkspace completes interrupted transactions under the same workspace
// commit lock used by every mutation. Frontend composition roots call this
// before loading configuration domains or projects so a crashed multi-artifact
// commit can never be exposed as a partial workspace snapshot. A quarantined
// transaction returns ErrCommitBlocked and keeps startup fail-closed until the
// quarantine is resolved.
func (s *CommitService) RecoverWorkspace() (*transaction.RecoveryReport, error) {
	var report *transaction.RecoveryReport
	err := s.withWorkspaceCommitLock(func() error {
		var recoverErr error
		report, recoverErr = s.recoverLocked(transaction.NewJournal(s.transactionsDir, s.roots))
		return recoverErr
	})
	return report, err
}

// Commit applies a single-project changeset atomically. It is a thin convenience
// wrapper over CommitMulti so the single- and multi-project paths share exactly
// one prepare/commit implementation and can never diverge.
func (s *CommitService) Commit(cs ChangeSet) (*CommitResult, error) {
	results, err := s.CommitMulti([]ChangeSet{cs})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// CommitMulti applies several changesets — potentially spanning multiple projects
// and their artifacts — as one atomic transaction. It first runs crash recovery
// on the journal and refuses to commit while any transaction is quarantined
// (ErrCommitBlocked). It then acquires every affected project's file lock in
// deterministic (sorted, de-duplicated) order to avoid deadlock between
// concurrent multi-commits, and only once all locks are held does it verify every
// expected revision. A single stale revision aborts the whole set with a
// RevisionConflictError before any bytes are written, and a target that resolves
// outside the approved roots is rejected during Prepare before anything is
// staged — so any unresolved preflight issue prevents all writes. On success it
// returns one CommitResult per changeset, in the order they were supplied, all
// sharing the transaction id that committed them together.
func (s *CommitService) CommitMulti(changesets []ChangeSet) ([]*CommitResult, error) {
	if len(changesets) == 0 {
		return nil, fmt.Errorf("storage: commit requires at least one changeset")
	}

	seenPaths := make(map[string]bool, len(changesets))
	var artifacts []ArtifactChange
	for i := range changesets {
		project := changesets[i].Project
		if project == nil {
			return nil, fmt.Errorf("storage: changeset %d requires a project", i)
		}
		if err := auditProjectBeforeSave(project); err != nil {
			return nil, err
		}
		if err := validateProjectPaths(i, project.Path, s.roots); err != nil {
			return nil, err
		}
		pathID := artifactPathIdentity(project.Path)
		if seenPaths[pathID] {
			return nil, fmt.Errorf("storage: changeset %d duplicates project path %q; a project may appear at most once per commit", i, project.Path)
		}
		seenPaths[pathID] = true
		artifacts = append(artifacts, changesets[i].Artifacts...)
		for j := range changesets[i].ProjectPreconditions {
			precondition := changesets[i].ProjectPreconditions[j]
			if err := validateProjectPreconditionPath(i, j, precondition.Path, s.roots); err != nil {
				return nil, err
			}
		}
	}
	artifactLocks, err := artifactLockPaths(artifacts, s.roots)
	if err != nil {
		return nil, fmt.Errorf("storage: changeset artifacts: %w", err)
	}

	// The whole commit pipeline — crash recovery plus the project-scoped apply —
	// runs under one workspace-level lock so concurrent committers can never run
	// recovery against each other's in-flight transactions. See
	// withWorkspaceCommitLock for why the per-project locks alone are insufficient.
	var results []*CommitResult
	if err := s.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(s.transactionsDir, s.roots)
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		// A prepared snapshot can outlive removal of a project it only read. Detect
		// that case under the workspace lock before opening an in-project lock file;
		// recreating the missing directory would turn a read-only failed guard into a
		// filesystem mutation.
		var projectPreconditionLocks []string
		for i := range changesets {
			for j := range changesets[i].ProjectPreconditions {
				precondition := changesets[i].ProjectPreconditions[j]
				if _, statErr := os.Stat(precondition.Path); os.IsNotExist(statErr) {
					if precondition.ExpectedRevision != "" {
						return &RevisionConflictError{
							ProjectID:   precondition.ProjectID,
							ProjectName: precondition.ProjectName,
							Expected:    precondition.ExpectedRevision,
							Actual:      "",
						}
					}
					continue
				} else if statErr != nil {
					return fmt.Errorf("reading project %s for precondition lock: %w", precondition.ProjectName, statErr)
				}
				projectPreconditionLocks = append(projectPreconditionLocks, filepath.Join(precondition.Path, LockFileName))
			}
		}

		lockPaths := make([]string, 0, len(changesets)+len(projectPreconditionLocks))
		for i := range changesets {
			project := changesets[i].Project
			if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
				return fmt.Errorf("failed to create project directory: %w", err)
			}
			lockPaths = append(lockPaths, filepath.Join(project.Path, LockFileName))
		}
		lockPaths = append(lockPaths, projectPreconditionLocks...)
		// Artifact and adjacent lock paths were containment-checked before the
		// workspace lock created anything. Only now may their parent directories be
		// created so the lock files can be opened safely.
		if err := createArtifactLockDirectories(artifactLocks); err != nil {
			return err
		}
		lockPaths = append(lockPaths, artifactLocks...)

		return withOrderedFileLocks(lockPaths, func() error {
			r, commitErr := s.commitMultiLocked(journal, changesets)
			if commitErr != nil {
				return commitErr
			}
			results = r
			return nil
		})
	}); err != nil {
		return nil, err
	}
	return results, nil
}

// CommitOps commits raw file operations — directory moves, tombstone writes, and
// recursive deletes — as ONE atomic transaction through the journal, under the
// workspace commit lock and after crash recovery. Call CommitOpsWithArtifacts
// when a lifecycle operation also carries revision-checked writes.
// It is the project-lifecycle counterpart to CommitMulti: where CommitMulti's unit
// is a project's tasks.md plus its artifacts, CommitOps carries arbitrary in-root
// file ops, so archive-location, trash, restore, and purge choreography commits
// atomically or not at all — a crash mid-commit is rolled forward by recovery, and
// any target that resolves outside the approved roots is rejected during Prepare
// before a single byte is staged. It records no undo-ledger entry: project
// trash/restore is reversible through the trash lifecycle itself and purge is
// irreversible, so neither participates in the task-oriented workspace ledger.
func (s *CommitService) CommitOps(ops []transaction.Op) error {
	return s.CommitOpsWithArtifacts(ops, nil)
}

// CommitOpsWithArtifacts commits lifecycle file operations and typed artifacts
// as one transaction. Unlike flattening artifacts into raw writes, this path
// acquires their domain locks and validates every ExpectedRevision before any
// move, delete, or write is staged. A stale catalog/config artifact therefore
// aborts the accompanying project rename, trash, or restore without changing the
// workspace.
func (s *CommitService) CommitOpsWithArtifacts(ops []transaction.Op, artifacts []ArtifactChange) error {
	if len(ops) == 0 && len(artifacts) == 0 {
		return nil
	}
	lockPaths, err := artifactLockPaths(artifacts, s.roots)
	if err != nil {
		return err
	}
	return s.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(s.transactionsDir, s.roots)
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}
		if err := createArtifactLockDirectories(lockPaths); err != nil {
			return err
		}
		return withOrderedFileLocks(lockPaths, func() error {
			if err := validateArtifactRevisions(artifacts); err != nil {
				return err
			}

			artifactOps, _, err := expandArtifactChanges(artifacts)
			if err != nil {
				return err
			}
			allOps := make([]transaction.Op, 0, len(ops)+len(artifactOps))
			allOps = append(allOps, ops...)
			allOps = append(allOps, artifactOps...)

			manifest, err := journal.Prepare(allOps)
			if err != nil {
				return fmt.Errorf("preparing transaction: %w", err)
			}
			// Prepare creates ordinary write parents while staging. A write nested
			// below an earlier directory move is staged below that move's source and
			// the move itself creates the destination parent. Creating write parents
			// here would occupy a rename destination and make the move fail. Only a
			// move's containing directory must be created explicitly.
			for _, op := range allOps {
				switch op.Kind {
				case transaction.OpMove:
					if mkErr := os.MkdirAll(filepath.Dir(filepath.Clean(op.Target)), DirPermissions); mkErr != nil {
						return fmt.Errorf("creating commit target directory: %w", mkErr)
					}
				case transaction.OpWrite, transaction.OpDelete:
					// Prepare or an earlier move provides write parents. Deletes need none.
				}
			}
			if err := journal.Commit(manifest); err != nil {
				return fmt.Errorf("committing transaction: %w", err)
			}
			return nil
		})
	})
}

// CommitArtifactTransaction commits an artifact-only user mutation as one
// recoverable transaction and, when the service has a workspace ledger, records
// its byte-exact before/after images as one undo entry. It deliberately does not
// rewrite any project tasks.md file. ProjectIDs control ledger selection only: an
// empty set makes the entry global-only, while any listed project can select and
// reverse the entire grouped transaction through ProjectScope.
func (s *CommitService) CommitArtifactTransaction(request ArtifactCommitRequest) (string, error) {
	return s.commitArtifacts(request, true)
}

// CommitArtifacts commits configuration-domain artifacts with no accompanying
// project tasks.md as one atomic journaled transaction. It remains the legacy
// config-only wrapper: although it shares the artifact transaction machinery, it
// intentionally records no workspace-ledger entry so existing configuration
// saves do not become user-visible undo steps.
func (s *CommitService) CommitArtifacts(artifacts []ArtifactChange) (string, error) {
	return s.commitArtifacts(ArtifactCommitRequest{Artifacts: artifacts}, false)
}

// CommitProjectRepair persists normalization performed while loading a project
// as a dedicated journaled maintenance transaction. expectedRawRevision must be
// the raw tasks.md revision returned by the load that produced project. The
// revision is checked under the canonical project lock before any bytes are
// staged, so a CLI or another TUI process that commits between load and repair
// makes this operation fail with ErrStaleArtifact instead of being overwritten.
//
// Repairs intentionally do not create a user-visible undo entry: they preserve
// the parser's normalized interpretation of existing data rather than expressing
// a user mutation. They still use the workspace journal so an interrupted repair
// is recoverable through the normal startup recovery boundary.
func (s *CommitService) CommitProjectRepair(project *models.Project, expectedRawRevision string) (string, error) {
	if project == nil {
		return "", fmt.Errorf("storage: project repair requires a project")
	}
	if err := auditProjectBeforeSave(project); err != nil {
		return "", err
	}
	doc, err := CanonicalProjectDocument(project, nil)
	if err != nil {
		return "", fmt.Errorf("rendering project repair for %s: %w", project.Name, err)
	}
	return s.CommitArtifacts([]ArtifactChange{{
		Path:             tasksFilePath(project.Path),
		Data:             []byte(doc.WriteV2()),
		Mode:             FilePermissions,
		ExpectedRevision: &expectedRawRevision,
	}})
}

// prepareTransaction stages one journal manifest and, when record is non-nil and
// an enabled workspace ledger is configured, includes the complete ledger append
// in that same manifest. A disabled ledger yields a user-operations-only manifest.
// The transaction ID is allocated before ledger planning so the entry and manifest
// share an identity. User operations stay first; retained blobs and ledger.json
// follow, with ledger.json last by prepareRecord's contract. A ledger planning
// failure returns before Journal.Prepare stages any operation.
func (s *CommitService) prepareTransaction(
	journal *transaction.Journal,
	ops []transaction.Op,
	record *LedgerRecord,
) (*transaction.Manifest, *preparedLedgerRecord, error) {
	if record == nil || s.ledger == nil {
		manifest, err := journal.Prepare(ops)
		if err != nil {
			return nil, nil, fmt.Errorf("preparing transaction: %w", err)
		}
		return manifest, nil, nil
	}

	txnID := uuid.NewString()
	ledgerRecord := *record
	ledgerRecord.TransactionID = txnID
	ledgerPlan, err := s.ledger.prepareRecord(ledgerRecord)
	if err != nil {
		return nil, nil, fmt.Errorf("preparing undo ledger entry: %w", err)
	}
	if ledgerPlan == nil {
		manifest, prepareErr := journal.PrepareWithID(txnID, ops)
		if prepareErr != nil {
			return nil, nil, fmt.Errorf("preparing transaction: %w", prepareErr)
		}
		return manifest, nil, nil
	}

	combinedOps := make([]transaction.Op, 0, len(ops)+len(ledgerPlan.ops))
	combinedOps = append(combinedOps, ops...)
	combinedOps = append(combinedOps, ledgerPlan.ops...)
	manifest, err := journal.PrepareWithID(txnID, combinedOps)
	if err != nil {
		return nil, nil, fmt.Errorf("preparing transaction: %w", err)
	}
	return manifest, ledgerPlan, nil
}

// commitArtifacts is the shared artifact-only persistence path. recordUndo is
// explicit rather than inferred from empty metadata because a valid global-only
// user mutation may have no project IDs or description, while CommitArtifacts
// must preserve its historical no-ledger behavior even on a ledger-backed
// service.
func (s *CommitService) commitArtifacts(request ArtifactCommitRequest, recordUndo bool) (string, error) {
	artifacts := request.Artifacts
	if len(artifacts) == 0 {
		return "", fmt.Errorf("storage: commit requires at least one artifact")
	}
	artifactLocks, lockErr := artifactLockPaths(artifacts, s.roots)
	if lockErr != nil {
		return "", lockErr
	}
	preconditionLocks, lockErr := artifactPreconditionLockPaths(request.Preconditions, s.roots)
	if lockErr != nil {
		return "", lockErr
	}
	artifactLocks = lockPathsNotHeldBy(artifactLocks, preconditionLocks)
	var txnID string
	commitErr := s.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(s.transactionsDir, s.roots)
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		// Preconditions guard the validity of artifact locations themselves. Take
		// and validate those locks first so a stale project path is rejected before
		// creating a note lock directory below a location that was just moved or
		// trashed. The workspace commit lock serializes other CommitServices while
		// config writers serialize on the held precondition lock.
		if err := createArtifactLockDirectories(preconditionLocks); err != nil {
			return err
		}
		return withOrderedFileLocks(preconditionLocks, func() error {
			if err := validateArtifactPreconditions(request.Preconditions); err != nil {
				return err
			}
			if err := createArtifactLockDirectories(artifactLocks); err != nil {
				return err
			}
			return withOrderedFileLocks(artifactLocks, func() error {
				// Preflight every artifact revision while the read-only guards remain
				// held, before capturing ledger images or staging journal operations.
				if err := validateArtifactRevisions(artifacts); err != nil {
					return err
				}
				var ledgerArtifacts []LedgerArtifact
				if recordUndo && s.ledger != nil {
					var captureErr error
					ledgerArtifacts, captureErr = captureArtifactLedgerImages(artifacts)
					if captureErr != nil {
						return fmt.Errorf("capturing undo images for artifact transaction: %w", captureErr)
					}
				}
				ops, dirsToCreate, expandErr := expandArtifactChanges(artifacts)
				if expandErr != nil {
					return expandErr
				}

				var ledgerRecord *LedgerRecord
				if recordUndo && s.ledger != nil {
					ledgerRecord = &LedgerRecord{
						Action:     request.UndoAction,
						Context:    request.UndoContext,
						ProjectIDs: request.ProjectIDs,
						Artifacts:  ledgerArtifacts,
					}
				}
				manifest, ledgerPlan, prepareErr := s.prepareTransaction(journal, ops, ledgerRecord)
				if prepareErr != nil {
					return prepareErr
				}
				for _, dir := range dirsToCreate {
					if err := os.MkdirAll(dir, DirPermissions); err != nil {
						return fmt.Errorf("creating commit target directory: %w", err)
					}
				}
				if err := journal.Commit(manifest); err != nil {
					return fmt.Errorf("committing transaction: %w", err)
				}
				if ledgerPlan != nil {
					// Cleanup is deliberately post-commit and best-effort. A crash can leak
					// an unreferenced content-addressed blob, but the journal has already
					// made the user artifacts and the ledger entry durable together.
					s.ledger.gcBlobs(ledgerPlan.file)
				}
				txnID = manifest.ID
				return nil
			})
		})
	})
	if commitErr != nil {
		return "", commitErr
	}
	return txnID, nil
}

func lockPathsNotHeldBy(paths, held []string) []string {
	heldSet := make(map[string]bool, len(held))
	for _, path := range held {
		heldSet[filepath.Clean(path)] = true
	}
	remaining := make([]string, 0, len(paths))
	for _, path := range paths {
		if !heldSet[filepath.Clean(path)] {
			remaining = append(remaining, path)
		}
	}
	return remaining
}

// RawArtifactRevision returns the optimistic-concurrency revision of an artifact's
// raw on-disk bytes: the hex-encoded SHA-256 of data. It is the storage-side
// definition of ArtifactChange.ExpectedRevision, and it matches the hash the
// config domain store computes for the same bytes, so a revision observed at load
// time by a config coordinator validates against the same file here.
func RawArtifactRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// validateArtifactRevisions enforces every artifact's ExpectedRevision contract
// against the current on-disk bytes. It must be called under the artifact locks so
// the observed state cannot change between the check and the staged write. A nil
// ExpectedRevision skips the artifact; an empty expected revision requires the
// file to be absent; a non-empty expected revision requires the current raw hash
// to match exactly. Any violation returns an *ArtifactConflictError.
func validateArtifactRevisions(artifacts []ArtifactChange) error {
	for i := range artifacts {
		a := artifacts[i]
		if a.ExpectedRevision == nil {
			continue
		}
		checkPath := artifactRevisionPath(a)
		data, err := os.ReadFile(checkPath)
		switch {
		case os.IsNotExist(err):
			if *a.ExpectedRevision != "" {
				return &ArtifactConflictError{Path: checkPath, Expected: *a.ExpectedRevision, Actual: ""}
			}
		case err != nil:
			return fmt.Errorf("reading artifact %s for concurrency check: %w", filepath.Base(checkPath), err)
		default:
			actual := RawArtifactRevision(data)
			if *a.ExpectedRevision == "" {
				return &ArtifactConflictError{Path: checkPath, Expected: "", Actual: actual}
			}
			if actual != *a.ExpectedRevision {
				return &ArtifactConflictError{Path: checkPath, Expected: *a.ExpectedRevision, Actual: actual}
			}
		}
	}
	return nil
}

// validateArtifactPreconditions checks read-only artifact guards under their
// held advisory locks. It deliberately shares ArtifactConflictError with
// ArtifactChange revision failures so callers can classify both as
// ErrStaleArtifact while inspecting the conflicting path and revisions.
func validateArtifactPreconditions(preconditions []ArtifactPrecondition) error {
	for i := range preconditions {
		precondition := preconditions[i]
		data, err := os.ReadFile(precondition.Path)
		switch {
		case os.IsNotExist(err):
			if precondition.ExpectedRevision != "" {
				return &ArtifactConflictError{Path: precondition.Path, Expected: precondition.ExpectedRevision, Actual: ""}
			}
		case err != nil:
			return fmt.Errorf("reading artifact %s for precondition check: %w", filepath.Base(precondition.Path), err)
		default:
			actual := RawArtifactRevision(data)
			if precondition.ExpectedRevision == "" || actual != precondition.ExpectedRevision {
				return &ArtifactConflictError{Path: precondition.Path, Expected: precondition.ExpectedRevision, Actual: actual}
			}
		}
	}
	return nil
}

func artifactRevisionPath(artifact ArtifactChange) string {
	if artifact.ExpectedRevisionPath != "" {
		return artifact.ExpectedRevisionPath
	}
	return artifact.Path
}

func artifactAdvisoryLockPath(path string) string {
	if filepath.Base(path) == tasksFileName {
		return filepath.Join(filepath.Dir(path), LockFileName)
	}
	return path + ".lock"
}

// validateProjectPaths checks every path CommitMulti reads, writes, or opens for
// a project before the workspace lock can create the transactions directory.
// PathWithinRoot resolves existing symlinks, so an escaping project directory,
// tasks.md, backup, or advisory lock is rejected without filesystem effects.
func validateProjectPaths(changeset int, projectPath string, roots []string) error {
	if projectPath == "" {
		return fmt.Errorf("storage: changeset %d project requires a path", changeset)
	}
	checks := []artifactPathRole{
		{path: projectPath, role: "project directory"},
		{path: tasksFilePath(projectPath), role: "project tasks"},
		{path: backupFilePath(projectPath), role: "project backup"},
		{path: filepath.Join(projectPath, LockFileName), role: "project lock"},
	}
	for _, check := range checks {
		if !pathWithinApprovedRoots(check.path, roots) {
			return fmt.Errorf("storage: changeset %d %s %q: %w", changeset, check.role, check.path, transaction.ErrTargetOutsideRoot)
		}
	}
	return nil
}

// validateProjectPreconditionPath checks every path a read-only project guard
// may inspect or open before the workspace lock can create transaction or lock
// files. A guard has no backup target because it never writes tasks.md.
func validateProjectPreconditionPath(changeset, precondition int, projectPath string, roots []string) error {
	if projectPath == "" {
		return fmt.Errorf("storage: changeset %d project precondition %d requires a path", changeset, precondition)
	}
	checks := []artifactPathRole{
		{path: projectPath, role: "project directory"},
		{path: tasksFilePath(projectPath), role: "project tasks"},
		{path: filepath.Join(projectPath, LockFileName), role: "project lock"},
	}
	for _, check := range checks {
		if !pathWithinApprovedRoots(check.path, roots) {
			return fmt.Errorf("storage: changeset %d project precondition %d %s %q: %w", changeset, precondition, check.role, check.path, transaction.ErrTargetOutsideRoot)
		}
	}
	return nil
}

// artifactLockPaths returns the deterministic lock-file path for every artifact
// target after validating both the write target and revision source against the
// approved roots. It is deliberately read-only: all artifact and lock paths,
// including existing symlink prefixes, must pass containment before the commit
// creates its workspace lock, target directories, staging files, or manifest.
// Domain artifacts use the adjacent ".lock" suffix; tasks.md uses the canonical
// project .tasks.md.lock so lifecycle rewrites serialize with ordinary commits.
func artifactLockPaths(artifacts []ArtifactChange, roots []string) ([]string, error) {
	if err := validateArtifactPaths(artifacts, roots); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(artifacts)*2)
	for i := range artifacts {
		artifact := artifacts[i]
		checkPath := artifactRevisionPath(artifact)
		paths = append(paths, artifactAdvisoryLockPath(checkPath))
		// A marked artifact reads Path to capture its backup. When revision
		// validation uses a different source, also hold the primary's advisory lock
		// so a checked single-file writer cannot change those bytes between capture
		// and journal preparation.
		if artifact.BackupPath != "" && artifactPathIdentity(checkPath) != artifactPathIdentity(artifact.Path) {
			paths = append(paths, artifactAdvisoryLockPath(artifact.Path))
		}
	}
	return paths, nil
}

type artifactPathRole struct {
	path string
	role string
}

// validateArtifactPaths checks every primary, revision source, backup target,
// and adjacent advisory lock before the commit boundary creates directories or
// lock files. Backup targets may not alias their own primary, another backup, an
// artifact target/revision source, or a lock file. Identity resolution follows
// existing symlink prefixes so an in-root symlink alias cannot evade collision
// checks while pathWithinApprovedRoots separately rejects symlink escapes.
func validateArtifactPaths(artifacts []ArtifactChange, roots []string) error {
	reserved := make([]artifactPathRole, 0, len(artifacts)*4)
	backups := make([]artifactPathRole, 0, len(artifacts))
	for i := range artifacts {
		artifact := artifacts[i]
		if artifact.Path == "" {
			return fmt.Errorf("storage: artifact change requires a path")
		}
		checkPath := artifactRevisionPath(artifact)
		checks := []artifactPathRole{
			{path: artifact.Path, role: fmt.Sprintf("artifact %d target", i)},
			{path: checkPath, role: fmt.Sprintf("artifact %d revision source", i)},
			{path: artifactAdvisoryLockPath(artifact.Path), role: fmt.Sprintf("artifact %d target lock", i)},
			{path: artifactAdvisoryLockPath(checkPath), role: fmt.Sprintf("artifact %d revision lock", i)},
		}
		for _, check := range checks {
			if !pathWithinApprovedRoots(check.path, roots) {
				return fmt.Errorf("storage: %s %q: %w", check.role, check.path, transaction.ErrTargetOutsideRoot)
			}
			reserved = append(reserved, check)
		}
		if artifact.BackupPath == "" {
			continue
		}
		if !pathWithinApprovedRoots(artifact.BackupPath, roots) {
			return fmt.Errorf("storage: artifact %d backup target %q: %w", i, artifact.BackupPath, transaction.ErrTargetOutsideRoot)
		}
		backupLock := artifactAdvisoryLockPath(artifact.BackupPath)
		if !pathWithinApprovedRoots(backupLock, roots) {
			return fmt.Errorf("storage: artifact %d backup lock %q: %w", i, backupLock, transaction.ErrTargetOutsideRoot)
		}
		backups = append(backups, artifactPathRole{path: artifact.BackupPath, role: fmt.Sprintf("artifact %d backup", i)})
	}

	for i, backup := range backups {
		backupID := artifactPathIdentity(backup.path)
		for _, path := range reserved {
			if backupID != artifactPathIdentity(path.path) {
				continue
			}
			if strings.Contains(path.role, " target") {
				return fmt.Errorf("storage: %s path %q is self-referential or collides with %s %q", backup.role, backup.path, path.role, path.path)
			}
			return fmt.Errorf("storage: %s path %q collides with %s %q", backup.role, backup.path, path.role, path.path)
		}
		for j := 0; j < i; j++ {
			if backupID == artifactPathIdentity(backups[j].path) {
				return fmt.Errorf("storage: %s path %q collides with %s path %q", backup.role, backup.path, backups[j].role, backups[j].path)
			}
		}
	}
	return nil
}

// artifactPathIdentity resolves symlinks on the longest existing prefix and
// rejoins any missing suffix. It is intentionally the collision-key counterpart
// to util.PathWithinRoot's containment resolution.
func artifactPathIdentity(path string) string {
	current := filepath.Clean(path)
	var rest []string
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, rest[i])
			}
			return filepath.Clean(resolved)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path)
		}
		rest = append(rest, filepath.Base(current))
		current = parent
	}
}

// expandArtifactChanges converts typed artifacts into ordered journal
// operations. Callers invoke it only after revision validation while holding the
// workspace and artifact locks. A marked artifact snapshots the current primary
// into a private backup write before its primary write/delete; an absent primary
// adds no backup op. The returned directories cover every generated write.
func expandArtifactChanges(artifacts []ArtifactChange) ([]transaction.Op, []string, error) {
	ops := make([]transaction.Op, 0, len(artifacts)*2)
	dirsToCreate := make([]string, 0, len(artifacts)*2)
	for _, artifact := range artifacts {
		if artifact.BackupPath != "" {
			current, err := os.ReadFile(artifact.Path)
			switch {
			case err == nil:
				ops = append(ops, transaction.Op{
					Kind:   transaction.OpWrite,
					Target: artifact.BackupPath,
					Data:   current,
					Mode:   FilePermissions,
				})
				dirsToCreate = append(dirsToCreate, filepath.Dir(artifact.BackupPath))
			case os.IsNotExist(err):
				// First save: there is no preceding committed value to preserve.
			default:
				return nil, nil, fmt.Errorf("reading artifact %s for backup: %w", filepath.Base(artifact.Path), err)
			}
		}

		if artifact.Delete {
			ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: artifact.Path})
			continue
		}
		mode := artifact.Mode
		if mode == 0 {
			mode = FilePermissions
		}
		ops = append(ops, transaction.Op{Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: mode})
		dirsToCreate = append(dirsToCreate, filepath.Dir(artifact.Path))
	}
	return ops, dirsToCreate, nil
}

// artifactPreconditionLockPaths validates each read-only guard and returns the
// same adjacent advisory lock path an ArtifactChange revision check would use.
func artifactPreconditionLockPaths(preconditions []ArtifactPrecondition, roots []string) ([]string, error) {
	paths := make([]string, 0, len(preconditions))
	for i := range preconditions {
		path := preconditions[i].Path
		if path == "" {
			return nil, fmt.Errorf("storage: artifact precondition requires a path")
		}
		if !pathWithinApprovedRoots(path, roots) {
			return nil, fmt.Errorf("storage: artifact precondition %q: %w", path, transaction.ErrTargetOutsideRoot)
		}
		lockPath := artifactAdvisoryLockPath(path)
		if !pathWithinApprovedRoots(lockPath, roots) {
			return nil, fmt.Errorf("storage: artifact precondition lock %q: %w", lockPath, transaction.ErrTargetOutsideRoot)
		}
		paths = append(paths, lockPath)
	}
	return paths, nil
}

// pathWithinApprovedRoots applies the journal's lexical and existing-symlink
// containment rule to a path without mutating the filesystem.
func pathWithinApprovedRoots(path string, roots []string) bool {
	for _, root := range roots {
		if util.PathWithinRoot(root, path) {
			return true
		}
	}
	return false
}

// createArtifactLockDirectories creates the already-validated parent directories
// needed by WithFileLock. Callers must invoke artifactLockPaths before entering any
// mutation-capable commit boundary and pass only its returned paths here.
func createArtifactLockDirectories(lockPaths []string) error {
	for _, lockPath := range lockPaths {
		if err := os.MkdirAll(filepath.Dir(lockPath), DirPermissions); err != nil {
			return fmt.Errorf("creating artifact lock directory: %w", err)
		}
	}
	return nil
}

// workspaceCommitLockName is the single workspace-level lock file that serializes
// the entire commit pipeline for a workspace.
const workspaceCommitLockName = "commit.lock"

type workspaceCommitLockTarget struct {
	identity string
	service  *CommitService
}

// WithWorkspaceCommitLocks runs fn while holding the commit locks for every
// supplied workspace. Locks are resolved to canonical filesystem identities,
// de-duplicated, and acquired in sorted order so callers that request the same
// workspaces in opposite directions cannot deadlock. Once every lock is held,
// each distinct workspace journal is recovered before fn is invoked.
//
// A CommitService supplies both the canonical transactions directory and the
// approved roots needed to recover that workspace's journal. The locks remain
// held until fn returns.
func WithWorkspaceCommitLocks(services []*CommitService, fn func() error) error {
	return withWorkspaceCommitLocks(services, true, fn)
}

// withWorkspaceCommitLock runs fn while holding an exclusive lock on one stable
// per-workspace lock file. It serializes the whole commit pipeline — crash
// recovery and the project-scoped apply together — across every committer of the
// same workspace, whether concurrent goroutines in one process or separate tt
// processes (for example a CLI invocation racing the TUI).
//
// The per-project locks alone are not enough: journal.Recover() runs before the
// project locks are taken and walks the shared transactions directory, so two
// committers could recover — and re-drive — each other's in-flight transactions
// at once, leaving both observing the same pre-image and both "succeeding" while
// one update is silently lost. This lock is acquired first, before any project
// lock, at one fixed path, so it can never deadlock with the ordered per-project
// locks.
func (s *CommitService) withWorkspaceCommitLock(fn func() error) error {
	return withWorkspaceCommitLocks([]*CommitService{s}, false, fn)
}

// withWorkspaceCommitLocks is the shared lock acquisition primitive. The
// recover flag is false for existing CommitService entry points because those
// paths perform recovery inside their callbacks as part of their established
// pipeline; the exported multi-workspace boundary requests recovery here.
func withWorkspaceCommitLocks(services []*CommitService, recover bool, fn func() error) error {
	if len(services) == 0 {
		return fmt.Errorf("storage: at least one workspace commit service is required")
	}
	if fn == nil {
		return fmt.Errorf("storage: workspace commit lock callback is required")
	}

	targets := make([]workspaceCommitLockTarget, 0, len(services))
	for i, service := range services {
		if service == nil {
			return fmt.Errorf("storage: workspace commit service %d is nil", i)
		}
		if service.transactionsDir == "" || service.transactionsDir == "." {
			return fmt.Errorf("storage: workspace commit service %d requires a transactions directory", i)
		}
		if err := os.MkdirAll(service.transactionsDir, DirPermissions); err != nil {
			return fmt.Errorf("creating transactions directory %s: %w", service.transactionsDir, err)
		}
		targets = append(targets, workspaceCommitLockTarget{
			identity: artifactPathIdentity(filepath.Join(service.transactionsDir, workspaceCommitLockName)),
			service:  service,
		})
	}

	sort.SliceStable(targets, func(i, j int) bool {
		return targets[i].identity < targets[j].identity
	})

	orderedLocks := make([]string, 0, len(targets))
	distinctJournals := make([]*CommitService, 0, len(targets))
	for _, target := range targets {
		if len(orderedLocks) > 0 && orderedLocks[len(orderedLocks)-1] == target.identity {
			continue
		}
		orderedLocks = append(orderedLocks, target.identity)
		distinctJournals = append(distinctJournals, target.service)
	}

	return acquireLocksInOrder(orderedLocks, func() error {
		if recover {
			for _, service := range distinctJournals {
				journal := transaction.NewJournal(service.transactionsDir, service.roots)
				if _, err := service.recoverLocked(journal); err != nil {
					return err
				}
			}
		}
		return fn()
	})
}

// recoverLocked runs one journal recovery pass while the caller holds the
// workspace commit lock. Keeping the quarantine classification here gives
// startup recovery and every mutation entry point the same fail-closed policy.
func (s *CommitService) recoverLocked(journal *transaction.Journal) (*transaction.RecoveryReport, error) {
	report, err := journal.Recover()
	if err != nil {
		return report, fmt.Errorf("transaction recovery failed: %w", err)
	}
	if report.Blocked() {
		return report, fmt.Errorf("%w: %w", ErrCommitBlocked, report.Err())
	}
	return report, nil
}

func (s *CommitService) preflightChangesets(changesets []ChangeSet) ([]*ProjectDocument, []RecordedConflict, error) {
	// mergedDocs[i], when non-nil, is the three-way-merged document that replaces
	// changeset i's local intent because its expected revision was stale but the
	// concurrent edits merged cleanly. Phase 3 writes it verbatim instead of
	// re-rendering the caller's local document.
	mergedDocs := make([]*ProjectDocument, len(changesets))
	// recorded accumulates every project whose stale-revision merge produced true
	// conflicts. A non-empty set aborts the whole commit after every conflict is
	// durably recorded, so no project or artifact is written.
	var recorded []RecordedConflict

	// Project preconditions are deliberately a separate first phase. They guard
	// the complete application-level read set and must expire atomically before
	// the ordinary ExpectedRevision path can mutate activity, attempt a merge, or
	// record a durable conflict bundle for any changeset.
	if err := s.validateProjectRevisionPreconditions(changesets); err != nil {
		return nil, nil, err
	}

	// Phase 1: preflight every project's expected revision under the held locks.
	// No bytes are written yet, so a conflict here leaves every project untouched.
	for i := range changesets {
		cs := changesets[i]
		project := cs.Project
		currentDoc, currentRevision, err := s.currentDocAndRevision(tasksFilePath(project.Path), project)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case cs.ExpectedRevision == "" || cs.ExpectedRevision == currentRevision:
			// In sync (or the caller opted out of the check): nothing to merge. Record
			// the activity diff against the current (== the loaded base) document, as
			// the legacy save path did, before Phase 3 renders the local intent. This
			// runs ONLY here: diffing the local intent against a divergent on-disk
			// document would append activity entries that then read as a spurious
			// same-field merge conflict, so the merge path below is left untouched.
			if currentDoc != nil {
				diffAndRecordActivity(currentDoc.Tasks, project.Tasks, MaxActivityEntries)
			}
		case s.conflicts != nil && cs.Base != nil && currentDoc != nil:
			// Stale revision with a real merge base and a durable conflict store:
			// three-way merge the loaded base, the caller's local intent, and the
			// current on-disk document under the held locks. A clean non-overlapping
			// merge replaces the local intent and commits against the current
			// revision; a true conflict is durably recorded as retry-only and aborts
			// the whole set. Retry-only is required because this project bundle cannot
			// represent the complete ChangeSet (including artifacts, attachment bytes,
			// clean companion projects, and undo intent).
			local, mergeErr := CanonicalProjectDocument(project, cs.Base)
			if mergeErr != nil {
				return nil, nil, fmt.Errorf("rendering local document for merge of %s: %w", project.Name, mergeErr)
			}
			merge, conflictID, mergeErr := MergeOrRecordConflict(s.conflicts, project, cs.Base, local, currentDoc, cs.UndoAction, cs.UndoContext)
			if mergeErr != nil {
				return nil, nil, fmt.Errorf("merging stale project %s: %w", project.Name, mergeErr)
			}
			if conflictID != "" {
				recorded = append(recorded, RecordedConflict{
					ProjectID:   project.ID,
					ProjectName: project.Name,
					ConflictID:  conflictID,
					Conflicts:   merge.Conflicts,
				})
				continue
			}
			// The merge retained current's durable activity history while excluding
			// Activity from user-authored field conflicts. Record the accepted local
			// business delta against current exactly once before canonicalization, so
			// a clean stale commit has the same audit semantics as an in-sync commit.
			diffAndRecordActivity(currentDoc.Tasks, merge.Merged.Tasks, MaxActivityEntries)
			// Re-parse the merged rendering so the committed revision is a fixed point
			// (reload-stable): it equals exactly what the next LoadProjectDocument sees.
			canonical, canonErr := ParseProjectDocument(merge.Merged.WriteV2())
			if canonErr != nil {
				return nil, nil, fmt.Errorf("canonicalizing merged document for %s: %w", project.Name, canonErr)
			}
			// The merge folded in the current on-disk state, so mergedDocs[i] is what
			// Phase 3 writes; the write is safe under the still-held project lock (no
			// further revision check is needed, the merge already reconciled current).
			mergedDocs[i] = canonical
		default:
			// Stale with no merge capability (nil base, corrupt or absent current doc,
			// or no conflict store): abort as before so the caller reloads and retries.
			return nil, nil, &RevisionConflictError{
				ProjectID:   project.ID,
				ProjectName: project.Name,
				Expected:    cs.ExpectedRevision,
				Actual:      currentRevision,
			}
		}
		// Validate every artifact's optimistic revision under the same held locks,
		// before any bytes are staged, so a stale config artifact aborts the whole
		// set without touching disk.
		if err := validateArtifactRevisions(cs.Artifacts); err != nil {
			return nil, nil, err
		}
	}
	return mergedDocs, recorded, nil
}

func (s *CommitService) validateProjectRevisionPreconditions(changesets []ChangeSet) error {
	for i := range changesets {
		for j := range changesets[i].ProjectPreconditions {
			precondition := changesets[i].ProjectPreconditions[j]
			identity := &models.Project{
				ID:   precondition.ProjectID,
				Name: precondition.ProjectName,
				Path: precondition.Path,
			}
			_, actual, err := s.currentDocAndRevision(tasksFilePath(precondition.Path), identity)
			if err != nil {
				return err
			}
			if actual != precondition.ExpectedRevision {
				return &RevisionConflictError{
					ProjectID:   precondition.ProjectID,
					ProjectName: precondition.ProjectName,
					Expected:    precondition.ExpectedRevision,
					Actual:      actual,
				}
			}
		}
	}
	return nil
}

// commitMultiLocked performs the full preflight, staging, and commit for every
// changeset while the caller holds every affected project's exclusive lock. It
// verifies all expected revisions before touching any file, so a stale revision
// anywhere aborts the whole set before any backup or staging is written.
func (s *CommitService) commitMultiLocked(journal *transaction.Journal, changesets []ChangeSet) ([]*CommitResult, error) {
	mergedDocs, recorded, preflightErr := s.preflightChangesets(changesets)
	if preflightErr != nil {
		return nil, preflightErr
	}

	// A single unresolved merge aborts the whole set: nothing is written, but every
	// conflict has been durably recorded for later resolution through the shared
	// resolution service.
	if len(recorded) > 0 {
		return nil, &CommitConflictError{Conflicts: recorded}
	}

	// Phase 2: back up every project's current tasks.md as an internal repair hook
	// (matches the legacy save path); best-effort and outside the transaction.
	for i := range changesets {
		if err := BackupProject(changesets[i].Project); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Phase 3: build every project write plus every artifact as ONE transaction so
	// the whole cross-project mutation is atomic. The markdown content is
	// byte-identical to the legacy save path so on-disk output is unchanged; only
	// the write mechanism moves behind the journal.
	ops := make([]transaction.Op, 0, len(changesets))
	// dirsToCreate collects target parent directories that must exist before the
	// journal renames staged bytes into place: attachment move/copy destinations
	// and any artifact write whose parent (e.g. a project's notes/ subdirectory)
	// does not yet exist. They are created only after Prepare validates every
	// target and before Commit applies the ops, so a rejected target never leaves
	// an empty directory behind.
	var dirsToCreate []string
	// When a ledger is attached, capture the before/after image of every file
	// each changeset touches under the held locks, before the transaction applies
	// (so before-images are the real pre-mutation bytes and an attachment delete
	// or move can be reversed). recordAction/recordContext take the first non-empty
	// description across the set — the whole multi-commit is one ledger entry.
	var ledgerArtifacts []LedgerArtifact
	var ledgerProjectIDs []string
	var recordAction, recordContext string
	committedDocs := make([]*ProjectDocument, len(changesets))
	for i := range changesets {
		cs := changesets[i]
		project := cs.Project
		// A stale changeset that merged cleanly writes the merged document; every
		// other changeset renders the caller's local intent over its base.
		committedDoc := mergedDocs[i]
		if committedDoc == nil {
			var err error
			committedDoc, err = CanonicalProjectDocument(project, cs.Base)
			if err != nil {
				return nil, fmt.Errorf("rendering project %s: %w", project.Name, err)
			}
		}
		content := committedDoc.WriteV2()
		committedDocs[i] = committedDoc
		if s.ledger != nil {
			arts, err := captureLedgerArtifacts(cs, content)
			if err != nil {
				return nil, fmt.Errorf("capturing undo images for project %s: %w", project.Name, err)
			}
			ledgerArtifacts = append(ledgerArtifacts, arts...)
			ledgerProjectIDs = append(ledgerProjectIDs, undoProjectID(project))
			if recordAction == "" {
				recordAction = cs.UndoAction
			}
			if recordContext == "" {
				recordContext = cs.UndoContext
			}
		}
		ops = append(ops, transaction.Op{
			Kind:   transaction.OpWrite,
			Target: tasksFilePath(project.Path),
			Data:   []byte(content),
			Mode:   FilePermissions,
		})
		artifactOps, artifactDirs, err := expandArtifactChanges(cs.Artifacts)
		if err != nil {
			return nil, err
		}
		ops = append(ops, artifactOps...)
		dirsToCreate = append(dirsToCreate, artifactDirs...)
		// Resolve this changeset's attachment side effects into transaction ops
		// under the held locks. A resolution failure (missing copy source, occupied
		// move destination, nested directory) aborts the whole commit here — before
		// Prepare stages any bytes — so no project or artifact is written.
		attachOps, dirs, attachErr := resolveAttachmentOps(cs.AttachmentOps)
		if attachErr != nil {
			return nil, fmt.Errorf("resolving attachment operations for project %s: %w", project.Name, attachErr)
		}
		ops = append(ops, attachOps...)
		dirsToCreate = append(dirsToCreate, dirs...)
	}

	var ledgerRecord *LedgerRecord
	if s.ledger != nil {
		ledgerRecord = &LedgerRecord{
			Action:     recordAction,
			Context:    recordContext,
			ProjectIDs: ledgerProjectIDs,
			Artifacts:  ledgerArtifacts,
		}
	}
	manifest, ledgerPlan, err := s.prepareTransaction(journal, ops, ledgerRecord)
	if err != nil {
		return nil, err
	}
	// Create every collected target parent directory (attachment destinations and
	// artifact write parents) now that Prepare validated every target and before
	// Commit applies the operations, so a rejected target never leaves an empty
	// directory and the rename-into-place never fails on a missing parent.
	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(dir, DirPermissions); err != nil {
			return nil, fmt.Errorf("creating commit target directory: %w", err)
		}
	}
	if err := journal.Commit(manifest); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}
	if ledgerPlan != nil {
		// Cleanup is deliberately post-commit and best-effort. A crash can leak an
		// unreferenced content-addressed blob, but the journal has already made the
		// project, companion artifacts, retained blobs, and ledger entry durable.
		s.ledger.gcBlobs(ledgerPlan.file)
	}

	results := make([]*CommitResult, len(changesets))
	for i := range changesets {
		results[i] = &CommitResult{
			Revision:      committedDocs[i].Revision,
			TransactionID: manifest.ID,
		}
	}
	return results, nil
}

// withOrderedFileLocks acquires every distinct lock path in sorted order and runs
// fn while all are held. Sorting gives a deterministic global acquisition order so
// two concurrent multi-commits touching an overlapping set of projects can never
// deadlock, and de-duplication avoids re-locking the same path in-process (flock
// is per-open-file-description, so a second lock on the same path from the same
// process would deadlock against the first).
func withOrderedFileLocks(lockPaths []string, fn func() error) error {
	seen := make(map[string]bool, len(lockPaths))
	ordered := make([]string, 0, len(lockPaths))
	for _, p := range lockPaths {
		identity := artifactPathIdentity(p)
		if seen[identity] {
			continue
		}
		seen[identity] = true
		ordered = append(ordered, identity)
	}
	sort.Strings(ordered)
	return acquireLocksInOrder(ordered, fn)
}

// acquireLocksInOrder recursively acquires each lock in paths (already sorted and
// de-duplicated) and runs fn once every lock is held.
func acquireLocksInOrder(paths []string, fn func() error) error {
	if len(paths) == 0 {
		return fn()
	}
	return util.WithFileLock(paths[0], true, func() error {
		return acquireLocksInOrder(paths[1:], fn)
	})
}

// currentDocAndRevision reads and parses the project's current tasks.md,
// returning the parsed document (the three-way merge's "current" input) and its
// canonical revision, or (nil, "") when the file is absent or unparseable. Unlike
// the legacy read it performs NO activity diff — that mutates the caller's task
// set and must not run before a merge, so Phase 1 records activity only on the
// in-sync path. A nil document routes a stale commit to a plain
// RevisionConflictError since it has no mergeable current, matching the legacy
// save path's tolerance of an unparseable on-disk file.
func (s *CommitService) currentDocAndRevision(tasksPath string, project *models.Project) (*ProjectDocument, string, error) {
	oldContent, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to read tasks file: %w", err)
	}
	oldDoc, parseErr := ParseProjectDocument(string(oldContent))
	if parseErr != nil {
		logging.Warn("could not read current document for %s: %v", project.Name, parseErr)
		return nil, "", nil
	}
	return oldDoc, oldDoc.Revision, nil
}

// resolveAttachmentOps translates a changeset's attachment side effects into
// concrete transaction operations. Copies stage the source bytes for the
// destination path; a move renames a non-empty task attachment directory (or
// deletes an empty source directory); a delete removes every file and then the
// directory. The returned dirsToCreate name destination .attachments parents
// that a move rename needs in place before commit. Any error aborts the caller's
// whole commit.
func resolveAttachmentOps(ops []AttachmentOp) (txnOps []transaction.Op, dirsToCreate []string, err error) {
	for i := range ops {
		op := ops[i]
		switch op.Kind {
		case AttachmentCopy:
			copyOp, copyErr := resolveAttachmentCopy(op)
			if copyErr != nil {
				return nil, nil, copyErr
			}
			txnOps = append(txnOps, copyOp)
		case AttachmentMove:
			moveOps, destParent, moveErr := resolveAttachmentMove(op)
			if moveErr != nil {
				return nil, nil, moveErr
			}
			if destParent != "" {
				dirsToCreate = append(dirsToCreate, destParent)
			}
			txnOps = append(txnOps, moveOps...)
		case AttachmentDelete:
			delOps, delErr := resolveAttachmentDelete(op)
			if delErr != nil {
				return nil, nil, delErr
			}
			txnOps = append(txnOps, delOps...)
		default:
			return nil, nil, fmt.Errorf("storage: unknown attachment op kind %d", op.Kind)
		}
	}
	return txnOps, dirsToCreate, nil
}

// resolveAttachmentCopy stages an external file's bytes for the task's destination
// attachment path. It rejects an existing destination (duplicate filename) so the
// copy contract matches CopyAttachment: a name already present is an error rather
// than a silent overwrite.
func resolveAttachmentCopy(op AttachmentOp) (transaction.Op, error) {
	if op.DestProject == nil {
		return transaction.Op{}, fmt.Errorf("storage: attachment copy requires a destination project")
	}
	if op.SourcePath == "" {
		return transaction.Op{}, fmt.Errorf("storage: attachment copy requires a source path")
	}
	filename := op.Filename
	if filename == "" {
		filename = filepath.Base(op.SourcePath)
	}
	destPath, err := resolveAttachmentPath(op.DestProject, op.Task, filename)
	if err != nil {
		return transaction.Op{}, err
	}
	if _, statErr := os.Stat(destPath); statErr == nil {
		return transaction.Op{}, fmt.Errorf("attachment %q already exists", filename)
	} else if !os.IsNotExist(statErr) {
		return transaction.Op{}, fmt.Errorf("failed to stat attachment destination: %w", statErr)
	}
	data, err := os.ReadFile(op.SourcePath)
	if err != nil {
		return transaction.Op{}, fmt.Errorf("failed to read attachment source: %w", err)
	}
	return transaction.Op{Kind: transaction.OpWrite, Target: destPath, Data: data, Mode: FilePermissions}, nil
}

// resolveAttachmentMove builds the transaction op that renames a task's whole
// attachments directory to the destination project. It is a no-op when the task
// has no attachment directory, deletes an empty source without creating a
// destination, and fails closed when the destination directory already exists.
// The returned destParent is the destination .attachments directory a non-empty
// rename target needs in place.
func resolveAttachmentMove(op AttachmentOp) (ops []transaction.Op, destParent string, err error) {
	if op.SourceProject == nil || op.DestProject == nil {
		return nil, "", fmt.Errorf("storage: attachment move requires source and destination projects")
	}
	srcDir, err := resolveTaskAttachmentsDir(op.SourceProject, op.Task)
	if err != nil {
		return nil, "", err
	}
	info, statErr := os.Stat(srcDir)
	if os.IsNotExist(statErr) {
		return nil, "", nil
	}
	if statErr != nil {
		return nil, "", fmt.Errorf("failed to stat source attachments directory: %w", statErr)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("source attachments path is not a directory: %s", srcDir)
	}
	destDir, err := resolveTaskAttachmentsDir(op.DestProject, op.Task)
	if err != nil {
		return nil, "", err
	}
	if _, destStatErr := os.Stat(destDir); destStatErr == nil {
		return nil, "", fmt.Errorf("destination task attachments directory already exists: %s", destDir)
	} else if !os.IsNotExist(destStatErr) {
		return nil, "", fmt.Errorf("failed to stat destination attachments directory: %w", destStatErr)
	}
	files, _, err := listTaskAttachmentFiles(srcDir)
	if err != nil {
		return nil, "", err
	}
	if len(files) == 0 {
		return []transaction.Op{{Kind: transaction.OpDelete, Target: srcDir}}, "", nil
	}
	return []transaction.Op{{Kind: transaction.OpMove, Source: srcDir, Target: destDir}}, filepath.Dir(destDir), nil
}

// resolveAttachmentDelete builds the delete ops for an attachment removal. When
// op.Filename is set it removes just that one file, leaving the directory and any
// sibling attachments intact (the single-attachment `tt attachment remove` case);
// otherwise it removes every file in the task's attachments directory plus the
// directory itself (the whole-task deletion case). It is a no-op when the target
// is absent — a missing single file is tolerated the same way the legacy
// RemoveAttachmentFile ignored os.IsNotExist. Deleting each file before the
// directory keeps every op a non-recursive removal, matching the transaction
// journal's delete primitive.
func resolveAttachmentDelete(op AttachmentOp) ([]transaction.Op, error) {
	project := op.SourceProject
	if project == nil {
		project = op.DestProject
	}
	if project == nil {
		return nil, fmt.Errorf("storage: attachment delete requires a project")
	}
	if op.Filename != "" {
		return resolveSingleAttachmentDelete(project, op)
	}
	dir, err := resolveTaskAttachmentsDir(project, op.Task)
	if err != nil {
		return nil, err
	}
	files, exists, err := listTaskAttachmentFiles(dir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	ops := make([]transaction.Op, 0, len(files)+1)
	for _, f := range files {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: f})
	}
	ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: dir})
	return ops, nil
}

// resolveSingleAttachmentDelete builds the delete op for one named attachment
// file, leaving the attachments directory and any siblings untouched. A file that
// is already gone yields no op — matching the legacy RemoveAttachmentFile, which
// ignored os.IsNotExist so a metadata removal was never blocked by missing bytes.
func resolveSingleAttachmentDelete(project *models.Project, op AttachmentOp) ([]transaction.Op, error) {
	path, err := resolveAttachmentPath(project, op.Task, op.Filename)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil, nil
	} else if statErr != nil {
		return nil, fmt.Errorf("failed to stat attachment file: %w", statErr)
	}
	return []transaction.Op{{Kind: transaction.OpDelete, Target: path}}, nil
}
