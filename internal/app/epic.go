package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// EpicBaselineResult is the presentation-neutral outcome of baselining an
// epic's scope: the standard task result plus the child count that was recorded
// as the baseline.
type EpicBaselineResult struct {
	// Task is the standard single-task mutation result.
	Task TaskResult
	// ChildCount is the number of direct children recorded as the baseline.
	ChildCount int
}

// EpicBaselineWorkflowResult preserves the epic-specific child count alongside
// the common workflow cascade effects.
type EpicBaselineWorkflowResult struct {
	Baseline EpicBaselineResult
	Cascade  BulkResult
}

// BaselineEpic records an epic's current direct-child count as its scope
// baseline, so ScopeDelta() reports growth or shrinkage relative to it.
// Re-baselining updates both fields (idempotent). Non-epic tasks and epics with
// no children are rejected before any mutation.
func (s *Service) BaselineEpic(taskID string) (EpicBaselineResult, error) {
	var childCount int
	res, err := s.mutateTask(taskID, "Baseline Epic", "", func(ref TaskRef) error {
		var mutationErr error
		childCount, mutationErr = s.baselineEpic(ref.Task)
		return mutationErr
	})
	if err != nil {
		return EpicBaselineResult{}, err
	}
	return EpicBaselineResult{Task: res, ChildCount: childCount}, nil
}

func (s *Service) baselineEpic(task *models.Task) (int, error) {
	if task.Type != models.TaskTypeEpic {
		return 0, invalidInput("epic", fmt.Sprintf("task #%d is a %s, not an epic", task.ShortID, task.Type))
	}
	if len(task.Children) == 0 {
		return 0, invalidInput("epic", fmt.Sprintf("epic #%d must have at least one child to baseline", task.ShortID))
	}
	now := s.clock.Now()
	attrs := task.EnsureEpicAttrs()
	attrs.InitialChildCount = len(task.Children)
	attrs.ScopeBaselinedAt = &now
	task.UpdatedAt = now
	return len(task.Children), nil
}

// BaselineEpicWithWorkflow records the current direct-child count through the
// same validation as BaselineEpic and runs the workflow cascade atomically.
func (s *Service) BaselineEpicWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (EpicBaselineWorkflowResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Baseline Epic")
	var childCount int
	cascade, err := s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		var mutationErr error
		childCount, mutationErr = s.baselineEpic(ref.Task)
		return mutationErr
	})
	if err != nil {
		return EpicBaselineWorkflowResult{}, err
	}
	baseline := EpicBaselineResult{Task: cascade.Tasks[0], ChildCount: childCount}
	return EpicBaselineWorkflowResult{Baseline: baseline, Cascade: cascade}, nil
}
