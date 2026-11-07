package storage

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/synclog"
)

// CommitSyncCheckpoint records this device's checkpoint of its whole history
// (see synclog.History.PlanCheckpoint) under the workspace commit lock, after
// crash recovery and after importing every external change, so the image is
// the workspace as it stands and the history that produced it. A history with
// open sync conflicts is refused: a checkpoint carries one version of each
// file, and a joiner starting from it would never learn the others competed.
func (s *CommitService) CommitSyncCheckpoint() (*synclog.Checkpoint, error) {
	if s.history == nil {
		return nil, fmt.Errorf("storage: a sync checkpoint requires a sync history")
	}
	var checkpoint *synclog.Checkpoint
	err := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}
		external, observeErr := s.commitExternalLocked(journal, true)
		if observeErr != nil {
			return observeErr
		}
		if len(external.Held) > 0 {
			return fmt.Errorf("%w: %s holds a change Docket cannot read (%s)", synclog.ErrCheckpointRefused, external.Held[0].Path, external.Held[0].Reason)
		}
		if s.merger != nil {
			conflicts, conflictErr := s.history.WorkspaceConflicts(s.merger)
			if conflictErr != nil {
				return conflictErr
			}
			if len(conflicts) > 0 {
				return fmt.Errorf("%w: %d sync conflict(s) are open; resolve them first", synclog.ErrCheckpointRefused, len(conflicts))
			}
		}
		plan, c, planErr := s.history.PlanCheckpoint()
		if planErr != nil {
			return planErr
		}
		prepared, prepareErr := journal.Prepare(plan.Ops)
		if prepareErr != nil {
			return fmt.Errorf("preparing sync checkpoint: %w", prepareErr)
		}
		if commitErr := s.commitJournal(journal, prepared); commitErr != nil {
			return fmt.Errorf("committing sync checkpoint: %w", commitErr)
		}
		checkpoint = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return checkpoint, nil
}

// InstallCheckpoint builds an empty, history-less working copy from c in one
// checked journal transaction (see synclog.History.PlanCheckpointInstall). It
// reports false, changing nothing, where a checkpoint has no place — a working
// copy that holds files or history already — so the caller imports item by
// item instead. Once prepared, a failure is left to journal recovery.
func (s *CommitService) InstallCheckpoint(c *synclog.Checkpoint, blobs synclog.BlobSource) (string, bool, error) {
	if s.history == nil {
		return "", false, nil
	}
	var txnID string
	accepted := false
	err := s.withWorkspaceCommitLock(func() error {
		journal := s.journal().WithBatchedWrites()
		if _, err := s.recoverLocked(journal); err != nil {
			return err
		}
		validator, _ := s.merger.(synclog.Validator)
		id := uuid.NewString()
		plan, err := s.history.PlanCheckpointInstall(c, blobs, validator, id)
		if err != nil || plan == nil {
			return err
		}
		accepted = true
		if err := s.commitReconcilePlan(journal, id, plan); err != nil {
			return err
		}
		txnID = id
		return nil
	})
	return txnID, accepted, err
}
