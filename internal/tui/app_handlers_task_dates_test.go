package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func TestHandleTaskMessagesSetDueDate(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := SetDueDateMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SetDueDateMsg should be handled")
	}
	if app.currentView() != ViewDateInput {
		t.Errorf("currentView should be DateInput, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesSetDueDateNilTask(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := SetDueDateMsg{Task: nil, Project: nil}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SetDueDateMsg with nil task should still be handled")
	}
	if app.currentView() == ViewDateInput {
		t.Error("currentView should not be DateInput when task is nil")
	}
}

func TestHandleTaskMessagesSetSnoozeDate(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := SetSnoozeDateMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SetSnoozeDateMsg should be handled")
	}
	if app.currentView() != ViewSnoozeInput {
		t.Errorf("currentView should be SnoozeInput, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesSetSnoozeDateNil(t *testing.T) {
	app := newTestAppModel()

	msg := SetSnoozeDateMsg{Task: nil, Project: nil}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SetSnoozeDateMsg with nil task should still be handled")
	}
	if app.currentView() == ViewSnoozeInput {
		t.Error("currentView should not be SnoozeInput when task is nil")
	}
}

// Regression: bulk due-date change skipped saving the first task's project,
// losing in-memory mutations to other tasks in that same project.
func TestHandleDueDateChangedBulkSameProjectPersistsAllTasks(t *testing.T) {
	app := newTestAppModel()

	project, err := models.NewProject(uuid.New().String(), "Test", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task1, _ := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "first")
	task2, _ := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "second")
	task3, _ := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "third")
	project.Tasks = []*models.Task{task1, task2, task3}

	// Simulate DateInputModel having already saved task1 with the new date.
	dueDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	task1.DueDate = &dueDate

	app.pendingBulkTasks = []PaletteBulkTaskItem{
		{Task: task1, Project: project},
		{Task: task2, Project: project},
		{Task: task3, Project: project},
	}
	app.pushView(ViewDateInput)

	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{Task: task1})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd that triggers async save")
	}

	if task2.DueDate == nil || !task2.DueDate.Equal(dueDate) {
		t.Errorf("task2 should have due date %v, got %v", dueDate, task2.DueDate)
	}
	if task3.DueDate == nil || !task3.DueDate.Equal(dueDate) {
		t.Errorf("task3 should have due date %v, got %v", dueDate, task3.DueDate)
	}

	// The buggy path falls through to setSuccessStatus immediately when
	// projectsToSave is empty. With the fix the status is set later via
	// SaveCompleteMsg, so the field is empty right after the handler returns.
	if app.statusMessage != "" {
		t.Errorf("statusMessage should be empty until async save completes, got %q", app.statusMessage)
	}
}

func TestHandleDueDateChangedBulkCrossProjectPersistsAllTasks(t *testing.T) {
	app := newTestAppModel()

	taskA := newBulkPickerTestTask(t, 1, "alpha due")
	taskB := newBulkPickerTestTask(t, 1, "beta due")
	projectA := newBulkPickerTestProject(t, "Alpha Due", taskA)
	projectB := newBulkPickerTestProject(t, "Beta Due", taskB)

	dueDate := time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)
	taskA.DueDate = &dueDate
	taskA.HasTime = true

	app.pendingBulkTasks = []PaletteBulkTaskItem{
		{Task: taskA, Project: projectA},
		{Task: taskB, Project: projectB},
	}
	app.pushView(ViewDateInput)

	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{Task: taskA, Project: projectA})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}

	requireBulkPickerSaveComplete(t, runBulkPickerBatchCmd(t, cmd))

	reloadedA := loadBulkPickerTestProject(t, projectA)
	reloadedB := loadBulkPickerTestProject(t, projectB)
	assertTaskDueDate(t, reloadedA.GetTaskByID(taskA.ID), dueDate, true)
	assertTaskDueDate(t, reloadedB.GetTaskByID(taskB.ID), dueDate, true)
}

func TestHandleDueDateChangedBulkExplicitValueUsesWorkflowAndClearsSelection(t *testing.T) {
	app := newTestAppModel()
	undoDepth := 10
	app.config.UndoHistoryDepth = &undoDepth

	taskA := newBulkPickerTestTask(t, 1, "alpha explicit due")
	taskB := newBulkPickerTestTask(t, 1, "beta explicit due")
	projectA := newBulkPickerTestProject(t, "Alpha Explicit Due", taskA)
	projectB := newBulkPickerTestProject(t, "Beta Explicit Due", taskB)
	app.config.Projects = []*models.Project{projectA, projectB}

	app.allTasksView = NewAllTasksViewModel(app.config, app.configManager)
	app.allTasksView.multiSelectMode = true
	app.allTasksView.selectedTasks[taskA.ID] = true
	app.allTasksView.selectedTasks[taskB.ID] = true
	app.pendingBulkTasks = []PaletteBulkTaskItem{
		{Task: taskA, Project: projectA},
		{Task: taskB, Project: projectB},
	}
	app.pushView(ViewDateInput)

	dueDate := time.Date(2026, 7, 1, 16, 45, 0, 0, time.UTC)
	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{
		DueDate:          &dueDate,
		HasTime:          true,
		HasExplicitValue: true,
	})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}
	if cmd == nil {
		t.Fatal("expected status timeout command")
	}

	reloadedA := loadBulkPickerTestProject(t, projectA)
	reloadedB := loadBulkPickerTestProject(t, projectB)
	assertTaskDueDate(t, reloadedA.GetTaskByID(taskA.ID), dueDate, true)
	assertTaskDueDate(t, reloadedB.GetTaskByID(taskB.ID), dueDate, true)
	if app.allTasksView.multiSelectMode || app.allTasksView.SelectedCount() != 0 {
		t.Fatal("bulk due-date completion should clear cached All Tasks multi-select")
	}
	assertBulkPickerUndoTaskIDs(t, projectA, []string{taskA.ID})
	assertBulkPickerUndoTaskIDs(t, projectB, []string{taskB.ID})
}

func TestHandleTaskMessagesSnoozeDateChanged(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewSnoozeInput)

	msg := SnoozeDateChangedMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SnoozeDateChangedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after snooze date changed, got %v", app.currentView())
	}
}

func TestHandleSnoozeDateChangedBulkCrossProjectPersistsAllTasks(t *testing.T) {
	app := newTestAppModel()

	taskA := newBulkPickerTestTask(t, 1, "alpha snooze")
	taskB := newBulkPickerTestTask(t, 1, "beta snooze")
	projectA := newBulkPickerTestProject(t, "Alpha Snooze", taskA)
	projectB := newBulkPickerTestProject(t, "Beta Snooze", taskB)

	snoozeDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	taskA.SnoozedUntil = &snoozeDate

	app.pendingBulkTasks = []PaletteBulkTaskItem{
		{Task: taskA, Project: projectA},
		{Task: taskB, Project: projectB},
	}
	app.pushView(ViewSnoozeInput)

	handled, cmd := app.handleSnoozeDateChanged(SnoozeDateChangedMsg{Task: taskA, Project: projectA})
	if !handled {
		t.Fatal("SnoozeDateChangedMsg should be handled")
	}

	requireBulkPickerSaveComplete(t, runBulkPickerBatchCmd(t, cmd))

	reloadedA := loadBulkPickerTestProject(t, projectA)
	reloadedB := loadBulkPickerTestProject(t, projectB)
	assertTaskSnoozedUntil(t, reloadedA.GetTaskByID(taskA.ID), snoozeDate)
	assertTaskSnoozedUntil(t, reloadedB.GetTaskByID(taskB.ID), snoozeDate)
}

func assertTaskDueDate(t *testing.T, task *models.Task, want time.Time, wantHasTime bool) {
	t.Helper()
	if task == nil {
		t.Fatal("task not found")
	}
	if task.DueDate == nil {
		t.Fatal("DueDate is nil")
	}
	if got, expected := task.DueDate.Format("2006-01-02 15:04"), want.Format("2006-01-02 15:04"); got != expected {
		t.Fatalf("DueDate = %s, want %s", got, expected)
	}
	if task.HasTime != wantHasTime {
		t.Fatalf("HasTime = %v, want %v", task.HasTime, wantHasTime)
	}
}

func assertTaskSnoozedUntil(t *testing.T, task *models.Task, want time.Time) {
	t.Helper()
	if task == nil {
		t.Fatal("task not found")
	}
	if task.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil is nil")
	}
	if got, expected := task.SnoozedUntil.Format("2006-01-02"), want.Format("2006-01-02"); got != expected {
		t.Fatalf("SnoozedUntil = %s, want %s", got, expected)
	}
}
