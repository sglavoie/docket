package storage

// WithTaskCreationRestoreGuard installs the domain check for undoing creation.
// path is read under its normal artifact lock alongside the project document;
// check must only read and validate, never write or acquire another lock. The
// original project IDs and created task ID identify the task being removed.
func (l *WorkspaceLedger) WithTaskCreationRestoreGuard(path string, check func(projectIDs []string, taskID string) error) *WorkspaceLedger {
	l.creationGuardPath, l.creationGuard = path, check
	return l
}

func (l *WorkspaceLedger) guardsTaskCreation(entry LedgerEntry) bool {
	return l.creationGuard != nil && len(entry.Artifacts) == 1 && l.isProjectTaskImage(entry.Artifacts[0])
}

func (l *WorkspaceLedger) checkTaskCreationRestore(entry LedgerEntry, blobs map[string][]byte, redo bool) error {
	if redo || !l.guardsTaskCreation(entry) {
		return nil
	}
	image := entry.Artifacts[0]
	if created := standaloneCreatedTaskInImages(blobs[image.BeforeHash], blobs[image.AfterHash]); created != nil {
		return l.creationGuard(entry.ProjectIDs, created.ID)
	}
	return nil
}
