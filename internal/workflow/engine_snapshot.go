package workflow

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// CopyTaskState creates a shallow copy of the fields that event-based
// triggers inspect (status, priority, section, tags, type, due date,
// recurrence, dependencies). This is intentionally minimal — only fields
// checked by matchesEventTrigger, matchesHierarchicalTrigger, and
// TaskStateChanged are copied.
func CopyTaskState(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}

	tagsCopy := make([]string, len(task.Tags))
	copy(tagsCopy, task.Tags)

	var recCopy *models.RecurrencePattern
	if task.Recurrence != nil {
		rc := *task.Recurrence
		if len(rc.Weekdays) > 0 {
			rc.Weekdays = append([]int(nil), rc.Weekdays...)
		}
		recCopy = &rc
	}

	var dueCopy *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueCopy = &d
	}

	return &models.Task{
		ID:            task.ID,
		Status:        task.Status,
		Priority:      task.Priority,
		Section:       task.Section,
		CustomSection: task.CustomSection,
		Tags:          tagsCopy,
		Type:          task.Type,
		DueDate:       dueCopy,
		Description:   task.Description,
		Recurrence:    recCopy,
		Parent:        task.Parent,
		Children:      task.Children,
		Level:         task.Level,
		DependsOn:     append([]string(nil), task.DependsOn...),
	}
}

// CaptureTaskStates creates a snapshot of task states for event trigger detection.
// Call this before making changes to tasks to capture their previous state.
func CaptureTaskStates(tasks []*models.Task) map[string]*models.Task {
	states := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		states[task.ID] = CopyTaskState(task)
	}
	return states
}

// TaskStateChanged returns true if the task's workflow-relevant state changed.
func TaskStateChanged(current, prev *models.Task) bool {
	if prev == nil {
		return true
	}
	if current.Status != prev.Status ||
		current.Priority != prev.Priority ||
		current.Section != prev.Section ||
		current.CustomSection != prev.CustomSection ||
		current.Type != prev.Type {
		return true
	}
	// Detect due date changes (added, removed, or modified)
	switch {
	case current.DueDate == nil && prev.DueDate == nil:
		// no change
	case current.DueDate == nil || prev.DueDate == nil:
		return true // one is nil, the other is not
	default:
		if !current.DueDate.Equal(*prev.DueDate) {
			return true
		}
	}
	if len(current.Tags) != len(prev.Tags) {
		return true
	}
	for i, tag := range current.Tags {
		if tag != prev.Tags[i] {
			return true
		}
	}
	return false
}

// BuildEvaluationContext creates an evaluation context for workflow rules.
// Only tasks that actually changed (or are new) are included in Changes
// so that state-based rules don't re-fire on every operation.
// AllTasks and TaskMap still contain all tasks for hierarchical trigger lookups.
func BuildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task, projectName, projectID string) *EvaluationContext {
	taskMap := make(map[string]*models.Task, len(tasks))
	var changes []TaskChangeContext

	for _, task := range tasks {
		taskMap[task.ID] = task
		prev := prevStates[task.ID]
		if prev == nil || TaskStateChanged(task, prev) {
			changes = append(changes, TaskChangeContext{
				Task:     task,
				Previous: prev,
			})
		}
	}

	return &EvaluationContext{
		Changes:     changes,
		AllTasks:    tasks,
		TaskMap:     taskMap,
		ProjectName: projectName,
		ProjectID:   projectID,
	}
}
