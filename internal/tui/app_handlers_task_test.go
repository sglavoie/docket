package tui

import (
	"testing"
)

func TestHandleTaskMessagesCreateTask(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := CreateTaskMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskCreator {
		t.Errorf("currentView should be TaskCreator, got %v", app.currentView())
	}

	// Verify the project is still accessible
	if app.currentProject != project {
		t.Error("currentProject should still be set")
	}
}

func TestHandleTaskMessagesCreateTaskWithParent(t *testing.T) {
	app, project := newTestAppModelWithProject()
	parentTask := project.Tasks[0]

	msg := CreateTaskMsg{ParentTask: parentTask}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg with parent should be handled")
	}
}

func TestHandleTaskMessagesEditTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := EditTaskMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("EditTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskEditor {
		t.Errorf("currentView should be TaskEditor, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesTaskCreated(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.pushView(ViewTaskCreator)

	msg := TaskCreatedMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskCreatedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after task created, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesTaskDeleted(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := TaskDeletedMsg{Task: project.Tasks[0]}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskDeletedMsg should be handled")
	}
}

func TestHandleTaskMessagesTasksArchived(t *testing.T) {
	app := newTestAppModel()

	msg := TasksArchivedMsg{Count: 5}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TasksArchivedMsg should be handled")
	}
	if app.statusMessage != "Archived 5 task(s) to completed.md" {
		t.Errorf("statusMessage should contain archive count, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("TasksArchivedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesTaskStatusChanged(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := TaskStatusChangedMsg{Task: project.Tasks[0]}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskStatusChangedMsg should be handled")
	}
}

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

func TestHandleTaskMessagesManageDependencies(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ManageDependenciesMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ManageDependenciesMsg should be handled")
	}
	if app.currentView() != ViewDependencySelector {
		t.Errorf("currentView should be DependencySelector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesDependencySelectorDone(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	app.pushView(ViewDependencySelector)

	msg := DependencySelectorDoneMsg{HasChanges: false, Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DependencySelectorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowDependencyViewer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowDependencyViewerMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowDependencyViewerMsg should be handled")
	}
	if app.currentView() != ViewDependencyViewer {
		t.Errorf("currentView should be DependencyViewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseDependencyViewer(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDependencyViewer)

	msg := CloseDependencyViewerMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseDependencyViewerMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing viewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowUndoHistory(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowUndoHistoryMsg{Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowUndoHistoryMsg should be handled")
	}
	if app.currentView() != ViewUndoHistory {
		t.Errorf("currentView should be UndoHistory, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseUndoHistory(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewUndoHistory)

	msg := CloseUndoHistoryMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseUndoHistoryMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing undo history, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesUndoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewUndoHistory)

	msg := UndoPerformedMsg{Action: "Task deleted"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("UndoPerformedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after undo, got %v", app.currentView())
	}
	if app.statusMessage != "Undid: Task deleted" {
		t.Errorf("statusMessage should contain undo action, got %q", app.statusMessage)
	}
}

func TestHandleTaskMessagesCloseParentSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewParentSelector)

	msg := CloseParentSelectorMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseParentSelectorMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowTaskComments(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskCommentsMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskCommentsMsg should be handled")
	}
	if app.currentView() != ViewTaskComments {
		t.Errorf("currentView should be TaskComments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskComments(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskComments)

	msg := CloseTaskCommentsMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskCommentsMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing comments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommentAdded(t *testing.T) {
	app := newTestAppModel()

	msg := CommentAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentAddedMsg should be handled")
	}
	if app.statusMessage != "Comment added" {
		t.Errorf("statusMessage should be 'Comment added', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := CommentDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentDeletedMsg should be handled")
	}
	if app.statusMessage != "Comment deleted" {
		t.Errorf("statusMessage should be 'Comment deleted', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentEdited(t *testing.T) {
	app := newTestAppModel()

	msg := CommentEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentEditedMsg should be handled")
	}
	if app.statusMessage != "Comment updated" {
		t.Errorf("statusMessage should be 'Comment updated', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentEditedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCloseProjectSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectSelector)

	msg := CloseProjectSelectorMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseProjectSelectorMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommandPaletteTaskActionNilTask(t *testing.T) {
	app := newTestAppModel()

	msg := CommandPaletteTaskActionMsg{Task: nil, Project: nil, Action: "complete"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommandPaletteTaskActionMsg with nil task should be handled")
	}
}

func TestHandleTaskMessagesCommandPaletteBulkActionEmpty(t *testing.T) {
	app := newTestAppModel()

	msg := CommandPaletteBulkActionMsg{Tasks: nil, Action: "complete"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommandPaletteBulkActionMsg with empty tasks should be handled")
	}
}
