package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
)

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

// ArtifactChange describes a single configuration artifact written or deleted in
// the same transaction as a project's tasks.md. Path is the absolute target; a
// write carries Data (and an optional Mode, defaulting to the standard private
// file permission), while Delete removes the target instead.
type ArtifactChange struct {
	Path   string
	Data   []byte
	Mode   os.FileMode
	Delete bool
}

// ChangeSet is the unit of a single project's mutation committed atomically: the
// project whose tasks.md is rewritten, the revision the caller loaded it at (for
// the optimistic concurrency check), any opaque header blocks to preserve, and
// any additional artifacts to write or delete alongside it in one transaction. A
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
	// Opaque carries header blocks preserved from load so the committed revision
	// reflects lossless content.
	Opaque []OpaqueBlock
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
	// ledger, when non-nil, records every successful commit into the workspace
	// undo ledger as the final finalization step, so callers cannot forget to
	// capture history. A nil ledger disables history recording entirely.
	ledger *WorkspaceLedger
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
// journaling every mutation, records each successful commit into ledger as one
// append-only workspace undo entry. Recording runs after the journal commits, so
// history reflects only mutations that actually landed, and it captures the
// before/after image of every file the transaction touched — project markdown,
// configuration artifacts, and attachment bytes alike — so undo and redo can
// reverse or replay the whole transaction.
func NewCommitServiceWithLedger(transactionsDir string, roots []string, ledger *WorkspaceLedger) *CommitService {
	return &CommitService{transactionsDir: transactionsDir, roots: roots, ledger: ledger}
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
	for i := range changesets {
		project := changesets[i].Project
		if project == nil {
			return nil, fmt.Errorf("storage: changeset %d requires a project", i)
		}
		if err := auditProjectBeforeSave(project); err != nil {
			return nil, err
		}
		cleanPath := filepath.Clean(project.Path)
		if seenPaths[cleanPath] {
			return nil, fmt.Errorf("storage: changeset %d duplicates project path %q; a project may appear at most once per commit", i, project.Path)
		}
		seenPaths[cleanPath] = true
	}

	// The whole commit pipeline — crash recovery plus the project-scoped apply —
	// runs under one workspace-level lock so concurrent committers can never run
	// recovery against each other's in-flight transactions. See
	// withWorkspaceCommitLock for why the per-project locks alone are insufficient.
	var results []*CommitResult
	if err := s.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(s.transactionsDir, s.roots)
		report, recoverErr := journal.Recover()
		if recoverErr != nil {
			return fmt.Errorf("transaction recovery failed: %w", recoverErr)
		}
		if report.Blocked() {
			return fmt.Errorf("%w: %w", ErrCommitBlocked, report.Err())
		}

		lockPaths := make([]string, 0, len(changesets))
		for i := range changesets {
			project := changesets[i].Project
			if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
				return fmt.Errorf("failed to create project directory: %w", err)
			}
			lockPaths = append(lockPaths, filepath.Join(project.Path, LockFileName))
		}

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

// CommitOps commits a set of raw file operations — a directory move, a tombstone
// write, a config rewrite, cross-reference repairs — as ONE atomic transaction
// through the journal, under the workspace commit lock and after crash recovery.
// It is the project-lifecycle counterpart to CommitMulti: where CommitMulti's unit
// is a project's tasks.md plus its artifacts, CommitOps carries arbitrary in-root
// file ops, so archive-location, trash, restore, and purge choreography commits
// atomically or not at all — a crash mid-commit is rolled forward by recovery, and
// any target that resolves outside the approved roots is rejected during Prepare
// before a single byte is staged. It records no undo-ledger entry: project
// trash/restore is reversible through the trash lifecycle itself and purge is
// irreversible, so neither participates in the task-oriented workspace ledger.
func (s *CommitService) CommitOps(ops []transaction.Op) error {
	if len(ops) == 0 {
		return nil
	}
	return s.withWorkspaceCommitLock(func() error {
		journal := transaction.NewJournal(s.transactionsDir, s.roots)
		report, recoverErr := journal.Recover()
		if recoverErr != nil {
			return fmt.Errorf("transaction recovery failed: %w", recoverErr)
		}
		if report.Blocked() {
			return fmt.Errorf("%w: %w", ErrCommitBlocked, report.Err())
		}
		manifest, err := journal.Prepare(ops)
		if err != nil {
			return fmt.Errorf("preparing transaction: %w", err)
		}
		// Create every write/move target's parent directory now that Prepare
		// validated every target and before Commit renames staged bytes (or a moved
		// directory) into place, so a rejected target never leaves an empty directory
		// and the rename never fails on a missing parent.
		for _, op := range ops {
			switch op.Kind {
			case transaction.OpWrite, transaction.OpMove:
				if mkErr := os.MkdirAll(filepath.Dir(filepath.Clean(op.Target)), DirPermissions); mkErr != nil {
					return fmt.Errorf("creating commit target directory: %w", mkErr)
				}
			}
		}
		if err := journal.Commit(manifest); err != nil {
			return fmt.Errorf("committing transaction: %w", err)
		}
		return nil
	})
}

// workspaceCommitLockName is the single workspace-level lock file that serializes
// the entire commit pipeline for a workspace.
const workspaceCommitLockName = "commit.lock"

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
	if err := os.MkdirAll(s.transactionsDir, DirPermissions); err != nil {
		return fmt.Errorf("creating transactions directory: %w", err)
	}
	return util.WithFileLock(filepath.Join(s.transactionsDir, workspaceCommitLockName), true, fn)
}

// commitMultiLocked performs the full preflight, staging, and commit for every
// changeset while the caller holds every affected project's exclusive lock. It
// verifies all expected revisions before touching any file, so a stale revision
// anywhere aborts the whole set before any backup or staging is written.
func (s *CommitService) commitMultiLocked(journal *transaction.Journal, changesets []ChangeSet) ([]*CommitResult, error) {
	// Phase 1: preflight every project's expected revision under the held locks.
	// No bytes are written yet, so a conflict here leaves every project untouched.
	for i := range changesets {
		cs := changesets[i]
		project := cs.Project
		currentRevision, err := s.currentRevisionAndActivity(tasksFilePath(project.Path), project)
		if err != nil {
			return nil, err
		}
		if cs.ExpectedRevision != "" && cs.ExpectedRevision != currentRevision {
			return nil, &RevisionConflictError{
				ProjectID:   project.ID,
				ProjectName: project.Name,
				Expected:    cs.ExpectedRevision,
				Actual:      currentRevision,
			}
		}
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
	for i := range changesets {
		cs := changesets[i]
		project := cs.Project
		content := WriteTasksMarkdownWithFullProjectMetadata(project.Name, project.Tasks, project.GetSections(), project.SectionColors, project.Tags, project.PinnedTags)
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
		for _, artifact := range cs.Artifacts {
			if artifact.Delete {
				ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: artifact.Path})
				continue
			}
			mode := artifact.Mode
			if mode == 0 {
				mode = FilePermissions
			}
			ops = append(ops, transaction.Op{
				Kind:   transaction.OpWrite,
				Target: artifact.Path,
				Data:   artifact.Data,
				Mode:   mode,
			})
			dirsToCreate = append(dirsToCreate, filepath.Dir(artifact.Path))
		}
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

	manifest, err := journal.Prepare(ops)
	if err != nil {
		return nil, fmt.Errorf("preparing transaction: %w", err)
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

	// Finalization: record the committed transaction into the workspace undo
	// ledger. This runs only after the journal committed, so history never
	// references a mutation that did not land. A record failure is surfaced —
	// the bytes are committed, but the caller learns history was not captured.
	if s.ledger != nil {
		if err := s.ledger.Record(LedgerRecord{
			TransactionID: manifest.ID,
			Action:        recordAction,
			Context:       recordContext,
			ProjectIDs:    ledgerProjectIDs,
			Artifacts:     ledgerArtifacts,
		}); err != nil {
			return nil, fmt.Errorf("recording undo ledger entry: %w", err)
		}
	}

	results := make([]*CommitResult, len(changesets))
	for i := range changesets {
		cs := changesets[i]
		results[i] = &CommitResult{
			Revision:      ToProjectDocument(cs.Project, cs.Opaque).Revision,
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
		clean := filepath.Clean(p)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		ordered = append(ordered, clean)
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

// currentRevisionAndActivity reads the project's current tasks.md, returning its
// canonical revision (empty when the file does not yet exist) and recording the
// activity diff against the project's new task set. A parse failure is
// non-fatal: activity diffing and the revision check are skipped rather than
// blocking the save, matching the legacy save path's tolerance.
func (s *CommitService) currentRevisionAndActivity(tasksPath string, project *models.Project) (string, error) {
	oldContent, err := os.ReadFile(tasksPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read tasks file: %w", err)
	}
	oldDoc, parseErr := ParseProjectDocument(string(oldContent))
	if parseErr != nil {
		logging.Warn("could not diff activity for %s: %v", project.Name, parseErr)
		return "", nil
	}
	diffAndRecordActivity(oldDoc.Tasks, project.Tasks, MaxActivityEntries)
	return oldDoc.Revision, nil
}

// resolveAttachmentOps translates a changeset's attachment side effects into
// concrete transaction operations. Copies stage the source bytes for the
// destination path; a move renames the task's whole attachments directory (a no-op
// when the task has none); a delete removes every file and then the directory. The
// returned dirsToCreate name destination .attachments parents that a move rename
// needs in place before commit. Any error aborts the caller's whole commit.
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
// attachments directory to the destination project. It is a no-op (no ops, no
// destination parent) when the task has no attachments directory, and it fails
// closed when the destination directory already exists. The returned destParent is
// the destination .attachments directory the rename target needs in place.
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
