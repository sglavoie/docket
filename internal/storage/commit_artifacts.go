package storage

import (
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/transaction"
)

// CommitArtifactTransaction commits an artifact-only user mutation as one
// recoverable transaction and, when the service has a workspace ledger, records
// its byte-exact before/after images as one undo entry. It deliberately does not
// rewrite any project tasks.md file. ProjectIDs control ledger selection only: an
// empty set makes the entry global-only, while any listed project can select and
// reverse the entire grouped transaction through ProjectScope.
func (s *CommitService) CommitArtifactTransaction(request ArtifactCommitRequest) (string, error) {
	return s.commitArtifacts(request, true, synclog.KindUser)
}

// CommitArtifacts commits configuration-domain artifacts with no accompanying
// project tasks.md as one atomic journaled transaction. It remains the legacy
// config-only wrapper: although it shares the artifact transaction machinery, it
// intentionally records no workspace-ledger entry so existing configuration
// saves do not become user-visible undo steps.
func (s *CommitService) CommitArtifacts(artifacts []ArtifactChange) (string, error) {
	return s.commitArtifacts(ArtifactCommitRequest{Artifacts: artifacts}, false, synclog.KindUser)
}

// CommitPlannedArtifacts builds and commits an artifact-only maintenance
// transaction while holding the workspace commit lock. The planner must be
// read-only: it exists for operations such as immutable workspace snapshots
// that must read one coherent source state after crash recovery and before any
// cooperating writer can commit. The returned artifacts are validated,
// revision-checked, staged, and published together through the normal journal;
// no workspace-ledger entry is recorded.
//
// Unlike CommitArtifacts, artifact paths cannot be validated before the lock is
// taken because the planner supplies them. Callers should therefore validate
// user-controlled roots before invoking this method and return only paths
// derived beneath the service's approved roots.
func (s *CommitService) CommitPlannedArtifacts(plan func() ([]ArtifactChange, error)) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("storage: planned artifact commit requires a planner")
	}

	var txnID string
	commitErr := s.withWorkspaceCommitLock(func() error {
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		artifacts, planErr := plan()
		if planErr != nil {
			return planErr
		}
		if len(artifacts) == 0 {
			return fmt.Errorf("storage: planned artifact commit requires at least one artifact")
		}
		artifactLocks, lockErr := artifactLockPaths(artifacts, s.roots)
		if lockErr != nil {
			return lockErr
		}
		if err := createArtifactLockDirectories(artifactLocks); err != nil {
			return err
		}
		return withOrderedFileLocks(artifactLocks, func() error {
			if err := validateArtifactRevisions(artifacts); err != nil {
				return err
			}
			ops, dirsToCreate, expandErr := expandArtifactChanges(artifacts)
			if expandErr != nil {
				return expandErr
			}
			manifest, _, prepareErr := s.prepareTransaction(journal, ops, nil, synclog.KindRepair)
			if prepareErr != nil {
				return prepareErr
			}
			for _, dir := range dirsToCreate {
				if err := os.MkdirAll(dir, DirPermissions); err != nil {
					return fmt.Errorf("creating commit target directory: %w", err)
				}
			}
			if err := s.commitJournal(journal, manifest); err != nil {
				return fmt.Errorf("committing transaction: %w", err)
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
	content, err := doc.WriteCurrent()
	if err != nil {
		return "", fmt.Errorf("rendering project repair for %s: %w", project.Name, err)
	}
	return s.commitArtifacts(ArtifactCommitRequest{Artifacts: []ArtifactChange{{
		Path:             tasksFilePath(project.Path),
		Data:             []byte(content),
		Mode:             FilePermissions,
		ExpectedRevision: &expectedRawRevision,
	}}}, false, synclog.KindRepair)
}

// prepareTransaction stages one journal manifest and, when record is non-nil and
// an enabled workspace ledger is configured, includes the complete ledger append
// in that same manifest. When the service carries a sync history, the outgoing
// record of ops — its blobs, immutable manifest, index entry, and tracking
// state — is staged in that manifest too, so the commit and its publication
// record recover together. Optional capture receipts share the manifest but
// stay outside sync and undo. The transaction ID is allocated before planning so every
// record cites the journal transaction that carries it. User operations stay
// first, sync history and capture receipts follow, and the ledger's blobs and ledger.json come
// last, with ledger.json last by prepareRecord's contract. A planning failure
// returns before Journal.Prepare stages any operation.
func (s *CommitService) prepareTransaction(
	journal *transaction.Journal,
	ops []transaction.Op,
	record *LedgerRecord,
	kind synclog.Kind,
	captures ...*CaptureReceipt,
) (*transaction.Manifest, *preparedLedgerRecord, error) {
	var capture *CaptureReceipt
	if len(captures) > 0 {
		capture = captures[0]
	}
	recordLedger := record != nil && s.ledger != nil
	if !recordLedger && s.history == nil && capture == nil {
		manifest, err := journal.Prepare(ops)
		if err != nil {
			return nil, nil, fmt.Errorf("preparing transaction: %w", err)
		}
		return manifest, nil, nil
	}

	txnID := uuid.NewString()
	combinedOps := append([]transaction.Op{}, ops...)
	if s.history != nil {
		syncPlan, err := s.history.PlanLocal(kind, txnID, ops)
		if err != nil {
			return nil, nil, fmt.Errorf("preparing sync history: %w", err)
		}
		combinedOps = append(combinedOps, syncPlan.Ops...)
	}
	if capture != nil {
		op, err := s.captureReceiptOp(capture, txnID)
		if err != nil {
			return nil, nil, err
		}
		combinedOps = append(combinedOps, op)
	}
	var ledgerPlan *preparedLedgerRecord
	if recordLedger {
		ledgerRecord := *record
		ledgerRecord.TransactionID = txnID
		var err error
		ledgerPlan, err = s.ledger.prepareRecord(ledgerRecord)
		if err != nil {
			return nil, nil, fmt.Errorf("preparing undo ledger entry: %w", err)
		}
		if ledgerPlan != nil {
			combinedOps = append(combinedOps, ledgerPlan.ops...)
		}
	}
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
func (s *CommitService) commitArtifacts(request ArtifactCommitRequest, recordUndo bool, kind synclog.Kind) (string, error) {
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
		journal := s.journal()
		if _, recoverErr := s.recoverLocked(journal); recoverErr != nil {
			return recoverErr
		}

		// Preconditions guard the validity of artifact locations themselves. Take
		// and validate those locks first so a stale project path is rejected before
		// creating a note lock directory below a location that was just moved. The
		// workspace commit lock serializes other CommitServices while
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
				manifest, ledgerPlan, prepareErr := s.prepareTransaction(journal, ops, ledgerRecord, kind)
				if prepareErr != nil {
					return prepareErr
				}
				for _, dir := range dirsToCreate {
					if err := os.MkdirAll(dir, DirPermissions); err != nil {
						return fmt.Errorf("creating commit target directory: %w", err)
					}
				}
				if err := s.commitJournal(journal, manifest); err != nil {
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
