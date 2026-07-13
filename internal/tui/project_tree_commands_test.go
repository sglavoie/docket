package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestCopyTaskStatePreservesAllFields(t *testing.T) {
	dueDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:            "test-id",
		WorkflowState: models.StateInProgress,
		Priority:      2,
		Tags:          []string{"backend", "urgent"},
		Type:          models.TaskTypeBug,
		DueDate:       &dueDate,
		Description:   "Fix the login bug",
		Recurrence: &models.RecurrencePattern{
			Interval:    2,
			Unit:        models.RecurrenceWeekly,
			Weekdays:    []int{1, 3, 5},
			Streak:      3,
			OriginalDay: 15,
			Mode:        models.RecurModeFromCompletion,
		},
	}

	copied := workflow.CopyTaskState(task)

	// Verify all fields are copied
	if copied.ID != task.ID {
		t.Errorf("ID: got %q, want %q", copied.ID, task.ID)
	}
	// CopyTaskState now carries the canonical WorkflowState rather than the
	// retired Status/Section fields (33.7.6.4), so the snapshot preserves the
	// task's lifecycle via State() (here in_progress, which outranks the Next
	// section under the state model).
	if copied.State() != task.State() {
		t.Errorf("State: got %q, want %q", copied.State(), task.State())
	}
	if copied.Priority != task.Priority {
		t.Errorf("Priority: got %d, want %d", copied.Priority, task.Priority)
	}
	if copied.Type != task.Type {
		t.Errorf("Type: got %q, want %q", copied.Type, task.Type)
	}
	if copied.Description != task.Description {
		t.Errorf("Description: got %q, want %q", copied.Description, task.Description)
	}

	// DueDate should be a deep copy (distinct pointer, same value)
	if copied.DueDate == nil {
		t.Fatal("DueDate should not be nil")
	}
	if !copied.DueDate.Equal(*task.DueDate) {
		t.Errorf("DueDate: got %v, want %v", *copied.DueDate, *task.DueDate)
	}
	if copied.DueDate == task.DueDate {
		t.Error("DueDate should be a distinct pointer (deep copy), not aliased to original")
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
	if copied.Recurrence.OriginalDay != 15 {
		t.Errorf("Recurrence.OriginalDay: got %d, want 15", copied.Recurrence.OriginalDay)
	}
	if copied.Recurrence.Mode != models.RecurModeFromCompletion {
		t.Errorf("Recurrence.Mode: got %q, want %q", copied.Recurrence.Mode, models.RecurModeFromCompletion)
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
		ID:            "test-id",
		WorkflowState: models.StateReady,
		Tags:          []string{},
	}

	copied := workflow.CopyTaskState(task)
	if copied.Recurrence != nil {
		t.Error("Recurrence should be nil when original is nil")
	}
}

func TestCopyTaskStateNilTask(t *testing.T) {
	copied := workflow.CopyTaskState(nil)
	if copied != nil {
		t.Error("workflow.CopyTaskState(nil) should return nil")
	}
}

func TestTaskStateChangedDetectsSection(t *testing.T) {
	current := &models.Task{
		ID:            "t1",
		WorkflowState: models.StateNext,
		Priority:      3,
		Tags:          []string{"backend"},
		Type:          models.TaskTypeTask,
	}
	prev := &models.Task{
		ID:            "t1",
		WorkflowState: models.StateInbox,
		Priority:      3,
		Tags:          []string{"backend"},
		Type:          models.TaskTypeTask,
	}

	if !workflow.TaskStateChanged(current, prev) {
		t.Error("taskStateChanged should detect Section change from Inbox to Next")
	}
}

func TestTaskStateChangedDetectsDueDateChange(t *testing.T) {
	oldDue := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newDue := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

	current := &models.Task{
		ID:            "t1",
		WorkflowState: models.StateReady,
		DueDate:       &newDue,
		Tags:          []string{},
		Type:          models.TaskTypeTask,
	}
	prev := &models.Task{
		ID:            "t1",
		WorkflowState: models.StateReady,
		DueDate:       &oldDue,
		Tags:          []string{},
		Type:          models.TaskTypeTask,
	}

	if !workflow.TaskStateChanged(current, prev) {
		t.Error("taskStateChanged should detect DueDate change")
	}
}

// TestDeleteRecurringStopRecurrence_KeepsTask verifies that pressing "s" on a
// delete-recurring confirmation stops recurrence but does NOT delete the task.
// This was a bug where "s" cleared recurrence then called executeDelete().
func TestDeleteRecurringStopRecurrence_KeepsTask(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{
		Name:        "Test",
		ID:          "test-proj",
		Slug:        "test-proj",
		NextShortID: 2,
		Path:        filepath.Join(dir, "projects", "test-proj"),
	}
	task, _ := models.NewTask("rec-task", 1, models.TaskTypeTask, "Recurring Task")
	task.Recurrence = &models.RecurrencePattern{
		Interval: 1,
		Unit:     models.RecurrenceDaily,
		Streak:   5,
	}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	// Simulate showing the delete-recurring confirm dialog. Stop-recurrence now
	// routes through the single-task coordinator (ledger-backed transaction), so
	// wire it over a disk-backed service and assert on committed state.
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
	m.SetTaskItemCoordinator(newTestSecondaryCoordinators(project).TaskItem)
	m.confirm.Show("delete-recurring", "Delete recurring task?")
	m.deleteTarget = task

	// Press "s" to stop recurrence and run the committed transaction.
	_, cmd := m.handleConfirmDialog("s")
	if cmd != nil {
		cmd()
	}

	// Task should still exist and recurrence cleared, on committed disk state.
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(fresh.Tasks) != 1 {
		t.Fatalf("expected 1 task (kept), got %d (task was deleted)", len(fresh.Tasks))
	}
	if fresh.Tasks[0].Recurrence != nil {
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

// TestRefreshViewClampsCursor verifies that refreshView clamps the cursor
// after rebuilding the tree, preventing out-of-bounds access when the
// visible list shrinks (e.g., after a task is filtered out or deleted).
func TestRefreshViewClampsCursor(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := &models.Project{
		Name:        "Test",
		ID:          "test-proj",
		NextShortID: 4,
	}
	t1, _ := models.NewTask("task-1", 1, models.TaskTypeTask, "Task 1")
	t2, _ := models.NewTask("task-2", 2, models.TaskTypeTask, "Task 2")
	t3, _ := models.NewTask("task-3", 3, models.TaskTypeTask, "Task 3")
	project.Tasks = []*models.Task{t1, t2, t3}
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, cfg, nil)
	m.SetSize(120, 40)

	// Cursor at last task (index 2)
	m.cursor = 2
	visible := m.visibleNodes()
	if len(visible) != 3 {
		t.Fatalf("expected 3 visible nodes, got %d", len(visible))
	}

	// Remove last two tasks, simulating deletion
	project.Tasks = []*models.Task{t1}
	project.BuildTaskTree()

	// refreshView should clamp cursor to 0 (only 1 item left)
	m.refreshView()
	visible = m.visibleNodes()
	if len(visible) != 1 {
		t.Fatalf("expected 1 visible node after delete, got %d", len(visible))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should be clamped to 0, got %d", m.cursor)
	}
}

// TestNewProjectTreeModelNilConfig verifies that NewProjectTreeModel does not
// panic when cfg is nil. Previously, cfg.TreeDefaultColumns was accessed
// outside the nil-check guard, causing a nil pointer dereference.
func TestNewProjectTreeModelNilConfig(t *testing.T) {
	project := &models.Project{
		Name:        "Test",
		ID:          "test-proj",
		NextShortID: 2,
	}
	task, _ := models.NewTask("t1", 1, models.TaskTypeTask, "Task 1")
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	// Should not panic with nil config
	m := NewProjectTreeModel(project, nil, nil)
	if m == nil {
		t.Fatal("NewProjectTreeModel returned nil")
	}
	visible := m.visibleNodes()
	if len(visible) != 1 {
		t.Errorf("expected 1 visible node, got %d", len(visible))
	}
}

func TestTaskStateChangedDetectsDueDateAddedRemoved(t *testing.T) {
	due := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// DueDate added
	current := &models.Task{ID: "t1", WorkflowState: models.StateReady, DueDate: &due, Tags: []string{}, Type: models.TaskTypeTask}
	prev := &models.Task{ID: "t1", WorkflowState: models.StateReady, DueDate: nil, Tags: []string{}, Type: models.TaskTypeTask}

	if !workflow.TaskStateChanged(current, prev) {
		t.Error("taskStateChanged should detect DueDate being added")
	}

	// DueDate removed
	current2 := &models.Task{ID: "t1", WorkflowState: models.StateReady, DueDate: nil, Tags: []string{}, Type: models.TaskTypeTask}
	prev2 := &models.Task{ID: "t1", WorkflowState: models.StateReady, DueDate: &due, Tags: []string{}, Type: models.TaskTypeTask}

	if !workflow.TaskStateChanged(current2, prev2) {
		t.Error("taskStateChanged should detect DueDate being removed")
	}
}
