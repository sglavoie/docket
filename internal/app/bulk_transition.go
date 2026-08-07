package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// BulkTransition moves workspace-unique task IDs to target in one transaction.
func (s *Service) BulkTransition(taskIDs []string, target models.WorkflowState) (BulkResult, error) {
	return s.bulkTransition(taskIDs, nil, target)
}

// BulkTransitionByLocators moves project-qualified tasks to target in one
// transaction.
func (s *Service) BulkTransitionByLocators(locators []TaskLocator, target models.WorkflowState) (BulkResult, error) {
	return s.bulkTransition(nil, locators, target)
}

func (s *Service) bulkTransition(taskIDs []string, locators []TaskLocator, target models.WorkflowState) (BulkResult, error) {
	if !target.IsValid() {
		return BulkResult{}, invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", target))
	}
	if len(taskIDs) == 0 && len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}

	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, taskIDs, locators)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture bulk transition rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	for _, ref := range refs {
		if transitionErr := validateLifecycleTransition(ref, target); transitionErr != nil {
			return BulkResult{}, transitionErr
		}
	}
	now := s.clock.Now()
	for _, ref := range refs {
		if stateErr := applyState(ref.Task, target, now); stateErr != nil {
			return BulkResult{}, invalidInput("state", stateErr.Error())
		}
	}

	commits, indexes := distinctProjectCommits(refs, snap)
	targetLocators := make([]TaskLocator, 0, len(refs))
	for _, ref := range refs {
		targetLocators = append(targetLocators, TaskLocator{
			ProjectID: ref.Project.ID,
			TaskID:    ref.Task.ID,
		})
	}
	if planningErr := attachPlanningArtifacts(
		commits,
		collectPlanningReconcileTargets(commits, nil, targetLocators),
		s.defaultedPlanningArtifacts(nil),
	); planningErr != nil {
		return BulkResult{}, planningErr
	}

	outcome, err := s.commitProjects(commits, "Bulk Change State", models.WorkflowStateGroupForState(target).Label())
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	result := BulkResult{
		Tasks:         make([]TaskResult, 0, len(refs)),
		TransactionID: transactionIDAt(outcome.results, 0),
	}
	for _, ref := range refs {
		result.Tasks = append(result.Tasks, newTaskResult(
			ref,
			revisionAt(outcome.results, indexes[ref.Project.ID]),
			transactionIDAt(outcome.results, indexes[ref.Project.ID]),
		))
	}
	for _, commit := range commits {
		result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
	}
	return result, nil
}
