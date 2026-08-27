package app

import "fmt"

// mutateRichRecordSelection gives checklist and comment CRUD the same in-memory
// atomicity as their storage commit. Repository snapshots can
// retain caller-visible pointers, so a failed commit restores the complete
// pre-mutation graph.
func (s *Service) mutateRichRecordSelection(locator TaskLocator, undoAction string, mutate func(TaskRef) error) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return TaskResult{}, fmt.Errorf("capture rich-record rollback state: %w", err)
	}
	keepMutation := false
	defer func() {
		if !keepMutation {
			restoreSnapshotProjects(snap, before)
		}
	}()
	if mutateErr := mutate(ref); mutateErr != nil {
		return TaskResult{}, mutateErr
	}
	result, err := s.commitProject(ref, revision, undoAction, "")
	if err != nil {
		return TaskResult{}, err
	}
	keepMutation = true
	return result, nil
}
