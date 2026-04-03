package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCopyTaskStatePreservesAllFields(t *testing.T) {
	dueDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:            "test-id",
		Status:        models.StatusInProgress,
		Priority:      2,
		Section:       "Next",
		CustomSection: "Sprint 1",
		Tags:          []string{"backend", "urgent"},
		Type:          models.TaskTypeBug,
		DueDate:       &dueDate,
		Description:   "Fix the login bug",
		Recurrence: &models.RecurrencePattern{
			Interval: 2,
			Unit:     models.RecurrenceWeekly,
			Weekdays: []int{1, 3, 5},
			Streak:   3,
		},
	}

	copied := copyTaskState(task)

	// Verify all fields are copied
	if copied.ID != task.ID {
		t.Errorf("ID: got %q, want %q", copied.ID, task.ID)
	}
	if copied.Status != task.Status {
		t.Errorf("Status: got %q, want %q", copied.Status, task.Status)
	}
	if copied.Priority != task.Priority {
		t.Errorf("Priority: got %d, want %d", copied.Priority, task.Priority)
	}
	if copied.Section != task.Section {
		t.Errorf("Section: got %q, want %q", copied.Section, task.Section)
	}
	if copied.CustomSection != task.CustomSection {
		t.Errorf("CustomSection: got %q, want %q", copied.CustomSection, task.CustomSection)
	}
	if copied.Type != task.Type {
		t.Errorf("Type: got %q, want %q", copied.Type, task.Type)
	}
	if copied.Description != task.Description {
		t.Errorf("Description: got %q, want %q", copied.Description, task.Description)
	}

	// DueDate should be the same pointer (it's a *time.Time, immutable value)
	if copied.DueDate == nil {
		t.Fatal("DueDate should not be nil")
	}
	if !copied.DueDate.Equal(*task.DueDate) {
		t.Errorf("DueDate: got %v, want %v", *copied.DueDate, *task.DueDate)
	}

	// Recurrence should be a deep copy
	if copied.Recurrence == nil {
		t.Fatal("Recurrence should not be nil")
	}
	if copied.Recurrence == task.Recurrence {
		t.Error("Recurrence should be a different pointer (deep copy)")
	}
	if copied.Recurrence.Interval != 2 {
		t.Errorf("Recurrence.Interval: got %d, want 2", copied.Recurrence.Interval)
	}
	if copied.Recurrence.Unit != models.RecurrenceWeekly {
		t.Errorf("Recurrence.Unit: got %q, want %q", copied.Recurrence.Unit, models.RecurrenceWeekly)
	}
	if len(copied.Recurrence.Weekdays) != 3 {
		t.Fatalf("Recurrence.Weekdays: got %d items, want 3", len(copied.Recurrence.Weekdays))
	}

	// Tags should be a deep copy
	if len(copied.Tags) != 2 {
		t.Fatalf("Tags: got %d items, want 2", len(copied.Tags))
	}
	// Mutating original tags should not affect copy
	task.Tags[0] = "mutated"
	if copied.Tags[0] == "mutated" {
		t.Error("Tags should be a deep copy, not shared slice")
	}

	// Mutating original recurrence weekdays should not affect copy
	task.Recurrence.Weekdays[0] = 99
	if copied.Recurrence.Weekdays[0] == 99 {
		t.Error("Recurrence.Weekdays should be a deep copy, not shared slice")
	}
}

func TestCopyTaskStateNilRecurrence(t *testing.T) {
	task := &models.Task{
		ID:     "test-id",
		Status: models.StatusTodo,
		Tags:   []string{},
	}

	copied := copyTaskState(task)
	if copied.Recurrence != nil {
		t.Error("Recurrence should be nil when original is nil")
	}
}

func TestCopyTaskStateNilTask(t *testing.T) {
	copied := copyTaskState(nil)
	if copied != nil {
		t.Error("copyTaskState(nil) should return nil")
	}
}

func TestTaskStateChangedDetectsCustomSection(t *testing.T) {
	current := &models.Task{
		ID:            "t1",
		Status:        models.StatusTodo,
		Priority:      3,
		Section:       "",
		CustomSection: "Sprint 2",
		Tags:          []string{"backend"},
		Type:          models.TaskTypeTask,
	}
	prev := &models.Task{
		ID:            "t1",
		Status:        models.StatusTodo,
		Priority:      3,
		Section:       "",
		CustomSection: "Sprint 1",
		Tags:          []string{"backend"},
		Type:          models.TaskTypeTask,
	}

	if !taskStateChanged(current, prev) {
		t.Error("taskStateChanged should detect CustomSection change from 'Sprint 1' to 'Sprint 2'")
	}
}

func TestTaskStateChangedDetectsDueDateChange(t *testing.T) {
	oldDue := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newDue := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

	current := &models.Task{
		ID:      "t1",
		Status:  models.StatusTodo,
		DueDate: &newDue,
		Tags:    []string{},
		Type:    models.TaskTypeTask,
	}
	prev := &models.Task{
		ID:      "t1",
		Status:  models.StatusTodo,
		DueDate: &oldDue,
		Tags:    []string{},
		Type:    models.TaskTypeTask,
	}

	if !taskStateChanged(current, prev) {
		t.Error("taskStateChanged should detect DueDate change")
	}
}

// TestDeleteRecurringStopRecurrence_KeepsTask verifies that pressing "s" on a
// delete-recurring confirmation stops recurrence but does NOT delete the task.
// This was a bug where "s" cleared recurrence then called executeDelete().
func TestDeleteRecurringStopRecurrence_KeepsTask(t *testing.T) {
	project := &models.Project{
		Name:        "Test",
		ID:          "test-proj",
		NextShortID: 2,
	}
	task, _ := models.NewTask("rec-task", 1, models.TaskTypeTask, "Recurring Task")
	task.Recurrence = &models.RecurrencePattern{
		Interval: 1,
		Unit:     models.RecurrenceDaily,
		Streak:   5,
	}
	project.Tasks = []*models.Task{task}

	// Simulate showing the delete-recurring confirm dialog
	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		project:           project,
	}
	m.confirm.Show("delete-recurring", "Delete recurring task?")
	m.deleteTarget = task

	// Press "s" to stop recurrence
	m.handleConfirmDialog("s")

	// Task should still exist in the project
	if len(project.Tasks) != 1 {
		t.Fatalf("expected 1 task (kept), got %d (task was deleted)", len(project.Tasks))
	}
	// Recurrence should be cleared
	if project.Tasks[0].Recurrence != nil {
		t.Error("recurrence should be nil after stop recurrence")
	}
	// Confirm dialog should be dismissed
	if m.confirm.IsActive() {
		t.Error("confirm dialog should be dismissed")
	}
	// deleteTarget should be cleared
	if m.deleteTarget != nil {
		t.Error("deleteTarget should be nil after stop recurrence")
	}
}

func TestTaskStateChangedDetectsDueDateAddedRemoved(t *testing.T) {
	due := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// DueDate added
	current := &models.Task{ID: "t1", Status: models.StatusTodo, DueDate: &due, Tags: []string{}, Type: models.TaskTypeTask}
	prev := &models.Task{ID: "t1", Status: models.StatusTodo, DueDate: nil, Tags: []string{}, Type: models.TaskTypeTask}

	if !taskStateChanged(current, prev) {
		t.Error("taskStateChanged should detect DueDate being added")
	}

	// DueDate removed
	current2 := &models.Task{ID: "t1", Status: models.StatusTodo, DueDate: nil, Tags: []string{}, Type: models.TaskTypeTask}
	prev2 := &models.Task{ID: "t1", Status: models.StatusTodo, DueDate: &due, Tags: []string{}, Type: models.TaskTypeTask}

	if !taskStateChanged(current2, prev2) {
		t.Error("taskStateChanged should detect DueDate being removed")
	}
}
