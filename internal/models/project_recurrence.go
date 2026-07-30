package models

import (
	"time"

	"github.com/google/uuid"
)

// GenerateRecurringInstances creates the next Inbox instance for every
// completed recurring task.
func (p *Project) GenerateRecurringInstances() int {
	return p.GenerateRecurringInstancesAt(time.Now())
}

// GenerateRecurringInstancesAt is the deterministic form used by callers that
// own the mutation clock.
func (p *Project) GenerateRecurringInstancesAt(now time.Time) int {
	return p.GenerateRecurringInstancesAtWithID(now, uuid.NewString)
}

// GenerateRecurringInstancesAtWithID also injects identity allocation. It keeps
// completion and generation in the caller's single project transaction while
// allowing application tests and undo records to observe deterministic IDs.
func (p *Project) GenerateRecurringInstancesAtWithID(now time.Time, newID func() string) int {
	if newID == nil {
		newID = uuid.NewString
	}

	completed := make([]*Task, 0)
	for _, task := range p.Tasks {
		if task != nil && task.Recurrence != nil && task.State() == StateDone {
			completed = append(completed, task)
		}
	}
	if len(completed) == 0 {
		return 0
	}

	generated := make([]*Task, 0, len(completed))
	for _, task := range completed {
		completedAt := now
		if task.CompletedAt != nil {
			completedAt = *task.CompletedAt
		}
		nextDue := task.NextDueDateAfter(completedAt)
		if nextDue == nil {
			// Invalid persisted patterns are left attached to the completed task
			// so a later validation/migration path can report them without data
			// loss. Canonical SetRecurrence prevents this for ordinary writes.
			continue
		}

		recurrence := &RecurrencePattern{
			Interval: task.Recurrence.Interval,
			Unit:     task.Recurrence.Unit,
		}
		checklist := make([]ChecklistItem, len(task.Checklist))
		for i, item := range task.Checklist {
			checklist[i] = ChecklistItem{Text: item.Text}
		}
		var parentID *string
		if task.ParentID != nil {
			value := *task.ParentID
			parentID = &value
		}

		generated = append(generated, &Task{
			ID:            newID(),
			ShortID:       p.GetNextShortID(),
			Title:         task.Title,
			Description:   task.Description,
			WorkflowState: StateInbox,
			Priority:      task.Priority,
			ParentID:      parentID,
			Tags:          append([]string(nil), task.Tags...),
			CreatedAt:     now,
			UpdatedAt:     now,
			DueDate:       nextDue,
			Recurrence:    recurrence,
			Checklist:     checklist,
		})

		// Moving the pattern to the generated task makes the hook idempotent.
		task.Recurrence = nil
		task.UpdatedAt = now
	}

	if len(generated) > 0 {
		p.Tasks = append(generated, p.Tasks...)
	}
	return len(generated)
}
