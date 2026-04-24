package cmd

import (
	"testing"
	"time"

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

	t.Run("deep copies DueDate pointer", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		due := time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local)
		task.DueDate = &due

		states := CaptureTaskStates([]*models.Task{task})
		state := states[task.ID]

		if state.DueDate == nil {
			t.Fatal("captured state DueDate should not be nil")
		}
		if !state.DueDate.Equal(due) {
			t.Errorf("captured DueDate = %v, want %v", *state.DueDate, due)
		}

		// Mutating the original task's DueDate should not affect the captured state
		newDue := time.Date(2025, 12, 25, 0, 0, 0, 0, time.Local)
		task.DueDate = &newDue

		if !state.DueDate.Equal(due) {
			t.Errorf("captured DueDate changed to %v after original mutation, want %v", *state.DueDate, due)
		}

		// Also verify the pointers are distinct (not aliased)
		if state.DueDate == task.DueDate {
			t.Error("captured DueDate should be a distinct pointer, not aliased to the original")
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

	t.Run("excludes unchanged tasks from Changes", func(t *testing.T) {
		changed := testutil.NewTestTaskWithStatus("Changed", models.StatusDone)
		unchanged := testutil.NewTestTaskWithStatus("Unchanged", models.StatusTodo)

		// Previous state: changed was Todo, unchanged was Todo
		prevChanged := testutil.NewTestTaskWithStatus("Changed", models.StatusTodo)
		prevChanged.ID = changed.ID
		prevUnchanged := testutil.NewTestTaskWithStatus("Unchanged", models.StatusTodo)
		prevUnchanged.ID = unchanged.ID

		prevStates := map[string]*models.Task{
			changed.ID:   prevChanged,
			unchanged.ID: prevUnchanged,
		}

		tasks := []*models.Task{changed, unchanged}
		ctx := BuildEvaluationContext(tasks, prevStates, "Test", "t1")

		// Only the changed task should be in Changes
		if len(ctx.Changes) != 1 {
			t.Fatalf("ctx.Changes length = %d, want 1 (unchanged task should be excluded)", len(ctx.Changes))
		}
		if ctx.Changes[0].Task.ID != changed.ID {
			t.Errorf("ctx.Changes[0].Task.ID = %s, want %s", ctx.Changes[0].Task.ID, changed.ID)
		}

		// AllTasks and TaskMap should still include all tasks
		if len(ctx.AllTasks) != 2 {
			t.Errorf("ctx.AllTasks length = %d, want 2", len(ctx.AllTasks))
		}
		if len(ctx.TaskMap) != 2 {
			t.Errorf("ctx.TaskMap length = %d, want 2", len(ctx.TaskMap))
		}
	})
}

func TestCmdTaskStateChanged(t *testing.T) {
	t.Run("nil prev means changed", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		if !cmdTaskStateChanged(task, nil) {
			t.Error("expected changed when prev is nil")
		}
	})

	t.Run("same state means unchanged", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Priority = 3
		task.Tags = []string{"a", "b"}
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		prev.Priority = 3
		prev.Tags = []string{"a", "b"}
		if cmdTaskStateChanged(task, prev) {
			t.Error("expected unchanged when all fields match")
		}
	})

	t.Run("status change detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusDone)
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when status differs")
		}
	})

	t.Run("priority change detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Priority = 1
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		prev.Priority = 3
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when priority differs")
		}
	})

	t.Run("tag change detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Tags = []string{"a", "b", "c"}
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		prev.Tags = []string{"a", "b"}
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when tags differ")
		}
	})

	t.Run("due date added detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		now := time.Now()
		task.DueDate = &now
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when due date added")
		}
	})

	t.Run("due date removed detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		now := time.Now()
		prev.DueDate = &now
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when due date removed")
		}
	})

	t.Run("section change detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Section = models.SectionNext
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		prev.Section = models.SectionInbox
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when section differs")
		}
	})

	t.Run("type change detected", func(t *testing.T) {
		task := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		task.Type = models.TaskTypeBug
		prev := testutil.NewTestTaskWithStatus("Task", models.StatusTodo)
		prev.ID = task.ID
		prev.Type = models.TaskTypeTask
		if !cmdTaskStateChanged(task, prev) {
			t.Error("expected changed when type differs")
		}
	})
}
