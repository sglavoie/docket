package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// ProjectCommitOptions labels an already-applied project mutation and names the
// tasks whose final states must be reconciled with workspace planning.
type ProjectCommitOptions struct {
	UndoAction      string
	UndoContext     string
	PlanningTaskIDs []string
}

// CommitProject validates and commits an already-mutated project through the
// workspace ledger. This is the retained transaction boundary for legacy
// project-tree mutations that edit an in-memory project before persistence.
func (s *Service) CommitProject(project *models.Project, opts ProjectCommitOptions) (BulkResult, error) {
	if project == nil || project.ID == "" {
		return BulkResult{}, invalidInput("project", "project is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	baseProject := snap.ProjectByID(project.ID)
	if baseProject == nil {
		return BulkResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, project.ID)
	}
	baseDocument := snap.Document(project.ID)
	if baseDocument == nil {
		return BulkResult{}, fmt.Errorf("app: project %s has no immutable base document", project.ID)
	}

	baseTasks := make(map[string]*models.Task, len(baseDocument.Tasks))
	for _, task := range baseDocument.Tasks {
		if task != nil {
			baseTasks[task.ID] = task
		}
	}
	now := s.clock.Now()
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		previous := baseTasks[task.ID]
		if previous == nil || previous.State() == task.State() {
			continue
		}
		ref := TaskRef{Task: task, Project: project, Base: baseDocument}
		if transitionErr := validateLifecycleTransitionFrom(previous.State(), ref, task.State()); transitionErr != nil {
			return BulkResult{}, transitionErr
		}
		if stateErr := applyState(task, task.State(), now); stateErr != nil {
			return BulkResult{}, invalidInput("state", stateErr.Error())
		}
	}

	commit := projectCommit{
		project:          project,
		expectedRevision: snap.Revision(project.ID),
		base:             baseDocument,
	}
	commits := []projectCommit{commit}
	locators := make([]TaskLocator, 0, len(opts.PlanningTaskIDs))
	for _, taskID := range opts.PlanningTaskIDs {
		locators = append(locators, TaskLocator{ProjectID: project.ID, TaskID: taskID})
	}
	if planningErr := attachPlanningArtifacts(
		commits,
		collectPlanningReconcileTargets(commits, nil, locators),
		s.defaultedPlanningArtifacts(nil),
	); planningErr != nil {
		return BulkResult{}, planningErr
	}

	action := opts.UndoAction
	if action == "" {
		action = "Save Project"
	}
	outcome, err := s.commitProjects(commits, action, opts.UndoContext)
	if err != nil {
		return BulkResult{}, err
	}
	return BulkResult{
		TouchedProjectIDs: []string{project.ID},
		TransactionID:     transactionIDAt(outcome.results, 0),
		Tasks: []TaskResult{{
			ProjectID:     project.ID,
			Revision:      revisionAt(outcome.results, 0),
			TransactionID: transactionIDAt(outcome.results, 0),
		}},
	}, nil
}
