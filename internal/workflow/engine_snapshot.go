package workflow

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// snapshotTaskForEvent creates a shallow copy of the fields that event-based
// triggers inspect (status, priority, section, tags, type, due date,
// recurrence). This is intentionally minimal — only fields checked by
// matchesEventTrigger and matchesHierarchicalTrigger are copied.
func snapshotTaskForEvent(task *models.Task) *models.Task {
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
