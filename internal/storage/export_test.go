package storage

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sglavoie/tt/internal/transaction"
)

var (
	crashTargetsMu sync.Mutex
	// crashTargets are the transactions directories whose next journal commit
	// stops, keyed by directory, each receiving the prepared manifest.
	crashTargets = map[string]*transaction.Manifest{}
	armedTargets = map[string]bool{}
)

// commitJournal dispatches to a crash only for an armed transactions
// directory, so the external sync fault harness can stop one replica's commit
// while every other replica and test commits normally.
func init() {
	next := commitJournal
	commitJournal = func(journal *transaction.Journal, manifest *transaction.Manifest) error {
		dir := filepath.Dir(journal.ManifestPath(manifest.ID))
		crashTargetsMu.Lock()
		armed := armedTargets[dir]
		if armed {
			delete(armedTargets, dir)
			crashTargets[dir] = manifest
		}
		crashTargetsMu.Unlock()
		if armed {
			return errInjectedCrash
		}
		return next(journal, manifest)
	}
}

// CrashCommitForTest runs fn, stopping the first journal commit under
// transactionsDir as a process killed after some prefix of its operations
// would leave it (see simulateCrashAfter). choose receives the stopped
// transaction's operation count and returns that prefix, from -1 (the
// manifest never became durable) to the count (every operation applied, the
// manifest not yet reclaimed). It reports the prefix and whether fn reached a
// commit at all; fn's error is returned when it did not, so a refusal before
// any commit is the caller's to judge.
func CrashCommitForTest(t *testing.T, transactionsDir string, choose func(ops int) int, fn func() error) (prefix, ops int, crashed bool, err error) {
	t.Helper()
	dir := filepath.Clean(transactionsDir)
	crashTargetsMu.Lock()
	armedTargets[dir] = true
	crashTargetsMu.Unlock()
	err = fn()
	crashTargetsMu.Lock()
	manifest := crashTargets[dir]
	delete(crashTargets, dir)
	delete(armedTargets, dir)
	crashTargetsMu.Unlock()
	if manifest == nil {
		return 0, 0, false, err
	}
	if !errors.Is(err, errInjectedCrash) {
		t.Fatalf("stopped commit returned %v, want the injected crash", err)
	}
	prefix = min(max(choose(len(manifest.Ops)), -1), len(manifest.Ops))
	simulateCrashAfter(t, dir, manifest, prefix)
	return prefix, len(manifest.Ops), true, nil
}
