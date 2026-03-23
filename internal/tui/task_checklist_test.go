package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskChecklistSpaceTogglesItem(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Write regression test")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{{Text: "toggle me"}}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	updatedModel, cmd := model.Update(syntheticSpecialKey(tea.KeySpace))
	if cmd == nil {
		t.Fatal("expected toggle command for space key")
	}

	updatedChecklist, ok := updatedModel.(*TaskChecklistModel)
	if !ok {
		t.Fatalf("expected *TaskChecklistModel, got %T", updatedModel)
	}

	msg := cmd()
	if !updatedChecklist.task.Checklist[0].Done {
		t.Fatal("expected checklist item to be toggled done")
	}

	if _, ok := msg.(ChecklistItemToggledMsg); !ok {
		t.Fatalf("expected ChecklistItemToggledMsg, got %T", msg)
	}
	if updatedChecklist.project.Tasks[0].Checklist[0].Done != true {
		t.Fatal("expected project task checklist state to stay in sync")
	}
}
