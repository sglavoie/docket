package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestCaptureTaskStates(t *testing.T) {
	t.Run("captures task states", func(t *testing.T) {
		task1 := testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo)
		task1.Priority = 2
		task1.Tags = []string{"urgent", "backend"}

		task2 := testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress)
		task2.Section = "In Progress"

		tasks := []*models.Task{task1, task2}
		states := CaptureTaskStates(tasks)

		if len(states) != 2 {
			t.Fatalf("CaptureTaskStates() captured %d states, want 2", len(states))
		}

		state1 := states[task1.ID]
		if state1 == nil {
			t.Fatal("task1 state not captured")
		}
		if state1.Status != task1.Status {
			t.Errorf("state1.Status = %s, want %s", state1.Status, task1.Status)
		}
		if state1.Priority != task1.Priority {
			t.Errorf("state1.Priority = %d, want %d", state1.Priority, task1.Priority)
		}
		if len(state1.Tags) != 2 {
			t.Errorf("state1.Tags length = %d, want 2", len(state1.Tags))
		}
		if state1 == task1 {
			t.Error("CaptureTaskStates() should create copies, not reference originals")
		}

		state2 := states[task2.ID]
		if state2 == nil {
			t.Fatal("task2 state not captured")
		}
		if state2.Section != task2.Section {
			t.Errorf("state2.Section = %s, want %s", state2.Section, task2.Section)
		}
	})

	t.Run("captures recurrence Mode and OriginalDay", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Recurring", models.StatusTodo)
		task.Recurrence = &models.RecurrencePattern{
			Interval:    1,
			Unit:        models.RecurrenceMonthly,
			OriginalDay: 31,
			Mode:        models.RecurModeFromCompletion,
			Streak:      5,
			MaxStreak:   10,
		}

		states := CaptureTaskStates([]*models.Task{task})
		state := states[task.ID]

		if state.Recurrence == nil {
			t.Fatal("captured state should have Recurrence")
		}
		if state.Recurrence.OriginalDay != 31 {
			t.Errorf("Recurrence.OriginalDay = %d, want 31", state.Recurrence.OriginalDay)
		}
		if state.Recurrence.Mode != models.RecurModeFromCompletion {
			t.Errorf("Recurrence.Mode = %q, want %q", state.Recurrence.Mode, models.RecurModeFromCompletion)
		}

		// Mutating original should not affect captured state
		task.Recurrence.Mode = models.RecurModeFromDue
		task.Recurrence.OriginalDay = 1
		if state.Recurrence.Mode != models.RecurModeFromCompletion {
			t.Error("captured Recurrence.Mode should not change after original mutation")
		}
		if state.Recurrence.OriginalDay != 31 {
			t.Error("captured Recurrence.OriginalDay should not change after original mutation")
		}
	})

	t.Run("handles empty task list", func(t *testing.T) {
		states := CaptureTaskStates([]*models.Task{})
		if len(states) != 0 {
			t.Errorf("CaptureTaskStates([]) should return empty map, got %d states", len(states))
		}
	})

	t.Run("captures CustomSection field", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.CustomSection = "Sprint-1"

		states := CaptureTaskStates([]*models.Task{task})
		state := states[task.ID]

		if state.CustomSection != "Sprint-1" {
			t.Errorf("captured state CustomSection = %q, want %q", state.CustomSection, "Sprint-1")
		}

		task.CustomSection = "Sprint-2"
		if state.CustomSection != "Sprint-1" {
			t.Errorf("captured state CustomSection should not change after original mutation")
		}
	})

	t.Run("modifying original tasks does not affect captured states", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Priority = 3
		task.Tags = []string{"original"}

		states := CaptureTaskStates([]*models.Task{task})
		capturedState := states[task.ID]

		task.Priority = 5
		task.Tags = []string{"modified"}

		if capturedState.Priority != 3 {
			t.Errorf("captured state Priority = %d, want 3 (should not change)", capturedState.Priority)
		}
		if len(capturedState.Tags) != 1 || capturedState.Tags[0] != "original" {
			t.Errorf("captured state Tags = %v, want [original] (should not change)", capturedState.Tags)
		}
	})
}

func TestBuildEvaluationContext(t *testing.T) {
	t.Run("builds context with previous states", func(t *testing.T) {
		task1 := testutil.NewTestTaskWithStatus("Task 1", models.StatusTodo)
		task2 := testutil.NewTestTaskWithStatus("Task 2", models.StatusInProgress)

		prevTask1 := testutil.NewTestTaskWithStatus("Task 1", models.StatusInProgress)
		prevTask1.ID = task1.ID

		prevStates := map[string]*models.Task{
			task1.ID: prevTask1,
		}

		tasks := []*models.Task{task1, task2}
		ctx := BuildEvaluationContext(tasks, prevStates, "TestProject", "proj-123")

		if ctx == nil {
			t.Fatal("BuildEvaluationContext() returned nil")
		}
		if ctx.ProjectName != "TestProject" {
			t.Errorf("ctx.ProjectName = %s, want TestProject", ctx.ProjectName)
		}
		if ctx.ProjectID != "proj-123" {
			t.Errorf("ctx.ProjectID = %s, want proj-123", ctx.ProjectID)
		}
		if len(ctx.Changes) != 2 {
			t.Fatalf("ctx.Changes length = %d, want 2", len(ctx.Changes))
		}

		change1 := ctx.Changes[0]
		if change1.Task.ID != task1.ID {
			t.Errorf("change1.Task.ID = %s, want %s", change1.Task.ID, task1.ID)
		}
		if change1.Previous == nil {
			t.Error("change1.Previous should not be nil (has previous state)")
		} else if change1.Previous.Status != models.StatusInProgress {
			t.Errorf("change1.Previous.Status = %s, want %s", change1.Previous.Status, models.StatusInProgress)
		}

		change2 := ctx.Changes[1]
		if change2.Task.ID != task2.ID {
			t.Errorf("change2.Task.ID = %s, want %s", change2.Task.ID, task2.ID)
		}
		if change2.Previous != nil {
			t.Error("change2.Previous should be nil (new task)")
		}

		if len(ctx.AllTasks) != 2 {
			t.Errorf("ctx.AllTasks length = %d, want 2", len(ctx.AllTasks))
		}
		if len(ctx.TaskMap) != 2 {
			t.Errorf("ctx.TaskMap length = %d, want 2", len(ctx.TaskMap))
		}
		if ctx.TaskMap[task1.ID] != task1 {
			t.Error("ctx.TaskMap[task1.ID] does not reference task1")
		}
	})

	t.Run("handles empty previous states", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		tasks := []*models.Task{task}

		ctx := BuildEvaluationContext(tasks, map[string]*models.Task{}, "", "")

		if len(ctx.Changes) != 1 {
			t.Fatalf("ctx.Changes length = %d, want 1", len(ctx.Changes))
		}
		if ctx.Changes[0].Previous != nil {
			t.Error("ctx.Changes[0].Previous should be nil when no previous state")
		}
	})
}
