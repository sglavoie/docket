package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
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

func TestPruneDeletedTaskRefs(t *testing.T) {
	t.Run("removes deleted tasks from in-memory lists", func(t *testing.T) {
		app, project := newTestAppModelWithProject()

		// Ensure project has tasks and is loadable
		task1 := project.Tasks[0]
		task2 := project.Tasks[1]

		// Create a fake "deleted" task not in any project
		deletedTask := &models.Task{ID: "deleted-task-id", Title: "Deleted"}
		deletedProject := &models.Project{ID: "proj-deleted"}

		// Populate in-memory lists with both valid and stale entries
		app.focusQueue = []FocusTask{
			{Task: task1, Project: project},
			{Task: deletedTask, Project: deletedProject},
			{Task: task2, Project: project},
		}
		app.focusQueueIDs = map[string]bool{
			task1.ID:          true,
			"deleted-task-id": true,
			task2.ID:          true,
		}
		app.dailyPlan = []FocusTask{
			{Task: deletedTask, Project: deletedProject},
		}
		app.dailyPlanIDs = map[string]bool{
			"deleted-task-id": true,
		}
		app.recentTaskVisits = []RecentTaskVisitEntry{
			{Task: task1, Project: project},
			{Task: deletedTask, Project: deletedProject},
		}

		app.pruneDeletedTaskRefs()

		// Focus queue: deleted task should be removed
		if len(app.focusQueue) != 2 {
			t.Fatalf("expected 2 focus queue entries, got %d", len(app.focusQueue))
		}
		if app.focusQueueIDs["deleted-task-id"] {
			t.Error("deleted task should be removed from focusQueueIDs")
		}

		// Daily plan: deleted task should be removed
		if len(app.dailyPlan) != 0 {
			t.Fatalf("expected 0 daily plan entries, got %d", len(app.dailyPlan))
		}
		if app.dailyPlanIDs["deleted-task-id"] {
			t.Error("deleted task should be removed from dailyPlanIDs")
		}

		// Recent visits: deleted task should be removed
		if len(app.recentTaskVisits) != 1 {
			t.Fatalf("expected 1 recent visit entry, got %d", len(app.recentTaskVisits))
		}
	})

	t.Run("no-op when all tasks are valid", func(t *testing.T) {
		app, project := newTestAppModelWithProject()
		task1 := project.Tasks[0]

		app.focusQueue = []FocusTask{{Task: task1, Project: project}}
		app.focusQueueIDs = map[string]bool{task1.ID: true}
		app.dailyPlan = []FocusTask{}
		app.dailyPlanIDs = map[string]bool{}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task1, Project: project}}

		app.pruneDeletedTaskRefs()

		if len(app.focusQueue) != 1 {
			t.Errorf("expected 1 focus queue entry, got %d", len(app.focusQueue))
		}
		if len(app.recentTaskVisits) != 1 {
			t.Errorf("expected 1 recent visit entry, got %d", len(app.recentTaskVisits))
		}
	})
}

func TestTaskDeletedMsgCleansConfigRefs(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	// Set up a task that will be "deleted"
	deletedTask := &models.Task{ID: "will-be-deleted", Title: "Ghost"}
	app.focusQueue = []FocusTask{{Task: deletedTask, Project: project}}
	app.focusQueueIDs = map[string]bool{"will-be-deleted": true}
	app.dailyPlan = []FocusTask{{Task: deletedTask, Project: project}}
	app.dailyPlanIDs = map[string]bool{"will-be-deleted": true}

	msg := TaskDeletedMsg{Task: deletedTask}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Fatal("TaskDeletedMsg should be handled")
	}
	if len(app.focusQueue) != 0 {
		t.Errorf("expected focus queue to be pruned, got %d entries", len(app.focusQueue))
	}
	if len(app.dailyPlan) != 0 {
		t.Errorf("expected daily plan to be pruned, got %d entries", len(app.dailyPlan))
	}
}

func TestTasksArchivedMsgCleansConfigRefs(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProjectWithTasks("Test")
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	app := newTestAppModel()
	app.config = cfg
	app.currentProject = project

	// Simulate an archived task that no longer exists in project
	archivedTask := &models.Task{ID: "archived-task", Title: "Done"}
	app.focusQueue = []FocusTask{{Task: archivedTask, Project: project}}
	app.focusQueueIDs = map[string]bool{"archived-task": true}
	app.dailyPlan = []FocusTask{}
	app.dailyPlanIDs = map[string]bool{}

	msg := TasksArchivedMsg{Count: 1}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Fatal("TasksArchivedMsg should be handled")
	}
	if len(app.focusQueue) != 0 {
		t.Errorf("expected focus queue to be pruned after archive, got %d", len(app.focusQueue))
	}
}
