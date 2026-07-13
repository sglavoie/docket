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

// BaselineEpic records an epic's current direct-child count as its scope
// baseline, so ScopeDelta() reports growth or shrinkage relative to it.
// Re-baselining updates both fields (idempotent). Non-epic tasks and epics with
// no children are rejected before any mutation.
func (s *Service) BaselineEpic(taskID string) (EpicBaselineResult, error) {
	var childCount int
	res, err := s.mutateTask(taskID, "Baseline Epic", "", func(ref TaskRef) error {
		task := ref.Task
		if task.Type != models.TaskTypeEpic {
			return invalidInput("epic", fmt.Sprintf("task #%d is a %s, not an epic", task.ShortID, task.Type))
		}
		if len(task.Children) == 0 {
			return invalidInput("epic", fmt.Sprintf("epic #%d must have at least one child to baseline", task.ShortID))
		}
		now := s.clock.Now()
		task.EnsureEpicAttrs().InitialChildCount = len(task.Children)
		task.EnsureEpicAttrs().ScopeBaselinedAt = &now
		task.UpdatedAt = now
		childCount = len(task.Children)
		return nil
	})
	if err != nil {
		return EpicBaselineResult{}, err
	}
	return EpicBaselineResult{Task: res, ChildCount: childCount}, nil
}
