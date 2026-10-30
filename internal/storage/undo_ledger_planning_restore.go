package storage

import (
	"path/filepath"

	"github.com/sglavoie/docket/internal/transaction"
)

// WithPlanningRestore installs the planning domain's pure restore function for
// path. It may return replacement bytes only when the recorded change can be
// applied without overwriting external work. It must not read, write, or lock
// files: preflight supplies all three images under the existing artifact lock.
func (l *WorkspaceLedger) WithPlanningRestore(path string, restore func(expected, desired, current []byte) ([]byte, bool)) *WorkspaceLedger {
	l.planningRestorePath, l.planningRestore = filepath.Clean(path), restore
	return l
}

// WithPlanningRestoreGuard installs the engine's membership eligibility check.
// prepare runs under the workspace lock and returns additional paths to lock
// and a read-only validator of the complete proposed restore. The validator
// runs under those locks before staging, including for exact-image restores.
func (l *WorkspaceLedger) WithPlanningRestoreGuard(prepare func(expected, desired []byte) ([]string, func([]transaction.Op) error, error)) *WorkspaceLedger {
	l.planningGuard = prepare
	return l
}

func (l *WorkspaceLedger) planningRestoreEntry(entry LedgerEntry) bool {
	for _, image := range entry.Artifacts {
		if filepath.Clean(image.Path) == l.planningRestorePath && image.imageKind() == ArtifactImageKindFile && image.BeforeExists && image.AfterExists {
			continue
		}
		if !l.isProjectTaskImage(image) {
			return false
		}
	}
	return true
}

func (l *WorkspaceLedger) preparePlanningRestoreGuard(entry LedgerEntry, redo bool) ([]string, func([]transaction.Op) error, error) {
	if l.planningGuard == nil || !l.planningRestoreEntry(entry) {
		return nil, nil, nil
	}
	for _, image := range entry.Artifacts {
		if filepath.Clean(image.Path) != l.planningRestorePath {
			continue
		}
		blobs, err := l.validateRestoreImages(entry)
		if err != nil {
			return nil, nil, err
		}
		expected, desired := blobs[image.AfterHash], blobs[image.BeforeHash]
		if redo {
			expected, desired = desired, expected
		}
		return l.planningGuard(expected, desired)
	}
	return nil, nil, nil
}

func (l *WorkspaceLedger) artifactRestore(entry LedgerEntry, image ArtifactImage, lf *ledgerFile, redo bool) func([]byte, []byte, []byte) ([]byte, bool) {
	if driftExplainedByLedger(lf, entry, image.Path, redo) {
		return nil
	}
	if l.planningRestore != nil && filepath.Clean(image.Path) == l.planningRestorePath && l.planningRestoreEntry(entry) {
		return l.planningRestore
	}
	if l.isProjectTaskImage(image) {
		return func(expected, desired, current []byte) ([]byte, bool) {
			return rebaseTaskRestore(expected, desired, current, len(entry.Artifacts) == 1)
		}
	}
	return nil
}
