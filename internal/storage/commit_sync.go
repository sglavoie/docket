package storage

import (
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/transaction"
)

// commitJournal is the one place a commit path hands a prepared manifest to
// its journal. It is a variable so crash-injection tests can stop a commit
// after preparation, exactly where a killed process would leave it.
var commitJournal = func(journal *transaction.Journal, manifest *transaction.Manifest) error {
	return journal.Commit(manifest)
}

// WithSyncHistory attaches the device's durable sync history and returns the
// same service. Every transaction the service commits from then on — project
// changesets, artifact-only and lifecycle transactions, maintenance, and the
// attached ledger's undo and redo — stages its outgoing sync record in its own
// journal manifest, so recovery never lands a commit without its record or a
// record without its commit.
func (s *CommitService) WithSyncHistory(history *synclog.History) *CommitService {
	s.history = history
	if s.ledger != nil {
		s.ledger.history = history
	}
	return s
}

// SyncHistory is the attached sync history, or nil.
func (s *CommitService) SyncHistory() *synclog.History { return s.history }

// WithSyncMerger attaches the reconciliation of concurrent artifacts and
// returns the same service. With a merger, CommitIncoming reconciles every
// incoming transaction with the device's whole history (see
// synclog.History.PlanReconcile) rather than applying fast-forwards alone.
func (s *CommitService) WithSyncMerger(merger synclog.Merger) *CommitService {
	s.merger = merger
	return s
}

// CommitIncoming applies another device's sync transaction and records its
// receipt as one journal transaction, under the workspace commit lock and the
// advisory locks of every path it touches, after crash recovery. Recovery
// therefore lands the applied bytes and the acknowledgement together: a crash
// can never acknowledge an unapplied transaction or apply one twice. A
// manifest that is held, incomplete, or already received is refused before
// anything is staged; see synclog.PlanIncoming. Without a merger a manifest
// concurrent with local changes is refused too; with one it is reconciled and
// the conflicts it leaves are recorded beside the history. It returns the
// journal transaction id.
func (s *CommitService) CommitIncoming(manifest *synclog.Manifest, blobs synclog.BlobSource) (string, error) {
	if s.history == nil {
		return "", fmt.Errorf("storage: incoming sync requires a sync history")
	}
	if manifest == nil {
		return "", fmt.Errorf("%w: no manifest", synclog.ErrInvalidManifest)
	}
	if err := manifest.Validate(); err != nil {
		return "", err
	}
	if s.merger != nil {
		return s.commitReconciled(manifest, blobs)
	}
	locked := make([]ArtifactChange, 0, len(manifest.Effects)*2)
	for _, path := range manifest.Paths() {
		locked = append(locked, ArtifactChange{Path: s.history.WorkspacePath(path)})
	}
	lockPaths, err := artifactLockPaths(locked, s.roots)
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
			id := uuid.NewString()
			plan, err := s.history.PlanIncoming(manifest, blobs, id)
			if err != nil {
				return err
			}
			prepared, err := journal.PrepareWithID(id, plan.Ops)
			if err != nil {
				return fmt.Errorf("preparing incoming transaction: %w", err)
			}
			for _, dir := range plan.MoveTargets {
				if err := os.MkdirAll(dir, DirPermissions); err != nil {
					return fmt.Errorf("creating commit target directory: %w", err)
				}
			}
			if err := commitJournal(journal, prepared); err != nil {
				return fmt.Errorf("committing incoming transaction: %w", err)
			}
			txnID = prepared.ID
			return nil
		})
	})
	if commitErr != nil {
		return "", commitErr
	}
	return txnID, nil
}

// commitReconciled is CommitIncoming with a merger. The paths a
// reconciliation writes are known only once it is planned, so it plans under
// the workspace commit lock, then takes the advisory locks of exactly those
// paths and confirms each still holds what the plan read before committing.
func (s *CommitService) commitReconciled(manifest *synclog.Manifest, blobs synclog.BlobSource) (string, error) {
	var txnID string
	commitErr := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}
		id := uuid.NewString()
		plan, err := s.history.PlanReconcile(manifest, blobs, s.merger, id)
		if err != nil {
			return err
		}
		locked := make([]ArtifactChange, 0, len(plan.Expect))
		for path := range plan.Expect {
			locked = append(locked, ArtifactChange{Path: path})
		}
		lockPaths, err := artifactLockPaths(locked, s.roots)
		if err != nil {
			return err
		}
		if err := createArtifactLockDirectories(lockPaths); err != nil {
			return err
		}
		return withOrderedFileLocks(lockPaths, func() error {
			for path, want := range plan.Expect {
				if err := expectSyncedBytes(path, want); err != nil {
					return err
				}
			}
			prepared, err := journal.PrepareWithID(id, plan.Ops)
			if err != nil {
				return fmt.Errorf("preparing incoming transaction: %w", err)
			}
			for _, dir := range plan.MoveTargets {
				if err := os.MkdirAll(dir, DirPermissions); err != nil {
					return fmt.Errorf("creating commit target directory: %w", err)
				}
			}
			if err := commitJournal(journal, prepared); err != nil {
				return fmt.Errorf("committing incoming transaction: %w", err)
			}
			txnID = prepared.ID
			return nil
		})
	})
	if commitErr != nil {
		return "", commitErr
	}
	return txnID, nil
}

func expectSyncedBytes(path string, want *string) error {
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err) && want == nil:
		return nil
	case err != nil && !os.IsNotExist(err):
		return fmt.Errorf("reading %s: %w", path, err)
	case want == nil || err != nil || synclog.Digest(data) != *want:
		return fmt.Errorf("%w: %s changed while reconciling", synclog.ErrUnrecordedChange, path)
	}
	return nil
}

// SyncConflicts derives the sync conflicts the device's history leaves open,
// identical on every device holding the same transactions.
func (s *CommitService) SyncConflicts() ([]synclog.Conflict, error) {
	if s.history == nil {
		return nil, fmt.Errorf("storage: sync conflicts require a sync history")
	}
	return s.history.OpenConflicts(s.merger)
}

// CommitSyncResolution resolves an open sync conflict by writing data to its
// path, or deleting the path when data is nil, as an undoable transaction of
// kind resolution. The resolution descends from every competing write, so it
// supersedes them on every device that receives it; choosing the bytes the
// workspace already holds still records the choice.
func (s *CommitService) CommitSyncResolution(conflictID string, data []byte) (string, error) {
	conflicts, err := s.SyncConflicts()
	if err != nil {
		return "", err
	}
	for _, conflict := range conflicts {
		if conflict.ID != conflictID {
			continue
		}
		change := ArtifactChange{Path: s.history.WorkspacePath(conflict.Path), Data: data, Mode: FilePermissions, Delete: data == nil}
		return s.commitArtifacts(ArtifactCommitRequest{
			Artifacts:   []ArtifactChange{change},
			UndoAction:  "Resolve sync conflict",
			UndoContext: conflict.Path,
		}, true, synclog.KindResolution)
	}
	return "", fmt.Errorf("storage: no open sync conflict %s", conflictID)
}

// CommitSyncPublished records that the outgoing transactions txnIDs reached
// the exchange, dropping them from the outbox through the journal under the
// workspace commit lock. Confirming what is no longer pending changes nothing.
func (s *CommitService) CommitSyncPublished(txnIDs []string) error {
	if s.history == nil {
		return fmt.Errorf("storage: publication tracking requires a sync history")
	}
	return s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}
		plan, err := s.history.PlanPublished(txnIDs)
		if err != nil {
			return err
		}
		if len(plan.Ops) == 0 {
			return nil
		}
		prepared, err := journal.Prepare(plan.Ops)
		if err != nil {
			return fmt.Errorf("preparing publication record: %w", err)
		}
		return commitJournal(journal, prepared)
	})
}
