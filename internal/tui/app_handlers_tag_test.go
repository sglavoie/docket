package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestHandleBulkTagSelectorDoneAppliesAcrossProjects(t *testing.T) {
	app := newTestAppModel()
	undoDepth := 10
	app.config.UndoHistoryDepth = &undoDepth

	taskA := newBulkPickerTestTask(t, 1, "alpha task", "old")
	taskB := newBulkPickerTestTask(t, 1, "beta task", "other")
	projectA := newBulkPickerTestProject(t, "Alpha Tags", taskA)
	projectB := newBulkPickerTestProject(t, "Beta Tags", taskB)
	app.config.Projects = []*models.Project{projectA, projectB}

	app.allTasksView = NewAllTasksViewModel(app.config, app.configManager)
	app.allTasksView.multiSelectMode = true
	app.allTasksView.selectedTasks[taskA.ID] = true
	app.allTasksView.selectedTasks[taskB.ID] = true

	app.pushView(ViewBulkTagSelector)
	handled, cmd := app.handleTagMessages(BulkTagSelectorDoneMsg{
		TaskRefs: []TaskProjectRef{
			{Task: taskA, Project: projectA},
			{Task: taskB, Project: projectB},
		},
		SelectedTags: []string{"shared", "next"},
	})

	if !handled {
		t.Fatal("BulkTagSelectorDoneMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("expected synchronous save with nil cmd, got %T", cmd())
	}

	reloadedA := loadBulkPickerTestProject(t, projectA)
	reloadedB := loadBulkPickerTestProject(t, projectB)

	if got := reloadedA.GetTaskByID(taskA.ID).Tags; !reflect.DeepEqual(got, []string{"shared", "next"}) {
		t.Fatalf("projectA task tags = %v, want [shared next]", got)
	}
	if got := reloadedB.GetTaskByID(taskB.ID).Tags; !reflect.DeepEqual(got, []string{"shared", "next"}) {
		t.Fatalf("projectB task tags = %v, want [shared next]", got)
	}
	if app.statusMessage != "Tagged 2 tasks (u to undo)" {
		t.Fatalf("statusMessage = %q, want %q", app.statusMessage, "Tagged 2 tasks (u to undo)")
	}
	if app.allTasksView.multiSelectMode || app.allTasksView.SelectedCount() != 0 {
		t.Fatal("bulk tag completion should clear cached All Tasks multi-select")
	}
	assertBulkPickerUndoTaskIDs(t, projectA, []string{taskA.ID})
	assertBulkPickerUndoTaskIDs(t, projectB, []string{taskB.ID})
}
