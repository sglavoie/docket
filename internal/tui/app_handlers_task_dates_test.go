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
