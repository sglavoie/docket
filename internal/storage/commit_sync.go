package storage

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

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
// advisory locks of every path it touches, after crash recovery and after
// importing every external change (see CommitExternalChanges), so a change
// made outside a commit is published rather than reconciled away. Recovery
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
		if _, observeErr := s.commitExternalLocked(journal, false); observeErr != nil {
			return observeErr
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
			if err := s.commitJournal(journal, prepared); err != nil {
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
		if _, observeErr := s.commitExternalLocked(journal, false); observeErr != nil {
			return observeErr
		}
		id := uuid.NewString()
		plan, err := s.history.PlanReconcile(manifest, blobs, s.merger, id)
		if err != nil {
			return err
		}
		if err := s.commitReconcilePlan(journal, id, plan); err != nil {
			return err
		}
		txnID = id
		return nil
	})
	if commitErr != nil {
		return "", commitErr
	}
	return txnID, nil
}

// BootstrapIncoming attempts one initial import into an empty working copy.
// A declined plan changes nothing and leaves ordinary per-item import to
// report missing history, incompatible records or existing workspace edits.
// Once prepared, any failure is returned for journal recovery, never retried
// as independent commits over a partially applied bootstrap.
func (s *CommitService) BootstrapIncoming(manifests []*synclog.Manifest, blobs synclog.BlobSource) (string, bool, error) {
	if s.history == nil || s.merger == nil {
		return "", false, nil
	}
	var txnID string
	accepted := false
	err := s.withWorkspaceCommitLock(func() error {
		journal := s.journal().WithBatchedWrites()
		if _, err := s.recoverLocked(journal); err != nil {
			return err
		}
		id := uuid.NewString()
		plan, err := s.history.PlanBootstrap(manifests, blobs, s.merger, id)
		if err != nil || plan == nil {
			return nil //nolint:nilerr // A declined bootstrap falls back to per-item import, which reports the history itself.
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

func (s *CommitService) commitReconcilePlan(journal *transaction.Journal, id string, plan *synclog.ReconcilePlan) error {
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
		if err := s.commitJournal(journal, prepared); err != nil {
			return fmt.Errorf("committing incoming transaction: %w", err)
		}
		return nil
	})
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

// SyncConflicts derives open conflicts and binds each to its current on-disk
// content. IDs agree on devices with the same history and working files, and
// change when an intervening edit changes the resolution's write precondition.
func (s *CommitService) SyncConflicts() ([]synclog.Conflict, error) {
	if s.history == nil {
		return nil, fmt.Errorf("storage: sync conflicts require a sync history")
	}
	return s.history.WorkspaceConflicts(s.merger)
}

// CommitSyncResolution resolves an open sync conflict by writing data to its
// path, or deleting the path when data is nil, as an undoable transaction of
// kind resolution. The resolution descends from every competing write, so it
// supersedes them on every device that receives it; choosing the bytes the
// workspace already holds still records the choice.
func (s *CommitService) CommitSyncResolution(conflictID string, data []byte) (string, error) {
	return s.CommitSyncResolutions([]SyncResolution{{ConflictID: conflictID, Data: data}})
}

// SyncResolution is the bytes one open sync conflict's path is resolved to; a
// nil Data deletes the path.
type SyncResolution struct {
	ConflictID string
	Data       []byte
}

// ErrSyncConflictClosed marks a resolution naming a conflict the history no
// longer leaves open: another resolution closed it, or an edit since it was
// read changed what it holds, which gives it a new id.
var ErrSyncConflictClosed = errors.New("storage: sync conflict is no longer open")

// CommitSyncResolutions resolves several open sync conflicts as one undoable
// transaction of kind resolution, so changes one transaction coupled across
// files are resolved together or not at all. Each path must still hold the
// bytes its conflict reported when the commit locks it; anything that moved
// in between refuses the whole resolution with an *ArtifactConflictError, so
// a racing edit is never written over.
func (s *CommitService) CommitSyncResolutions(resolutions []SyncResolution) (string, error) {
	if len(resolutions) == 0 {
		return "", fmt.Errorf("storage: a sync resolution names at least one conflict")
	}
	conflicts, err := s.SyncConflicts()
	if err != nil {
		return "", err
	}
	open := make(map[string]synclog.Conflict, len(conflicts))
	for _, conflict := range conflicts {
		open[conflict.ID] = conflict
	}
	changes := make([]ArtifactChange, 0, len(resolutions))
	paths := make([]string, 0, len(resolutions))
	seen := map[string]bool{}
	for _, resolution := range resolutions {
		conflict, ok := open[resolution.ConflictID]
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrSyncConflictClosed, resolution.ConflictID)
		}
		if seen[conflict.Path] {
			return "", fmt.Errorf("storage: a sync resolution names %s twice", conflict.Path)
		}
		seen[conflict.Path] = true
		expected := ""
		if conflict.ResultBlob != nil {
			expected = strings.TrimPrefix(*conflict.ResultBlob, "sha256:")
		}
		changes = append(changes, ArtifactChange{
			Path:             s.history.WorkspacePath(conflict.Path),
			Data:             resolution.Data,
			Mode:             FilePermissions,
			Delete:           resolution.Data == nil,
			ExpectedRevision: &expected,
		})
		paths = append(paths, conflict.Path)
	}
	sort.Strings(paths)
	return s.commitArtifacts(ArtifactCommitRequest{
		Artifacts:   changes,
		UndoAction:  "Resolve sync conflict",
		UndoContext: strings.Join(paths, ", "),
	}, true, synclog.KindResolution)
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
		return s.commitJournal(journal, prepared)
	})
}

// ExternalChanges reports one observation of the workspace against the sync
// history's last indexed revision, in the shape a status surface serializes.
type ExternalChanges struct {
	// TransactionID is the journal transaction that recorded the observation,
	// or empty when nothing was recorded.
	TransactionID string `json:"transaction_id,omitempty"`
	// Baselined reports that the device indexed its workspace for the first
	// time; nothing was imported.
	Baselined bool `json:"baselined,omitempty"`
	// Imported is the outgoing transaction of kind external that publishes the
	// valid changes, or nil when there were none.
	Imported *synclog.Manifest `json:"imported,omitempty"`
	// Held lists the invalid changes left on disk, unpublished, for repair.
	Held []synclog.HeldChange `json:"held,omitempty"`
}

// CommitExternalChanges imports every change made to the synced workspace
// outside a commit — an external editor, a script, or a checked domain save —
// as one outgoing transaction of kind external, under the workspace commit
// lock after crash recovery. A change the live readers would refuse is held:
// its bytes stay on disk untouched and unpublished, and reconciliation will
// not write over them until a repaired version is observed. The first
// observation on a device only indexes the workspace it starts from.
func (s *CommitService) CommitExternalChanges() (*ExternalChanges, error) {
	if s.history == nil {
		return nil, fmt.Errorf("storage: external-change observation requires a sync history")
	}
	var report *ExternalChanges
	err := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}
		var observeErr error
		report, observeErr = s.commitExternalLocked(journal, true)
		return observeErr
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

// commitExternalLocked is CommitExternalChanges under a held workspace commit
// lock. The observation is its own journal transaction: it writes nothing but
// the history, so it needs no artifact lock, and a commit racing it outside
// the lock is caught by the next observation or by reconciliation's own
// recheck.
//
// index lets a device that has never indexed its workspace do so now. An
// incoming apply passes false: its own plan indexes the workspace in the same
// journal transaction as the apply, so a refused manifest still leaves the
// history untouched.
func (s *CommitService) commitExternalLocked(journal *transaction.Journal, index bool) (*ExternalChanges, error) {
	if s.history == nil {
		return &ExternalChanges{}, nil
	}
	validator, _ := s.merger.(synclog.Validator)
	id := uuid.NewString()
	plan, err := s.history.PlanObserve(validator, id, index)
	if err != nil {
		return nil, fmt.Errorf("observing external changes: %w", err)
	}
	report := &ExternalChanges{Baselined: plan.Baselined, Imported: plan.Manifest, Held: plan.Held}
	if len(plan.Ops) == 0 {
		return report, nil
	}
	prepared, err := journal.PrepareWithID(id, plan.Ops)
	if err != nil {
		return nil, fmt.Errorf("preparing external-change record: %w", err)
	}
	if err := s.commitJournal(journal, prepared); err != nil {
		return nil, fmt.Errorf("committing external-change record: %w", err)
	}
	report.TransactionID = prepared.ID
	return report, nil
}

// CommitSyncGenesis records the first transaction of a workspace enrolled in
// sync, on the device that enrolled it: one outgoing transaction of kind
// enroll that writes every synced file as it stands, so a device joining with
// nothing can rebuild the workspace from the published history alone. It runs
// under the workspace commit lock after crash recovery and writes only the
// history. A device that has already recorded anything, or a workspace holding
// a file the live readers would refuse, is refused whole
// (synclog.ErrGenesisRefused) and nothing is written.
func (s *CommitService) CommitSyncGenesis() (*synclog.Manifest, error) {
	if s.history == nil {
		return nil, fmt.Errorf("storage: an enrollment genesis requires a sync history")
	}
	var genesis *synclog.Manifest
	err := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}
		validator, _ := s.merger.(synclog.Validator)
		id := uuid.NewString()
		plan, err := s.history.PlanGenesis(validator, id)
		if err != nil {
			return err
		}
		prepared, err := journal.PrepareWithID(id, plan.Ops)
		if err != nil {
			return fmt.Errorf("preparing enrollment genesis: %w", err)
		}
		if err := s.commitJournal(journal, prepared); err != nil {
			return fmt.Errorf("committing enrollment genesis: %w", err)
		}
		genesis = plan.Manifest
		return nil
	})
	if err != nil {
		return nil, err
	}
	return genesis, nil
}
