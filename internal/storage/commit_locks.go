package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/transaction"
)

// WorkspaceCommitLockName is the single workspace-level lock file that serializes
// the entire commit pipeline for a workspace.
const WorkspaceCommitLockName = "commit.lock"

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
			identity: artifactPathIdentity(filepath.Join(service.transactionsDir, WorkspaceCommitLockName)),
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
				journal := service.journal()
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
	return s.checkRecovery(journal.RecoverPending())
}

func (s *CommitService) checkRecovery(report *transaction.RecoveryReport, err error) (*transaction.RecoveryReport, error) {
	if err != nil {
		return report, fmt.Errorf("transaction recovery failed: %w", err)
	}
	// A transaction another machine is still driving is skipped rather than
	// recovered, which means this pass deliberately left durable state unfinished.
	// That is correct and it is not an error, but it is the one recovery outcome a
	// user cannot see anywhere else: their edit exists on the other machine and has
	// not landed here yet. Log it so the reason is on record when they ask.
	for _, foreign := range report.ForeignLive {
		logging.Warn(
			"left transaction %s (%s) alone: owned by machine %s, heartbeat %s, manifest %s",
			foreign.ID, foreign.State, foreign.Machine, foreign.Heartbeat, foreign.Path,
		)
	}
	if report.Blocked() {
		return report, fmt.Errorf("%w: %w", ErrCommitBlocked, report.Err())
	}
	return report, nil
}
