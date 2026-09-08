package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/transaction"
)

// CommitService is the mutation commit boundary. It drives one or more
// changesets through the durable transaction journal: recover any interrupted
// transaction first, acquire every affected project's lock in deterministic
// order, verify each optimistic revision under the locks, then prepare and commit
// every project write plus its artifacts as one atomic transaction. Commit is the
// single-project convenience; CommitMulti spans multiple projects and their
// artifacts in one transaction.
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
	// machineID is the identity stamped onto the transaction manifests this
	// service prepares, and the one recovery compares against when deciding
	// whether an in-flight transaction is this machine's to finish. An empty value
	// means the running process's own machine, which is what every production
	// caller wants; OnMachine exists for the callers that hold an explicit machine
	// home, so two machines sharing a synced workspace never claim each other's
	// live transactions.
	machineID string
}

// OnMachine binds this service to an explicit machine identity and returns the
// same service for fluent construction. An empty machineID leaves the default in
// place: the machine the process is running on.
func (s *CommitService) OnMachine(machineID string) *CommitService {
	s.machineID = machineID
	if s.ledger != nil {
		// The ledger's standalone record and restore paths open their own journals,
		// so they need the same owner as the service that attached the ledger.
		s.ledger.machineID = machineID
	}
	return s
}

// journal returns a transaction journal owned by this service's machine. Every
// commit and recovery path goes through it, so there is exactly one place a
// journal's ownership is decided.
func (s *CommitService) journal() *transaction.Journal {
	return transaction.NewJournal(s.transactionsDir, s.roots, s.machineID)
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
		report, recoverErr = s.recoverLocked(s.journal())
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
	var artifactPreconditions []ArtifactPrecondition
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
		artifactPreconditions = append(artifactPreconditions, changesets[i].ArtifactPreconditions...)
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
	// Read-only artifact guards are containment-checked here, before the workspace
	// lock creates anything, exactly like the writes they accompany. They yield the
	// same adjacent advisory lock a write of those bytes would take, so the guard
	// and a concurrent writer serialize on one lock.
	artifactPreconditionLocks, err := artifactPreconditionLockPaths(artifactPreconditions, s.roots)
	if err != nil {
		return nil, fmt.Errorf("storage: changeset artifact preconditions: %w", err)
	}

	// The whole commit pipeline — crash recovery plus the project-scoped apply —
	// runs under one workspace-level lock so concurrent committers can never run
	// recovery against each other's in-flight transactions. See
	// withWorkspaceCommitLock for why the per-project locks alone are insufficient.
	var results []*CommitResult
	if err := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
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
		if err := createArtifactLockDirectories(artifactPreconditionLocks); err != nil {
			return err
		}
		lockPaths = append(lockPaths, artifactPreconditionLocks...)

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

// CommitOps commits raw file operations as one atomic transaction through the
// journal, under the workspace commit lock and after crash recovery. Call
// CommitOpsWithArtifacts when an operation also carries revision-checked writes.
// It is the project-lifecycle counterpart to CommitMulti: where CommitMulti's unit
// is a project's tasks.md plus its artifacts, CommitOps carries arbitrary in-root
// file ops, so directory and maintenance choreography commits atomically or not at
// all. A crash mid-commit is rolled forward by recovery, and any target resolving
// outside the approved roots is rejected before a byte is staged.
func (s *CommitService) CommitOps(ops []transaction.Op) error {
	return s.CommitOpsWithArtifacts(ops, nil)
}

// CommitOpsWithArtifacts commits lifecycle file operations and typed artifacts
// as one transaction. Unlike flattening artifacts into raw writes, this path
// acquires their domain locks and validates every ExpectedRevision before any
// move, delete, or write is staged. A stale catalog/config artifact therefore
// aborts the accompanying project rename or maintenance operation without changing
// the workspace.
func (s *CommitService) CommitOpsWithArtifacts(ops []transaction.Op, artifacts []ArtifactChange) error {
	_, err := s.commitOps(OpsCommitRequest{Ops: ops, Artifacts: artifacts}, false)
	return err
}

// CommitPlannedOpsWithArtifacts builds and commits one maintenance transaction
// while holding the workspace commit lock. The planner may inspect the
// workspace after crash recovery and returns typed artifact writes/deletes plus
// raw directory operations. Artifact revisions are validated under their
// ordered locks before anything is staged.
//
// This is the activation boundary for migrations that must prove a complete
// source inventory under the same workspace lock that publishes canonical
// replacements and removes now-empty legacy directories. Artifact operations
// are applied before raw operations so callers can delete every file in a
// legacy directory before removing the directory itself.
func (s *CommitService) CommitPlannedOpsWithArtifacts(
	plan func() ([]transaction.Op, []ArtifactChange, error),
) error {
	if plan == nil {
		return fmt.Errorf("storage: planned ops commit requires a planner")
	}

	return s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		ops, artifacts, planErr := plan()
		if planErr != nil {
			return planErr
		}
		if len(ops) == 0 && len(artifacts) == 0 {
			return fmt.Errorf("storage: planned ops commit requires at least one operation")
		}
		lockPaths, lockErr := artifactLockPaths(artifacts, s.roots)
		if lockErr != nil {
			return lockErr
		}
		if err := createArtifactLockDirectories(lockPaths); err != nil {
			return err
		}
		return withOrderedFileLocks(lockPaths, func() error {
			if err := validateArtifactRevisions(artifacts); err != nil {
				return err
			}
			artifactOps, _, expandErr := expandArtifactChanges(artifacts)
			if expandErr != nil {
				return expandErr
			}
			allOps := make([]transaction.Op, 0, len(artifactOps)+len(ops))
			allOps = append(allOps, artifactOps...)
			allOps = append(allOps, ops...)

			manifest, prepareErr := journal.Prepare(allOps)
			if prepareErr != nil {
				return fmt.Errorf("preparing transaction: %w", prepareErr)
			}
			for _, op := range allOps {
				if op.Kind != transaction.OpMove {
					continue
				}
				if err := os.MkdirAll(filepath.Dir(filepath.Clean(op.Target)), DirPermissions); err != nil {
					return fmt.Errorf("creating commit target directory: %w", err)
				}
			}
			if err := journal.Commit(manifest); err != nil {
				return fmt.Errorf("committing transaction: %w", err)
			}
			return nil
		})
	})
}

// CommitOpsTransaction commits raw lifecycle operations and artifacts while
// recording their complete file/directory topology in the workspace undo
// ledger. The ledger append shares the same journal manifest as the user
// mutation, so a successful lifecycle change can never be missing its undo
// boundary.
func (s *CommitService) CommitOpsTransaction(request OpsCommitRequest) (string, error) {
	return s.commitOps(request, true)
}

func (s *CommitService) commitOps(request OpsCommitRequest, recordUndo bool) (string, error) {
	ops := request.Ops
	artifacts := request.Artifacts
	if len(ops) == 0 && len(artifacts) == 0 {
		return "", nil
	}
	lockPaths, err := artifactLockPaths(artifacts, s.roots)
	if err != nil {
		return "", err
	}
	var txnID string
	commitErr := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
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

			var ledgerArtifacts []LedgerArtifact
			if recordUndo && s.ledger != nil {
				var captureErr error
				ledgerArtifacts, captureErr = captureOpsLedgerImages(ops, artifacts)
				if captureErr != nil {
					return fmt.Errorf("capturing undo images for lifecycle transaction: %w", captureErr)
				}
			}
			artifactOps, _, err := expandArtifactChanges(artifacts)
			if err != nil {
				return err
			}
			allOps := make([]transaction.Op, 0, len(ops)+len(artifactOps))
			allOps = append(allOps, ops...)
			allOps = append(allOps, artifactOps...)

			var ledgerRecord *LedgerRecord
			if recordUndo && s.ledger != nil {
				ledgerRecord = &LedgerRecord{
					Action:     request.UndoAction,
					Context:    request.UndoContext,
					ProjectIDs: request.ProjectIDs,
					Artifacts:  ledgerArtifacts,
				}
			}
			manifest, ledgerPlan, err := s.prepareTransaction(journal, allOps, ledgerRecord)
			if err != nil {
				return err
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
			if ledgerPlan != nil {
				s.ledger.gcBlobs(ledgerPlan.file)
			}
			txnID = manifest.ID
			return nil
		})
	})
	if commitErr != nil {
		return "", commitErr
	}
	return txnID, nil
}
