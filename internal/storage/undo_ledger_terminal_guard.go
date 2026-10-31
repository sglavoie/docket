package storage

import (
	"path/filepath"

	"github.com/sglavoie/docket/internal/transaction"
)

// WithTerminalTaskRestoreGuard checks planning before a restore makes a task
// terminal. The planning path is locked beside the project images. check reads
// current planning when planning is nil, otherwise validates the proposed bytes;
// it must not write or acquire locks. This also covers exact-image restores and
// status entries that originally changed no planning memberships.
func (l *WorkspaceLedger) WithTerminalTaskRestoreGuard(path string, check func(projectIDs, taskIDs []string, planning []byte) error) *WorkspaceLedger {
	l.terminalGuardPath, l.terminalGuard = filepath.Clean(path), check
	return l
}

func (l *WorkspaceLedger) guardsTerminalTaskRestore(entry LedgerEntry) bool {
	if l.terminalGuard != nil {
		for _, image := range entry.Artifacts {
			if l.isProjectTaskImage(image) {
				return true
			}
		}
	}
	return false
}

func (l *WorkspaceLedger) checkTerminalTaskRestore(entry LedgerEntry, blobs map[string][]byte, redo bool, ops []transaction.Op) error {
	if !l.guardsTerminalTaskRestore(entry) {
		return nil
	}
	var taskIDs []string
	for _, image := range entry.Artifacts {
		if !l.isProjectTaskImage(image) {
			continue
		}
		expectedBytes, desiredBytes := blobs[image.AfterHash], blobs[image.BeforeHash]
		if redo {
			expectedBytes, desiredBytes = desiredBytes, expectedBytes
		}
		expected, expectedErr := parseProjectDocumentContent(string(expectedBytes))
		desired, desiredErr := parseProjectDocumentContent(string(desiredBytes))
		if expectedErr != nil || desiredErr != nil {
			continue
		}
		before := indexTasks(expected.Tasks)
		for _, task := range desired.Tasks {
			if prior := before[task.ID]; prior != nil && task.IsComplete() &&
				(prior.WorkflowState != task.WorkflowState || !prior.StateChangedAt.Equal(task.StateChangedAt) || !sameOptionalInstant(prior.CompletedAt, task.CompletedAt)) {
				taskIDs = append(taskIDs, task.ID)
			}
		}
	}
	if len(taskIDs) == 0 {
		return nil
	}
	var planning []byte
	for _, op := range ops {
		if filepath.Clean(op.Target) == l.terminalGuardPath {
			if op.Kind != transaction.OpWrite {
				return &ArtifactConflictError{Path: op.Target, Expected: "planning present", Actual: "planning removed"}
			}
			planning = op.Data
		}
	}
	return l.terminalGuard(entry.ProjectIDs, taskIDs, planning)
}
