package workflow

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// applyActionWithContext applies an action with full context for hierarchical and deferred actions.
func applyActionWithContext(task *models.Task, action RuleAction, evalCtx *EvaluationContext, result *EvaluationResult) bool {
	if IsHierarchicalAction(action.ActionType) {
		return applyHierarchicalAction(task, action, evalCtx, result)
	}

	switch action.ActionType {
	case ActionCreateFollowup:
		return applyCreateFollowup(task, action, result)
	case ActionMoveToProject:
		return applyMoveToProject(task, action, evalCtx, result)
	default:
	}

	return applyAction(task, action)
}

// applyHierarchicalAction applies actions that operate on parent/children.
// When the action mutates a related task (parent or child), the related task
// is appended to result.ModifiedTasks so the caller persists it alongside the
// current task. Without this, parent/child mutations would be silently lost.
func applyHierarchicalAction(task *models.Task, action RuleAction, evalCtx *EvaluationContext, result *EvaluationResult) bool {
	switch action.ActionType {
	case ActionCompleteParent:
		if task.Parent != nil && !task.Parent.IsComplete() {
			task.Parent.Complete()
			recordModified(result, task.Parent)
			return true
		}

	case ActionBlockParent:
		if task.Parent != nil && task.Parent.Status != models.StatusBlocked {
			task.Parent.Block()
			recordModified(result, task.Parent)
			return true
		}

	case ActionCascadeTag:
		modified := false
		for _, child := range task.Children {
			if child.HasTag(action.Value) {
				continue
			}
			child.AddTag(action.Value)
			// task.AddTag silently rejects invalid tags (empty, too long, or
			// containing forbidden characters). Re-check presence so we don't
			// record children as modified when nothing actually changed.
			if !child.HasTag(action.Value) {
				continue
			}
			recordModified(result, child)
			modified = true
		}
		return modified

	case ActionInheritTag:
		if task.Parent != nil && task.Parent.HasTag(action.Value) && !task.HasTag(action.Value) {
			task.AddTag(action.Value)
			return task.HasTag(action.Value)
		}

	case ActionNotifyParent:
		if task.Parent != nil {
			task.Parent.AddComment("[Workflow] " + action.Value)
			recordModified(result, task.Parent)
			return true
		}
	default:
	}

	_ = evalCtx
	return false
}

// recordModified appends t to result.ModifiedTasks if not already present
// (pointer equality), so hierarchical mutations of parent/child tasks are
// surfaced to the caller for persistence.
func recordModified(result *EvaluationResult, t *models.Task) {
	if result == nil || t == nil {
		return
	}
	if containsTask(result.ModifiedTasks, t) {
		return
	}
	result.ModifiedTasks = append(result.ModifiedTasks, t)
}

// applyCreateFollowup creates a new follow-up task linked to the original.
// The task is appended to result.NewFollowupTasks for the caller to persist.
func applyCreateFollowup(task *models.Task, action RuleAction, result *EvaluationResult) bool {
	followup, err := models.NewTask(uuid.New().String(), 0, models.TaskTypeTask, action.Value)
	if err != nil {
		return false
	}
	followup.Priority = models.PriorityDefault
	followup.AddTag("followup")
	_ = followup.AddRelatedTo(task.ID)
	_ = task.AddRelatedTo(followup.ID)
	result.NewFollowupTasks = append(result.NewFollowupTasks, followup)
	return true
}

// applyMoveToProject records a deferred move for the task.
// The actual move is handled by the caller that has access to project storage.
// Skips the move when the destination matches the current project (by name or
// ID) so a self-targeted rule doesn't inflate trigger counts, kick off
// recursive re-evaluation, or queue a destructive no-op that callers would
// execute as a real move (dropping cross-project deps/relations and reassigning
// ShortIDs).
func applyMoveToProject(task *models.Task, action RuleAction, evalCtx *EvaluationContext, result *EvaluationResult) bool {
	if evalCtx != nil {
		dest := action.Value
		if (evalCtx.ProjectName != "" && strings.EqualFold(dest, evalCtx.ProjectName)) ||
			(evalCtx.ProjectID != "" && dest == evalCtx.ProjectID) {
			return false
		}
	}
	result.DeferredMoves = append(result.DeferredMoves, DeferredMove{
		TaskID:      task.ID,
		Destination: action.Value,
	})
	return true
}

// applyAction applies a rule action to a task. Returns true if the task was modified.
func applyAction(task *models.Task, action RuleAction) bool { //nolint:gocyclo
	switch action.ActionType {
	case ActionSetStatus:
		newStatus := models.TaskStatus(action.Value)
		if !newStatus.IsValid() || task.Status == newStatus {
			return false
		}
		// Use lifecycle methods to ensure side effects (timestamps, sections, flags)
		// are applied consistently. Skip CanTransitionTo check since workflow rules
		// are intentional overrides (e.g., force-completing a blocked task).
		task.ApplyTransition(newStatus)
		return true
	case ActionSetPriority:
		newPriority, err := strconv.Atoi(action.Value)
		if err == nil && newPriority >= models.PriorityMin && newPriority <= models.PriorityMax && task.Priority != newPriority {
			task.Priority = newPriority
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionSetSection:
		if models.IsGTDSection(action.Value) || action.Value == "" {
			if task.Section != action.Value || task.CustomSection != "" {
				task.SetSection(action.Value)
				task.UpdatedAt = time.Now()
				return true
			}
		} else {
			if task.CustomSection != action.Value || task.Section != "" {
				task.SetSection(action.Value)
				// task.SetSection silently rejects invalid custom section
				// names (whitespace, semicolons, colons, over-length, or
				// case-variant GTD collisions). Re-check that the assignment
				// actually took effect so we don't churn UpdatedAt or report
				// a modification that never happened.
				if task.CustomSection != action.Value {
					return false
				}
				task.UpdatedAt = time.Now()
				return true
			}
		}
	case ActionAddTag:
		if !task.HasTag(action.Value) {
			task.AddTag(action.Value)
			// task.AddTag silently rejects invalid tags (empty, too long,
			// or containing forbidden characters). Re-check presence so we
			// don't report a modification that never happened.
			return task.HasTag(action.Value)
		}
	case ActionRemoveTag:
		if task.HasTag(action.Value) {
			task.RemoveTag(action.Value)
			return true
		}
	case ActionSetDueRelative:
		if task.DueDate != nil {
			return false
		}
		days, err := ParseRelativeDuration(action.Value)
		if err == nil {
			newDue := time.Now().AddDate(0, 0, days)
			task.DueDate = &newDue
			task.HasTime = false // Relative durations are always date-only.
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionClearDue:
		if task.DueDate != nil {
			task.DueDate = nil
			task.HasTime = false
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionSetActive:
		active := action.Value == "true"
		if task.Active != active {
			if active {
				task.Activate()
			} else {
				task.Deactivate()
			}
			return true
		}
	case ActionSetPinned:
		pinned := action.Value == "true"
		if task.Pinned != pinned {
			if pinned {
				task.Pin()
			} else {
				task.Unpin()
			}
			return true
		}
	case ActionAddComment:
		commentText := "[Workflow] " + action.Value
		// Dedupe against ALL existing comments, not just the most recent.
		// A rule may have appended the workflow comment earlier in the
		// session followed by unrelated user comments; without scanning
		// the full list the same workflow note would be appended again.
		for _, c := range task.Comments {
			if c != nil && c.Text == commentText {
				return false
			}
		}
		task.AddComment(commentText)
		return true
	case ActionSetType:
		newType := models.TaskType(action.Value)
		if !newType.IsValid() || task.Type == newType {
			return false
		}
		// Validate hierarchy: new type must be valid under current parent
		if task.Parent != nil && !newType.CanBeChildOf(task.Parent.Type) {
			return false
		}
		// Validate hierarchy: all children must be valid under the new type
		for _, child := range task.Children {
			if !child.Type.CanBeChildOf(newType) {
				return false
			}
		}
		// If new type cannot have children but task has children, reject
		if len(task.Children) > 0 && !newType.CanHaveChildren() {
			return false
		}
		task.Type = newType
		task.ClearTypeAttrs()
		task.UpdatedAt = time.Now()
		return true
	case ActionSetSnooze:
		days, err := ParseRelativeDuration(action.Value)
		if err != nil {
			return false
		}
		snoozeUntil := time.Now().AddDate(0, 0, days)
		// Treat a re-snooze to essentially the same target as a no-op so
		// recursive rule evaluation isn't triggered for an idle change.
		if task.SnoozedUntil != nil {
			diff := snoozeUntil.Sub(*task.SnoozedUntil)
			if diff < time.Minute && diff > -time.Minute {
				return false
			}
		}
		task.SnoozedUntil = &snoozeUntil
		task.UpdatedAt = time.Now()
		return true
	case ActionSetRecurrence:
		pattern := models.ParseRecurrence(action.Value)
		if pattern == nil {
			return false
		}
		// Skip the update when the task already has an equivalent pattern so
		// we don't churn UpdatedAt or spuriously re-trigger downstream rules.
		if recurrencePatternsEqual(task.Recurrence, pattern) {
			return false
		}
		// Use SetRecurrence to populate OriginalDay for monthly patterns
		if err := task.SetRecurrence(pattern); err != nil {
			return false
		}
		return true
	default:
	}
	return false
}

// recurrencePatternsEqual reports whether two recurrence patterns are
// semantically equivalent for the fields a workflow rule would set
// (interval, unit, weekdays, mode). Counters such as Streak/Skipped are
// ignored so a rule that re-asserts the same pattern is a no-op.
// Weekdays are compared as sets (order-independent) because ParseRecurrence
// preserves input order while stored patterns may be sorted differently.
func recurrencePatternsEqual(a, b *models.RecurrencePattern) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Interval != b.Interval || a.Unit != b.Unit || a.Mode != b.Mode {
		return false
	}
	if len(a.Weekdays) != len(b.Weekdays) {
		return false
	}
	// Compare as sets: build a count map from a and subtract using b.
	seen := make(map[int]int, len(a.Weekdays))
	for _, d := range a.Weekdays {
		seen[d]++
	}
	for _, d := range b.Weekdays {
		seen[d]--
		if seen[d] < 0 {
			return false
		}
	}
	return true
}
