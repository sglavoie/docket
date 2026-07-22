package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// PlanningReconcileTarget names a task whose workflow state a transaction set,
// paired with the final state the task holds after the mutation and cascade. A
// PlanningArtifactBuilder consumes these to enforce Today/Next exclusivity for
// exactly the tasks a transaction touched: a task now in StateNext must leave the
// Daily Plan (Today), and a task that left StateNext must drop its stale Next
// order-index entry — both folded into the same durable ledger transaction as the
// task write so undo restores them together.
type PlanningReconcileTarget struct {
	TaskID    string
	ProjectID string
	State     models.WorkflowState
}

// PlanningArtifactBuilder renders the durable planning-collection change a
// transition implies into zero or more revision-checked artifacts committed in
// the same transaction as the task write. It is the planning-domain analog of
// WorkflowProjectStatusArtifactBuilder: the application layer stays config-
// agnostic and the adapter (CLI or TUI) supplies the concrete planning.json
// reconciliation. Returning no artifacts (nil) means the transition implied no
// planning change.
type PlanningArtifactBuilder func([]PlanningReconcileTarget) ([]storage.ArtifactChange, error)

// collectPlanningReconcileTargets resolves each task id to its final state and
// owning project among the committed projects, so the builder reconciles against
// post-mutation, post-cascade state. Ids absent from every committed project
// (e.g. a task moved out by a workflow rule) are skipped.
func collectPlanningReconcileTargets(commits []projectCommit, taskIDs []string, locators []TaskLocator) []PlanningReconcileTarget {
	if len(taskIDs) == 0 && len(locators) == 0 {
		return nil
	}
	seen := make(map[TaskLocator]bool, len(taskIDs)+len(locators))
	targets := make([]PlanningReconcileTarget, 0, len(taskIDs)+len(locators))
	appendTarget := func(locator TaskLocator, task *models.Task) {
		if task == nil || !locator.Valid() || seen[locator] {
			return
		}
		seen[locator] = true
		targets = append(targets, PlanningReconcileTarget{
			TaskID: locator.TaskID, ProjectID: locator.ProjectID, State: task.State(),
		})
	}
	for _, locator := range locators {
		for _, c := range commits {
			if c.project != nil && c.project.ID == locator.ProjectID {
				appendTarget(locator, c.project.GetTaskByID(locator.TaskID))
				break
			}
		}
	}
	for _, id := range taskIDs {
		if id == "" {
			continue
		}
		for _, c := range commits {
			if c.project == nil {
				continue
			}
			if task := c.project.GetTaskByID(id); task != nil {
				appendTarget(TaskLocator{ProjectID: c.project.ID, TaskID: id}, task)
			}
		}
	}
	return targets
}

// attachPlanningArtifacts folds the planning reconciliation implied by targets
// into the first project's changeset, mirroring attachWorkflowProjectStatusArtifacts.
// Every planning artifact must carry an expected revision so a concurrent config
// writer is detected rather than silently overwritten.
func attachPlanningArtifacts(commits []projectCommit, targets []PlanningReconcileTarget, builder PlanningArtifactBuilder) error {
	if builder == nil || len(targets) == 0 {
		return nil
	}
	artifacts, err := builder(targets)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		return nil
	}
	if len(commits) == 0 {
		return fmt.Errorf("app: planning reconciliation requires at least one project commit")
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return fmt.Errorf("app: planning artifact %q has no expected revision", artifact.Path)
		}
	}
	commits[0].artifacts = append(commits[0].artifacts, artifacts...)
	return nil
}
