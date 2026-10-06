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

// CommitIncoming applies another device's sync transaction and records its
// receipt as one journal transaction, under the workspace commit lock and the
// advisory locks of every path it touches, after crash recovery. Recovery
// therefore lands the applied bytes and the acknowledgement together: a crash
// can never acknowledge an unapplied transaction or apply one twice. A
// manifest that is held, incomplete, already received, or concurrent with local
// changes is refused before anything is staged; see synclog.PlanIncoming. It
// returns the journal transaction id.
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
