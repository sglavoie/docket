package tui

import (
	"testing"
	"time"

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

// Regression: the non-explicit multiselect due-date fallback (msg carries the
// already-updated first task, no explicit value) applies the first task's date to
// every selected task in the same project. Migrated onto the BulkCoordinator: one
// async transaction commits all three tasks on disk, and the workspace ledger
// restores the prior dates.
func TestHandleDueDateChangedBulkSameProjectPersistsAllTasks(t *testing.T) {
	task1 := newBulkPickerTestTask(t, 1, "first")
	task2 := newBulkPickerTestTask(t, 2, "second")
	task3 := newBulkPickerTestTask(t, 3, "third")
	initial := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	task1.DueDate = &initial
	task2.DueDate = &initial
	task3.DueDate = &initial
	env := newAppBulkPickerEnv(t, []*models.Task{task1, task2, task3})
	app := env.appModel()

	// Simulate DateInputModel having applied the new date to task1 in memory.
	dueDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)
	task1.DueDate = &dueDate

	app.pendingBulkTasks = env.refs(0, task1, task2, task3)
	app.pushView(ViewDateInput)

	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{Task: task1})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}
	if app.pendingBulkTasks != nil {
		t.Fatal("pending bulk tasks should be cleared once the batch is issued")
	}
	// The status line is set later via the async BatchTransactionResultMsg, not
	// synchronously in the handler.
	if app.statusMessage != "" {
		t.Errorf("statusMessage should be empty until the async batch commits, got %q", app.statusMessage)
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Set due date to 2026-06-15 for 3 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Set due date to 2026-06-15 for 3 tasks")
	}

	reloaded := env.reload(t, 0)
	assertTaskDueDate(t, reloaded.GetTaskByID(task1.ID), dueDate, false)
	assertTaskDueDate(t, reloaded.GetTaskByID(task2.ID), dueDate, false)
	assertTaskDueDate(t, reloaded.GetTaskByID(task3.ID), dueDate, false)

	env.undo(t)
	undone := env.reload(t, 0)
	assertTaskDueDate(t, undone.GetTaskByID(task1.ID), initial, false)
	assertTaskDueDate(t, undone.GetTaskByID(task2.ID), initial, false)
	assertTaskDueDate(t, undone.GetTaskByID(task3.ID), initial, false)
}

func TestHandleDueDateChangedBulkCrossProjectPersistsAllTasks(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha due")
	taskB := newBulkPickerTestTask(t, 1, "beta due")
	initialA := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	initialB := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	taskA.DueDate = &initialA
	taskB.DueDate = &initialB
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()

	// Simulate DateInputModel having applied the new timed date to taskA in memory.
	dueDate := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)
	taskA.DueDate = &dueDate
	taskA.HasTime = true

	app.pendingBulkTasks = append(env.refs(0, taskA), env.refs(1, taskB)...)
	app.pushView(ViewDateInput)

	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{Task: taskA, Project: env.projects[0]})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Set due date to 2026-06-15 for 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Set due date to 2026-06-15 for 2 tasks")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(taskA.ID), dueDate, true)
	assertTaskDueDate(t, env.reload(t, 1).GetTaskByID(taskB.ID), dueDate, true)

	env.undo(t)
	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(taskA.ID), initialA, false)
	assertTaskDueDate(t, env.reload(t, 1).GetTaskByID(taskB.ID), initialB, false)
}

// TestHandleDueDateChangedBulkExplicitValueMigratedCommitsAsync verifies the
// explicit-value multiselect due-date picker now commits one asynchronous
// cross-project transaction through the BulkCoordinator: the batch result
// carries the selection to clear and the per-view summary, both projects' due
// dates (with the timed flag preserved) are committed on disk, the shell clears
// the cross-project Outliner multi-select on commit, and the workspace ledger restores
// the prior due dates.
func TestHandleDueDateChangedBulkExplicitValueMigratedCommitsAsync(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha explicit due")
	taskB := newBulkPickerTestTask(t, 1, "beta explicit due")
	initialA := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	initialB := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	taskA.DueDate = &initialA
	taskB.DueDate = &initialB
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = env.projects
	app.crossOutliner.multiSelectMode = true
	app.crossOutliner.SelectTaskIdentity(taskIdentityFor(taskA, env.projects[0]))
	app.crossOutliner.SelectTaskIdentity(taskIdentityFor(taskB, env.projects[1]))
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)

	app.pendingBulkTasks = append(env.refs(0, taskA), env.refs(1, taskB)...)
	app.pushView(ViewDateInput)

	dueDate := time.Date(2026, 7, 1, 16, 45, 0, 0, time.Local)
	handled, cmd := app.handleDueDateChanged(DueDateChangedMsg{
		DueDate:          &dueDate,
		HasTime:          true,
		HasExplicitValue: true,
	})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}
	if app.pendingBulkTasks != nil {
		t.Fatal("pending bulk tasks should be cleared once the batch is issued")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Set due date to 2026-07-01 for 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Set due date to 2026-07-01 for 2 tasks")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(taskA.ID), dueDate, true)
	assertTaskDueDate(t, env.reload(t, 1).GetTaskByID(taskB.ID), dueDate, true)

	app.handleBatchTransactionResult(msg)
	if app.crossOutliner.multiSelectMode || app.crossOutliner.SelectedCount() != 0 {
		t.Fatal("committed batch should clear cross-project Outliner multi-select")
	}

	env.undo(t)
	assertTaskDueDate(t, env.reload(t, 0).GetTaskByID(taskA.ID), initialA, false)
	assertTaskDueDate(t, env.reload(t, 1).GetTaskByID(taskB.ID), initialB, false)
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

// Regression: closing the due-date picker from ViewCrossProjectOutliner left
// stale rows in m.crossOutliner because refreshAllViews() rebuilt m.outliner
// only. The expected post-fix behavior is that the task disappears from the
// cross-project rows once a due date is set under a DueDateNone filter.
func TestHandleDueDateChangedCrossProjectOutlinerRefreshesStaleRow(t *testing.T) {
	app := newTestAppModel()

	task := newBulkPickerTestTask(t, 1, "cross outliner no due date")
	project := newBulkPickerTestProject(t, "Cross Outliner Due", task)
	setTestAppProjects(app, project)

	crossOutliner := NewOutlinerModel(nil, app.configManager, app.runtime)
	crossOutliner.loadedProjects = []*models.Project{project}
	crossOutliner.activePerspective = PerspectiveAll
	crossOutliner.filter.DueDate = DueDateNone
	crossOutliner.crossForceRebuild = true
	crossOutliner.crossProjectRebuildRows()

	app.crossOutliner = crossOutliner
	app.views.Register(ViewCrossProjectOutliner, crossOutliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}

	before := collectCrossProjectTaskTitles(crossOutliner)
	if !before[task.Title] {
		t.Fatalf("setup: task %q should appear under DueDateNone, got rows %v", task.Title, before)
	}

	// Simulate DateInputModel having saved the new date before sending the
	// picker-close message.
	dueDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	task.DueDate = &dueDate

	app.pushView(ViewDateInput)
	handled, _ := app.handleDueDateChanged(DueDateChangedMsg{Task: task, Project: project})
	if !handled {
		t.Fatal("DueDateChangedMsg should be handled")
	}

	after := collectCrossProjectTaskTitles(crossOutliner)
	if after[task.Title] {
		t.Errorf("task %q should disappear from m.crossOutliner rows under DueDateNone after the due date is set; got rows %v", task.Title, after)
	}
}

// Regression: same shape as the due-date case but exercised through the
// SnoozeDateChangedMsg path. PerspectiveAll filters out snoozed tasks, so the
// row must disappear once SnoozedUntil is set in the future and the cross
// outliner is refreshed.
func TestHandleSnoozeDateChangedCrossProjectOutlinerRefreshesStaleRow(t *testing.T) {
	app := newTestAppModel()

	task := newBulkPickerTestTask(t, 1, "cross outliner active task")
	project := newBulkPickerTestProject(t, "Cross Outliner Snooze", task)
	setTestAppProjects(app, project)

	crossOutliner := NewOutlinerModel(nil, app.configManager, app.runtime)
	crossOutliner.loadedProjects = []*models.Project{project}
	crossOutliner.activePerspective = PerspectiveAll
	crossOutliner.crossForceRebuild = true
	crossOutliner.crossProjectRebuildRows()

	app.crossOutliner = crossOutliner
	app.views.Register(ViewCrossProjectOutliner, crossOutliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}

	before := collectCrossProjectTaskTitles(crossOutliner)
	if !before[task.Title] {
		t.Fatalf("setup: task %q should appear in PerspectiveAll, got rows %v", task.Title, before)
	}

	// Simulate SnoozeInputModel having saved the snooze before sending the
	// picker-close message. SnoozedUntil must be at least one day in the future
	// so models.Task.IsSnoozed() returns true.
	snoozeUntil := time.Now().AddDate(0, 0, 3)
	task.SnoozedUntil = &snoozeUntil

	app.pushView(ViewSnoozeInput)
	handled, _ := app.handleSnoozeDateChanged(SnoozeDateChangedMsg{Task: task, Project: project})
	if !handled {
		t.Fatal("SnoozeDateChangedMsg should be handled")
	}

	after := collectCrossProjectTaskTitles(crossOutliner)
	if after[task.Title] {
		t.Errorf("task %q should disappear from m.crossOutliner rows in PerspectiveAll after snooze; got rows %v", task.Title, after)
	}
}

func TestHandleSnoozeDateChangedBulkCrossProjectPersistsAllTasks(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha snooze")
	taskB := newBulkPickerTestTask(t, 1, "beta snooze")
	initialA := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	initialB := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	taskA.SnoozedUntil = &initialA
	taskB.SnoozedUntil = &initialB
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()

	// Simulate SnoozeInputModel having applied the new snooze to taskA in memory.
	snoozeDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.Local)
	taskA.SnoozedUntil = &snoozeDate

	app.pendingBulkTasks = append(env.refs(0, taskA), env.refs(1, taskB)...)
	app.pushView(ViewSnoozeInput)

	handled, cmd := app.handleSnoozeDateChanged(SnoozeDateChangedMsg{Task: taskA, Project: env.projects[0]})
	if !handled {
		t.Fatal("SnoozeDateChangedMsg should be handled")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Snoozed 2 tasks until 2026-06-20" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Snoozed 2 tasks until 2026-06-20")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	assertTaskSnoozedUntil(t, env.reload(t, 0).GetTaskByID(taskA.ID), snoozeDate)
	assertTaskSnoozedUntil(t, env.reload(t, 1).GetTaskByID(taskB.ID), snoozeDate)

	env.undo(t)
	assertTaskSnoozedUntil(t, env.reload(t, 0).GetTaskByID(taskA.ID), initialA)
	assertTaskSnoozedUntil(t, env.reload(t, 1).GetTaskByID(taskB.ID), initialB)
}

// TestHandleSnoozeDateChangedBulkExplicitValueMigratedCommitsAsync verifies the
// explicit-value multiselect snooze picker now commits one asynchronous
// cross-project transaction through the BulkCoordinator: the batch result
// carries the selection and per-view summary, both projects' snooze dates are
// committed on disk, and the workspace ledger restores the prior snooze.
func TestHandleSnoozeDateChangedBulkExplicitValueMigratedCommitsAsync(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha explicit snooze")
	taskB := newBulkPickerTestTask(t, 1, "beta explicit snooze")
	initialA := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	initialB := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)
	taskA.SnoozedUntil = &initialA
	taskB.SnoozedUntil = &initialB
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = env.projects
	app.crossOutliner.multiSelectMode = true
	app.crossOutliner.SelectTaskIdentity(taskIdentityFor(taskA, env.projects[0]))
	app.crossOutliner.SelectTaskIdentity(taskIdentityFor(taskB, env.projects[1]))
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)

	app.pendingBulkTasks = append(env.refs(0, taskA), env.refs(1, taskB)...)
	app.pushView(ViewSnoozeInput)

	snooze := time.Date(2026, 7, 5, 0, 0, 0, 0, time.Local)
	handled, cmd := app.handleSnoozeDateChanged(SnoozeDateChangedMsg{
		SnoozedUntil:     &snooze,
		HasExplicitValue: true,
	})
	if !handled {
		t.Fatal("SnoozeDateChangedMsg should be handled")
	}
	if app.pendingBulkTasks != nil {
		t.Fatal("pending bulk tasks should be cleared once the batch is issued")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Snoozed 2 tasks until 2026-07-05" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Snoozed 2 tasks until 2026-07-05")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}

	assertTaskSnoozedUntil(t, env.reload(t, 0).GetTaskByID(taskA.ID), snooze)
	assertTaskSnoozedUntil(t, env.reload(t, 1).GetTaskByID(taskB.ID), snooze)

	app.handleBatchTransactionResult(msg)
	if app.crossOutliner.multiSelectMode || app.crossOutliner.SelectedCount() != 0 {
		t.Fatal("committed batch should clear cross-project Outliner multi-select")
	}

	env.undo(t)
	assertTaskSnoozedUntil(t, env.reload(t, 0).GetTaskByID(taskA.ID), initialA)
	assertTaskSnoozedUntil(t, env.reload(t, 1).GetTaskByID(taskB.ID), initialB)
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
