package storage

// WithTaskRemovalRestoreGuard installs the domain check for undoing creation or
// redoing deletion. path is read under its normal artifact lock alongside the
// project document; check must only read and validate, never write or acquire
// another lock. The original project IDs and removed task ID name the target.
func (l *WorkspaceLedger) WithTaskRemovalRestoreGuard(path string, check func(projectIDs []string, taskID string) error) *WorkspaceLedger {
	l.removalGuardPath, l.removalGuard = path, check
	return l
}

func (l *WorkspaceLedger) guardsTaskRemoval(entry LedgerEntry) bool {
	return l.removalGuard != nil && len(entry.Artifacts) == 1 && l.isProjectTaskImage(entry.Artifacts[0])
}

func (l *WorkspaceLedger) checkTaskRemovalRestore(entry LedgerEntry, blobs map[string][]byte, redo bool) error {
	if !l.guardsTaskRemoval(entry) {
		return nil
	}
	image := entry.Artifacts[0]
	expected, desired := blobs[image.AfterHash], blobs[image.BeforeHash]
	if redo {
		expected, desired = desired, expected
	}
	if removed := standaloneAddedTaskInImages(desired, expected); removed != nil {
		return l.removalGuard(entry.ProjectIDs, removed.ID)
	}
	return nil
}
