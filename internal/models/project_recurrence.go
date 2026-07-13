package models

import (
	"maps"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/util"
)

// GenerateRecurringInstances checks for completed recurring tasks and creates
// their next instances. Returns the number of instances generated.
// New instances are placed in the Inbox section with status todo.
func (p *Project) GenerateRecurringInstances() int {
	generated := 0
	now := time.Now()

	// Collect tasks that need new instances (don't modify slice while iterating)
	var toGenerate []*Task
	for _, task := range p.Tasks {
		if task.Recurrence != nil && task.State() == StateDone {
			// Only generate on successful terminal completion (done, not
			// canceled). State() routes this through the canonical workflow
			// model instead of reading the legacy Status field directly.
			toGenerate = append(toGenerate, task)
		}
	}

	for _, task := range toGenerate {
		// Update streak based on completion timeliness
		if task.Recurrence != nil {
			onTime := true
			if task.DueDate != nil && task.CompletedAt != nil {
				completedDay := util.StartOfDay(*task.CompletedAt)
				dueDay := util.StartOfDay(*task.DueDate)
				onTime = !completedDay.After(dueDay)
			}
			// No DueDate counts as on-time
			if onTime {
				task.Recurrence.Streak++
				if task.Recurrence.Streak > task.Recurrence.MaxStreak {
					task.Recurrence.MaxStreak = task.Recurrence.Streak
				}
			} else {
				task.Recurrence.Streak = 0
			}
		}

		// Calculate next due date
		nextDue := task.NextDueDate()

		// Create new instance — carry forward recurrence pattern with streak data
		newRecurrence := &RecurrencePattern{
			Interval:    task.Recurrence.Interval,
			Unit:        task.Recurrence.Unit,
			Weekdays:    append([]int{}, task.Recurrence.Weekdays...),
			Skipped:     task.Recurrence.Skipped,
			Streak:      task.Recurrence.Streak,
			MaxStreak:   task.Recurrence.MaxStreak,
			OriginalDay: task.Recurrence.OriginalDay,
			Mode:        task.Recurrence.Mode,
		}

		// Copy score dimensions from the completed task
		scoreDims := make(map[string]int, len(task.ScoreDimensions))
		maps.Copy(scoreDims, task.ScoreDimensions)

		// Copy checklist items (reset all to unchecked for the new instance)
		var checklist []ChecklistItem
		if len(task.Checklist) > 0 {
			checklist = make([]ChecklistItem, len(task.Checklist))
			for i, item := range task.Checklist {
				checklist[i] = ChecklistItem{Text: item.Text, Done: false}
			}
		}

		var startDateCopy *time.Time
		if task.StartDate != nil {
			sd := *task.StartDate
			startDateCopy = &sd
		}

		newTask := &Task{
			ID:              uuid.New().String(),
			ShortID:         p.GetNextShortID(),
			Type:            task.Type,
			Title:           task.Title,
			Description:     task.Description, // Carry forward description for context
			WorkflowState:   StateInbox,       // New instances go to Inbox for triage
			Priority:        task.Priority,
			Tags:            append([]string{}, task.Tags...),
			DependsOn:       []string{},
			RelatedTo:       []string{},
			ScoreDimensions: scoreDims,
			Children:        []*Task{},
			DueDate:         nextDue,
			HasTime:         task.HasTime,
			Recurrence:      newRecurrence,
			Checklist:       checklist,
			StartDate:       startDateCopy,
			EstimatedDays:   task.EstimatedDays,
			Source:          task.Source,
			URL:             task.URL,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		// Copy type-specific attributes (value types, so shallow copy is safe)
		if task.BugAttrs != nil {
			copied := *task.BugAttrs
			newTask.BugAttrs = &copied
		}
		if task.IdeaAttrs != nil {
			copied := *task.IdeaAttrs
			newTask.IdeaAttrs = &copied
		}
		if task.TaskAttrs != nil {
			copied := *task.TaskAttrs
			newTask.TaskAttrs = &copied
		}
		if task.StoryAttrs != nil {
			copied := *task.StoryAttrs
			newTask.StoryAttrs = &copied
		}
		if task.EpicAttrs != nil {
			copied := *task.EpicAttrs
			// Deep-copy the *time.Time so old/new tasks don't share the
			// underlying time value (a later in-place modification of one
			// would otherwise leak into the other).
			if task.EpicAttrs.ScopeBaselinedAt != nil {
				baselined := *task.EpicAttrs.ScopeBaselinedAt
				copied.ScopeBaselinedAt = &baselined
			}
			newTask.EpicAttrs = &copied
		}

		// Clear recurrence from completed task (pattern moves to new instance)
		task.Recurrence = nil
		task.UpdatedAt = now

		// Add new task at the beginning of the list
		p.Tasks = append([]*Task{newTask}, p.Tasks...)
		generated++
	}

	// Ensure Inbox section exists if we generated anything
	if generated > 0 {
		p.EnsureSection(SectionInbox)
	}

	return generated
}
