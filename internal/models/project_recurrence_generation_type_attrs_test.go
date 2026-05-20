package models

import (
	"testing"
	"time"
)

func TestGenerateRecurringInstancesCopiesTypeAttrs(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("BugAttrs copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-1",
			ShortID:    1,
			Type:       TaskTypeBug,
			Title:      "Recurring Bug",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
			BugAttrs: &BugAttrs{
				Severity:     BugSeverityCritical,
				Reproducible: BugReproducibleYes,
				Workaround:   "restart the server",
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.BugAttrs == nil {
			t.Fatal("BugAttrs should be copied to new instance")
		}
		if newTask.BugAttrs.Severity != BugSeverityCritical {
			t.Errorf("Severity = %s, want critical", newTask.BugAttrs.Severity)
		}
		if newTask.BugAttrs.Reproducible != BugReproducibleYes {
			t.Errorf("Reproducible = %s, want yes", newTask.BugAttrs.Reproducible)
		}
		if newTask.BugAttrs.Workaround != "restart the server" {
			t.Errorf("Workaround = %q, want 'restart the server'", newTask.BugAttrs.Workaround)
		}
		task.BugAttrs.Severity = BugSeverityMinor
		if newTask.BugAttrs.Severity != BugSeverityCritical {
			t.Error("BugAttrs should be a deep copy, not a shared pointer")
		}
	})

	t.Run("TaskAttrs copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-2",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Recurring Task",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			TaskAttrs: &TaskAttrs{
				Effort:   EffortSmall,
				WorkType: WorkTypeDevelopment,
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.TaskAttrs == nil {
			t.Fatal("TaskAttrs should be copied to new instance")
		}
		if newTask.TaskAttrs.Effort != EffortSmall {
			t.Errorf("Effort = %s, want small", newTask.TaskAttrs.Effort)
		}
		if newTask.TaskAttrs.WorkType != WorkTypeDevelopment {
			t.Errorf("WorkType = %s, want development", newTask.TaskAttrs.WorkType)
		}
	})

	t.Run("IdeaAttrs copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-3",
			ShortID:    1,
			Type:       TaskTypeIdea,
			Title:      "Recurring Idea",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
			IdeaAttrs: &IdeaAttrs{
				Category:  IdeaCategoryFeature,
				Effort:    EffortLarge,
				Value:     IdeaValueHigh,
				Validated: true,
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.IdeaAttrs == nil {
			t.Fatal("IdeaAttrs should be copied to new instance")
		}
		if newTask.IdeaAttrs.Category != IdeaCategoryFeature {
			t.Errorf("Category = %s, want feature", newTask.IdeaAttrs.Category)
		}
		if !newTask.IdeaAttrs.Validated {
			t.Error("Validated should be true")
		}
	})

	t.Run("Source and URL copied to new instance", func(t *testing.T) {
		task := &Task{
			ID:         "task-4",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Recurring with metadata",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
			Source:     "github",
			URL:        "https://github.com/example/issue/42",
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.Source != "github" {
			t.Errorf("Source = %q, want 'github'", newTask.Source)
		}
		if newTask.URL != "https://github.com/example/issue/42" {
			t.Errorf("URL = %q, want 'https://github.com/example/issue/42'", newTask.URL)
		}
	})

	t.Run("EpicAttrs deep-copies ScopeBaselinedAt pointer", func(t *testing.T) {
		baseline := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
		task := &Task{
			ID:         "task-epic",
			ShortID:    1,
			Type:       TaskTypeEpic,
			Title:      "Recurring Epic",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
			EpicAttrs: &EpicAttrs{
				Scope:             EpicScopeDefined,
				Milestone:         "Q1",
				InitialChildCount: 3,
				ScopeBaselinedAt:  &baseline,
			},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.EpicAttrs == nil {
			t.Fatal("EpicAttrs should be copied to new instance")
		}
		if newTask.EpicAttrs.ScopeBaselinedAt == nil {
			t.Fatal("ScopeBaselinedAt should be copied (non-nil)")
		}
		if !newTask.EpicAttrs.ScopeBaselinedAt.Equal(baseline) {
			t.Errorf("ScopeBaselinedAt = %v, want %v", *newTask.EpicAttrs.ScopeBaselinedAt, baseline)
		}
		// The two pointers must reference distinct underlying time values so a
		// later mutation of one does not silently bleed into the other.
		if newTask.EpicAttrs.ScopeBaselinedAt == task.EpicAttrs.ScopeBaselinedAt {
			t.Error("ScopeBaselinedAt pointer is shared between old and new task (must be deep-copied)")
		}
		mutated := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
		*task.EpicAttrs.ScopeBaselinedAt = mutated
		if newTask.EpicAttrs.ScopeBaselinedAt.Equal(mutated) {
			t.Error("mutating original ScopeBaselinedAt leaked into the new instance")
		}
	})

	t.Run("nil attrs not copied", func(t *testing.T) {
		task := &Task{
			ID:         "task-5",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Plain recurring",
			Status:     StatusDone,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}
		project := &Project{Tasks: []*Task{task}, NextShortID: 2}
		project.GenerateRecurringInstances()

		newTask := project.Tasks[0]
		if newTask.BugAttrs != nil {
			t.Error("BugAttrs should be nil for task without bug attrs")
		}
		if newTask.TaskAttrs != nil {
			t.Error("TaskAttrs should be nil for task without task attrs")
		}
	})
}
