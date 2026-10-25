package app

import (
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// applyTaskLineEditEdges submits the edited task's own parent change together
// with every child it adopts as one checked hierarchy patch, so cycles and
// nesting depth are measured on the graph the edit actually produces rather than
// an intermediate one. It returns the number of tasks whose parent really moved,
// which is what makes an already-direct child a no-op. With no adopted children
// the singleton reparent keeps the error vocabulary it has always produced.
func applyTaskLineEditEdges(project *models.Project, task, parent *models.Task, parentChanged bool, children []*models.Task, now time.Time) (int, error) {
	if len(children) == 0 {
		if !parentChanged {
			return 0, nil
		}
		if err := project.ReparentTaskAt(task, parent, now); err != nil {
			return 0, invalidInput("parent", err.Error())
		}
		return 1, nil
	}
	assignments := make([]models.ParentAssignment, 0, len(children)+1)
	if parentChanged {
		assignments = append(assignments, models.ParentAssignment{Task: task, Parent: parent})
	}
	for _, child := range children {
		assignments = append(assignments, models.ParentAssignment{Task: child, Parent: task})
	}
	changed, err := project.ApplyParentAssignmentsAt(assignments, now)
	if err != nil {
		return 0, invalidInput("child", err.Error())
	}
	return changed, nil
}

// taskLineDestinationChildrenGuard is the incomplete-child half of the lifecycle
// policy, run after the edit's hierarchy patch has landed. Splitting it away from
// the parse-time state-edge check is what stops a $done or $canceled destination
// from being cleared against the children the same edit is replacing.
//
// It deliberately carries no equal-state shortcut. A destination the task already
// holds is still an explicit terminal claim, and skipping it would let $done on an
// already-done task adopt an open child and be silently reopened by
// ReconcileParentCompletionAt, which is exactly the outcome the policy forbids.
func taskLineDestinationChildrenGuard(ref TaskRef, destination *CaptureDestination) error {
	if destination == nil {
		return nil
	}
	return childrenCompletionGuard(ref.Task, ref.Project, destination.WorkflowState())
}

func validateTaskLineDestination(ref TaskRef, destination CaptureDestination) error {
	target := destination.WorkflowState()
	switch destination {
	case CaptureDestinationToday:
		if ref.Task.IsTerminalState() {
			return invalidInput("planning", "terminal tasks cannot be added to Today")
		}
		if ref.Task.State() == models.StateDoing {
			return nil
		}
	case CaptureDestinationNext:
		if ref.Task.IsTerminalState() {
			return invalidInput("planning", "terminal tasks cannot be added to Next")
		}
	case CaptureDestinationBacklog:
		// Backlog uses the canonical planning action below so it also clears Today
		// and Next membership.
	default:
	}
	if target == ref.Task.State() {
		return nil
	}
	return validateLifecycleStateEdge(ref.Task.State(), target)
}

func applyPreparedTaskLineFields(task *models.Task, prepared preparedTaskLineEdit, now time.Time) bool {
	changed := false
	if task.Title != prepared.title {
		task.Title = prepared.title
		changed = true
	}
	if prepared.description != nil && task.Description != *prepared.description {
		task.Description = *prepared.description
		changed = true
	}
	if prepared.resumeNote != nil && task.ResumeNote != *prepared.resumeNote {
		task.ResumeNote = *prepared.resumeNote
		changed = true
	}
	if prepared.priority != nil && task.Priority != *prepared.priority {
		task.Priority = *prepared.priority
		changed = true
	}
	if prepared.dueSet && !dueDatePtrsEqual(task.DueDate, prepared.due) {
		task.DueDate = cloneDueDatePtr(prepared.due)
		changed = true
	}
	if prepared.context != nil && task.Context != *prepared.context {
		task.Context = *prepared.context
		changed = true
	}
	if !slices.Equal(task.Tags, prepared.tags) {
		task.Tags = slices.Clone(prepared.tags)
		changed = true
	}
	if prepared.pinned != nil && task.ApplyPinned(*prepared.pinned, now) {
		changed = true
	}
	if prepared.review != nil && task.ApplyReview(*prepared.review, now) {
		changed = true
	}
	// A patch that resolves to the set already stored — adding what is there,
	// removing what is not, clearing nothing — leaves the field and the touch time
	// alone, so it reaches the caller as an unchanged result.
	if prepared.dependenciesSet && !models.TaskReferencesEqual(task.Dependencies, prepared.dependencies) {
		task.Dependencies = models.CloneTaskReferences(prepared.dependencies)
		changed = true
	}
	if changed {
		task.UpdatedAt = now
	}
	return changed
}

// applyPreparedTaskLineDeferral writes the deferral after the lifecycle move,
// because entering a state clears DeferredUntil. A line that defers always moves
// to Backlog, so writing it alongside the other scalars would lose it.
func applyPreparedTaskLineDeferral(task *models.Task, prepared preparedTaskLineEdit, now time.Time) (bool, error) {
	if !prepared.deferSet || instantPtrsEqual(task.DeferredUntil, prepared.deferUntil) {
		return false, nil
	}
	if prepared.deferUntil != nil {
		if err := task.ApplyDeferAt(*prepared.deferUntil, now); err != nil {
			return false, invalidInput("defer", err.Error())
		}
		return true, nil
	}
	task.DeferredUntil = nil
	task.UpdatedAt = now
	return true, nil
}

func patchedTaskLineTags(existing, add, remove []string) []string {
	result := slices.Clone(existing)
	for _, tag := range remove {
		result = slices.DeleteFunc(result, func(candidate string) bool {
			normalized, err := models.NormalizeTaskTag(candidate)
			return err == nil && normalized == tag
		})
	}
	for _, tag := range add {
		if !slices.ContainsFunc(result, func(candidate string) bool {
			normalized, err := models.NormalizeTaskTag(candidate)
			return err == nil && normalized == tag
		}) {
			result = append(result, tag)
		}
	}
	return result
}

func instantPtrsEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func dueDatePtrsEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func (s *Service) applyTaskLineDestination(ref TaskRef, destination *CaptureDestination) ([]PlanningReconcileTarget, bool, error) {
	if destination == nil {
		return nil, false, nil
	}
	var action PlanningAction
	switch *destination {
	case CaptureDestinationToday:
		action = PlanningAddToday
	case CaptureDestinationNext:
		action = PlanningAddNext
	case CaptureDestinationBacklog:
		action = PlanningBacklog
	default:
		state := destination.WorkflowState()
		if state == ref.Task.State() {
			return nil, false, nil
		}
		if err := applyState(ref.Task, state, s.clock.Now()); err != nil {
			return nil, false, invalidInput("destination", err.Error())
		}
		return []PlanningReconcileTarget{{
			TaskID: ref.Task.ID, ProjectID: ref.Project.ID, State: ref.Task.State(), StateChanged: true,
		}}, true, nil
	}
	target, changed, err := ApplyPlanningActionToTask(ref.Task, ref.Project.ID, action, s.clock.Now())
	if err != nil {
		return nil, false, err
	}
	return []PlanningReconcileTarget{target}, changed, nil
}
