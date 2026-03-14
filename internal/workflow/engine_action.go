package workflow

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// applyActionWithContext applies an action with full context for hierarchical and deferred actions.
func applyActionWithContext(task *models.Task, action RuleAction, evalCtx *EvaluationContext, result *EvaluationResult) bool {
	if IsHierarchicalAction(action.ActionType) {
		return applyHierarchicalAction(task, action, evalCtx)
	}

	switch action.ActionType {
	case ActionCreateFollowup:
		return applyCreateFollowup(task, action, result)
	case ActionMoveToProject:
		return applyMoveToProject(task, action, result)
	}

	return applyAction(task, action)
}

// applyHierarchicalAction applies actions that operate on parent/children.
func applyHierarchicalAction(task *models.Task, action RuleAction, evalCtx *EvaluationContext) bool {
	switch action.ActionType {
	case ActionCompleteParent:
		if task.Parent != nil && !task.Parent.IsComplete() {
			task.Parent.Complete()
			return true
		}

	case ActionBlockParent:
		if task.Parent != nil && task.Parent.Status != models.StatusBlocked {
			task.Parent.Block()
			return true
		}

	case ActionCascadeTag:
		modified := false
		for _, child := range task.Children {
			if !child.HasTag(action.Value) {
				child.AddTag(action.Value)
				modified = true
			}
		}
		return modified

	case ActionInheritTag:
		if task.Parent != nil && task.Parent.HasTag(action.Value) && !task.HasTag(action.Value) {
			task.AddTag(action.Value)
			return true
		}

	case ActionNotifyParent:
		if task.Parent != nil {
			task.Parent.AddComment("[Workflow] " + action.Value)
			return true
		}
	}

	_ = evalCtx
	return false
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
func applyMoveToProject(task *models.Task, action RuleAction, result *EvaluationResult) bool {
	result.DeferredMoves = append(result.DeferredMoves, DeferredMove{
		TaskID:      task.ID,
		Destination: action.Value,
	})
	return true
}

// applyAction applies a rule action to a task. Returns true if the task was modified.
func applyAction(task *models.Task, action RuleAction) bool {
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
			return true
		}
	case ActionSetSection:
		if models.IsGTDSection(action.Value) || action.Value == "" {
			if task.Section != action.Value {
				task.Section = action.Value
				return true
			}
		} else {
			if task.CustomSection != action.Value {
				task.CustomSection = action.Value
				return true
			}
		}
	case ActionAddTag:
		if !task.HasTag(action.Value) {
			task.AddTag(action.Value)
			return true
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
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionClearDue:
		if task.DueDate != nil {
			task.DueDate = nil
			task.UpdatedAt = time.Now()
			return true
		}
	case ActionSetActive:
		active := action.Value == "true"
		if task.Active != active {
			task.Active = active
			task.UpdatedAt = time.Now()
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
		if len(task.Comments) > 0 {
			last := task.Comments[len(task.Comments)-1]
			if last.Text == commentText {
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
		task.SnoozedUntil = &snoozeUntil
		task.UpdatedAt = time.Now()
		return true
	case ActionSetRecurrence:
		pattern := models.ParseRecurrence(action.Value)
		if pattern == nil {
			return false
		}
		task.Recurrence = pattern
		task.UpdatedAt = time.Now()
		return true
	}
	return false
}
