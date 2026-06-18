package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newDirectMutationUndoTestApp(project *models.Project) *AppModel {
	app := newTestAppModel()
	depth := 10
	app.config.UndoHistoryDepth = &depth
	app.config.Projects = []*models.Project{project}
	app.config.CurrentProject = project.ID
	app.currentProject = project
	app.persister = NewProjectService()
	return app
}

func requireDirectMutationSave(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	msgs := runBulkPickerBatchCmd(t, cmd)
	requireBulkPickerSaveComplete(t, msgs)
}

func requireUndoRedoTask(t *testing.T, project *models.Project, taskID string, afterUndo, afterRedo func(*models.Task)) {
	t.Helper()

	action, err := storage.PerformUndo(project)
	if err != nil {
		t.Fatalf("PerformUndo: %v", err)
	}
	if action == "" {
		t.Fatal("PerformUndo returned empty action")
	}
	if err = storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after undo: %v", err)
	}
	restored := project.GetTaskByID(taskID)
	if restored == nil {
		t.Fatalf("task %q missing after undo", taskID)
	}
	afterUndo(restored)

	action, err = storage.PerformRedo(project)
	if err != nil {
		t.Fatalf("PerformRedo: %v", err)
	}
	if action == "" {
		t.Fatal("PerformRedo returned empty action")
	}
	if err = storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after redo: %v", err)
	}
	redone := project.GetTaskByID(taskID)
	if redone == nil {
		t.Fatalf("task %q missing after redo", taskID)
	}
	afterRedo(redone)
}

func TestHandleTaskContentEditedRecordsUndoRedoForTitle(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Old title")
	project := newBulkPickerTestProject(t, "Direct Mutation Title", task)
	app := newDirectMutationUndoTestApp(project)

	handled, cmd := app.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		NewTitle:       "New title",
		NewDescription: task.Description,
	})
	if !handled {
		t.Fatal("TaskContentEditedMsg was not handled")
	}
	requireDirectMutationSave(t, cmd)

	requireUndoRedoTask(t, project, task.ID, func(restored *models.Task) {
		if restored.Title != "Old title" {
			t.Fatalf("undo title = %q, want %q", restored.Title, "Old title")
		}
	}, func(redone *models.Task) {
		if redone.Title != "New title" {
			t.Fatalf("redo title = %q, want %q", redone.Title, "New title")
		}
	})
}

func TestHandleDescriptionEditorDoneRecordsUndoRedo(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Task with description")
	task.Description = "old description"
	project := newBulkPickerTestProject(t, "Direct Mutation Description", task)
	app := newDirectMutationUndoTestApp(project)

	handled, cmd := app.handleDescriptionEditorDone(DescriptionEditorDoneMsg{
		Task:           task,
		Project:        project,
		NewDescription: "new description",
	})
	if !handled {
		t.Fatal("DescriptionEditorDoneMsg was not handled")
	}
	requireDirectMutationSave(t, cmd)

	requireUndoRedoTask(t, project, task.ID, func(restored *models.Task) {
		if restored.Description != "old description" {
			t.Fatalf("undo description = %q, want %q", restored.Description, "old description")
		}
	}, func(redone *models.Task) {
		if redone.Description != "new description" {
			t.Fatalf("redo description = %q, want %q", redone.Description, "new description")
		}
	})
}

func TestHandleDependencySelectorDoneRecordsUndoRedo(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Blocked task")
	dependency := newBulkPickerTestTask(t, 2, "Dependency")
	project := newBulkPickerTestProject(t, "Direct Mutation Dependency", task, dependency)
	app := newDirectMutationUndoTestApp(project)
	app.pushView(ViewDependencySelector)

	handled, cmd := app.handleDependencyMessages(DependencySelectorDoneMsg{
		Task:       task,
		Project:    project,
		NewDepsOn:  []string{dependency.ID},
		HasChanges: true,
	})
	if !handled {
		t.Fatal("DependencySelectorDoneMsg was not handled")
	}
	requireDirectMutationSave(t, cmd)

	requireUndoRedoTask(t, project, task.ID, func(restored *models.Task) {
		if len(restored.DependsOn) != 0 {
			t.Fatalf("undo DependsOn = %v, want empty", restored.DependsOn)
		}
	}, func(redone *models.Task) {
		if len(redone.DependsOn) != 1 || redone.DependsOn[0] != dependency.ID {
			t.Fatalf("redo DependsOn = %v, want [%q]", redone.DependsOn, dependency.ID)
		}
	})
}

func TestHandlePriorityPickerDoneRecordsUndoRedo(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Prioritized task")
	task.Priority = 4
	project := newBulkPickerTestProject(t, "Direct Mutation Priority", task)
	app := newDirectMutationUndoTestApp(project)
	app.pushView(ViewPriorityPicker)

	handled, cmd := app.handleSectionMessages(PriorityPickerDoneMsg{
		Task:     task,
		Project:  project,
		Priority: 1,
	})
	if !handled {
		t.Fatal("PriorityPickerDoneMsg was not handled")
	}
	requireDirectMutationSave(t, cmd)

	requireUndoRedoTask(t, project, task.ID, func(restored *models.Task) {
		if restored.Priority != 4 {
			t.Fatalf("undo priority = %d, want %d", restored.Priority, 4)
		}
	}, func(redone *models.Task) {
		if redone.Priority != 1 {
			t.Fatalf("redo priority = %d, want %d", redone.Priority, 1)
		}
	})
}

func TestHandleSectionPickerDoneRecordsUndoRedo(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "Section task")
	project := newBulkPickerTestProject(t, "Direct Mutation Section", task)
	app := newDirectMutationUndoTestApp(project)
	app.pushView(ViewSectionPicker)

	handled, cmd := app.handleSectionMessages(SectionPickerDoneMsg{
		Task:    task,
		Project: project,
		Section: models.SectionNext,
	})
	if !handled {
		t.Fatal("SectionPickerDoneMsg was not handled")
	}
	requireDirectMutationSave(t, cmd)

	requireUndoRedoTask(t, project, task.ID, func(restored *models.Task) {
		if restored.Section != "" {
			t.Fatalf("undo section = %q, want empty", restored.Section)
		}
	}, func(redone *models.Task) {
		if redone.Section != models.SectionNext {
			t.Fatalf("redo section = %q, want %q", redone.Section, models.SectionNext)
		}
	})
}
