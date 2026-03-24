package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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

func TestHandleTaskMessagesShowTaskDetail(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskDetailMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskDetailMsg should be handled")
	}
	if app.currentView() != ViewTaskDetail {
		t.Errorf("currentView should be TaskDetail, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskDetail(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskDetail)

	msg := CloseTaskDetailMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskDetailMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing detail, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowTaskChecklist(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskChecklistMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskChecklistMsg should be handled")
	}
	if app.currentView() != ViewTaskChecklist {
		t.Errorf("currentView should be TaskChecklist, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskChecklist(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskChecklist)

	msg := CloseTaskChecklistMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskChecklistMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing checklist, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesChecklistItemAdded(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemAddedMsg should be handled")
	}
	if app.statusMessage != "Checklist item added" {
		t.Errorf("statusMessage should be 'Checklist item added', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("ChecklistItemAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemDeletedMsg should be handled")
	}
	if app.statusMessage != "Checklist item deleted" {
		t.Errorf("statusMessage should be 'Checklist item deleted', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("ChecklistItemDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemEdited(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemEditedMsg should be handled")
	}
	if app.statusMessage != "Checklist item updated" {
		t.Errorf("statusMessage should be 'Checklist item updated', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("ChecklistItemEditedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemToggled(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemToggledMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemToggledMsg should be handled")
	}
}

func TestHandleTaskMessagesOpenTaskActivity(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := OpenTaskActivityMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("OpenTaskActivityMsg should be handled")
	}
	if app.currentView() != ViewTaskActivity {
		t.Errorf("currentView should be TaskActivity, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskActivity(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskActivity)

	msg := CloseTaskActivityMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskActivityMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing activity, got %v", app.currentView())
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

func TestHandleTaskMessagesCreateLinkedTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateLinkedTaskMsg should be handled")
	}
	if app.currentView() != ViewLinkedTaskCreator {
		t.Errorf("currentView should be LinkedTaskCreator, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseLinkedTaskSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewLinkedTaskSelector)

	msg := CloseLinkedTaskSelectorMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseLinkedTaskSelectorMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowCrossProjectLinker(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowCrossProjectLinkerMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectLinker {
		t.Errorf("currentView should be CrossProjectLinker, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseCrossProjectLinker(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewCrossProjectLinker)

	msg := CloseCrossProjectLinkerMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseCrossProjectLinkerMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing linker, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowProjectSelectorForMove(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowProjectSelectorForMoveMsg should be handled")
	}
	if app.currentView() != ViewProjectSelector {
		t.Errorf("currentView should be ProjectSelector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommentResolved(t *testing.T) {
	app := newTestAppModel()

	msg := CommentResolvedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentResolvedMsg should be handled")
	}
	if app.statusMessage != "Comment resolved" {
		t.Errorf("statusMessage should be 'Comment resolved', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentResolvedMsg should return a status timeout command")
	}
}

// --- Quick note overlay tests ---

func TestOpenQuickNoteMsgCreatesOverlay(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := OpenQuickNoteMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("OpenQuickNoteMsg should be handled")
	}
	if app.quickNoteOverlay == nil {
		t.Error("quickNoteOverlay should be set after OpenQuickNoteMsg")
	}
}

func TestOpenQuickNoteMsgNilTask(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := OpenQuickNoteMsg{Task: nil, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("OpenQuickNoteMsg with nil task should still be handled")
	}
	if app.quickNoteOverlay != nil {
		t.Error("quickNoteOverlay should not be set when task is nil")
	}
}

func TestCommentAddedMsgClearsOverlay(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	// Simulate overlay being open
	app.quickNoteOverlay = newQuickNoteOverlay(task, project)

	msg := CommentAddedMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentAddedMsg should be handled")
	}
	if app.quickNoteOverlay != nil {
		t.Error("quickNoteOverlay should be cleared after CommentAddedMsg")
	}
}

func TestQuickNoteOverlayEscDismisses(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	app.quickNoteOverlay = newQuickNoteOverlay(task, project)

	if app.quickNoteOverlay == nil {
		t.Fatal("overlay should be set before test")
	}

	_, dismiss, _ := app.quickNoteOverlay.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !dismiss {
		t.Error("Esc should set dismiss=true")
	}
}

func TestAppendCommentAddsComment(t *testing.T) {
	_, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	initialCount := len(task.Comments)

	result, err := appendComment(project, task.ID, "  hello  ")
	if err != nil {
		t.Fatalf("appendComment should not error: %v", err)
	}
	if result == nil {
		t.Fatal("appendComment should return the task")
	}
	if len(result.Comments) != initialCount+1 {
		t.Errorf("expected %d comments, got %d", initialCount+1, len(result.Comments))
	}
	if result.Comments[len(result.Comments)-1].Text != "hello" {
		t.Errorf("comment text should be trimmed, got %q", result.Comments[len(result.Comments)-1].Text)
	}
}

func TestAppendCommentEmptyTextReturnsError(t *testing.T) {
	_, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	_, err := appendComment(project, task.ID, "   ")
	if err == nil {
		t.Error("appendComment with empty text should return error")
	}
}

func TestAppendCommentUnknownTaskReturnsError(t *testing.T) {
	_, project := newTestAppModelWithProject()

	_, err := appendComment(project, "nonexistent-id", "some text")
	if err == nil {
		t.Error("appendComment with unknown task ID should return error")
	}
}
