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

	// Verify the toggle happens synchronously in Update(), not in the Cmd goroutine.
	// This is critical to avoid data races between Update/View goroutines.
	if !updatedChecklist.task.Checklist[0].Done {
		t.Fatal("expected checklist item to be toggled done synchronously before cmd() runs")
	}

	msg := cmd()

	if _, ok := msg.(ChecklistItemToggledMsg); !ok {
		t.Fatalf("expected ChecklistItemToggledMsg, got %T", msg)
	}
	if updatedChecklist.project.Tasks[0].Checklist[0].Done != true {
		t.Fatal("expected project task checklist state to stay in sync")
	}
}

func TestTaskChecklistSaveItemMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)

	// Enter add mode
	model.addMode = true
	model.textInput.SetValue("New item")

	// saveItem should mutate model state synchronously
	cmd := model.saveItem()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveItem")
	}

	// Verify state was mutated before cmd() runs
	if model.addMode {
		t.Fatal("expected addMode to be false after saveItem()")
	}
	if len(model.task.Checklist) != 1 {
		t.Fatalf("expected 1 checklist item, got %d", len(model.task.Checklist))
	}
	if model.task.Checklist[0].Text != "New item" {
		t.Fatalf("expected checklist item text 'New item', got %q", model.task.Checklist[0].Text)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}
}

func TestTaskChecklistDeleteMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{
		{Text: "item 1"},
		{Text: "item 2"},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	model.cursor = 1 // Point to second item

	// executeDelete should mutate model state synchronously
	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Verify state was mutated before cmd() runs
	if len(model.task.Checklist) != 1 {
		t.Fatalf("expected 1 checklist item after delete, got %d", len(model.task.Checklist))
	}
	if model.task.Checklist[0].Text != "item 1" {
		t.Fatalf("expected remaining item to be 'item 1', got %q", model.task.Checklist[0].Text)
	}
	// Cursor should be adjusted since we deleted the last item
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0 after deleting last item, got %d", model.cursor)
	}
}

func TestTaskChecklistReorderOOBAfterResolve(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{
		{Text: "item 1"},
		{Text: "item 2"},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	model.cursor = 0

	// Simulate external deletion that makes indices invalid
	task.Checklist = nil

	// reorderItem must not panic when indices are out of bounds after resolveTask
	cmd := model.reorderItem(0, 1)
	if cmd != nil {
		t.Fatal("expected nil command when indices are out of bounds")
	}
}

func TestTaskChecklistToggleOOBAfterResolve(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{{Text: "item"}}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	model.cursor = 0

	// Simulate external deletion
	task.Checklist = nil

	// toggleItem must not panic when cursor is out of bounds after resolveTask
	cmd := model.toggleItem()
	if cmd != nil {
		t.Fatal("expected nil command when cursor is out of bounds")
	}
}

func TestTaskChecklistDeleteOOBAfterResolve(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{{Text: "item"}}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	model.cursor = 0

	// Simulate external deletion
	task.Checklist = nil

	// executeDelete must not panic when cursor is out of bounds after resolveTask
	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil error command when cursor is out of bounds")
	}
}

func TestTaskChecklistEditExternalEditorFromEditModeOOB(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{{Text: "item"}}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	model.editMode = true
	model.editIndex = 0

	// Simulate external deletion
	task.Checklist = nil

	// Must not panic; should cancel edit mode and return error cmd
	cmd := model.editItemInExternalEditorFromEditMode()
	if cmd == nil {
		t.Fatal("expected non-nil error command when editIndex is out of bounds")
	}
	if model.editMode {
		t.Fatal("expected editMode to be canceled when item was deleted externally")
	}
}

func TestTaskChecklistSaveEditMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Checklist Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Checklist = []models.ChecklistItem{{Text: "original text", Done: true}}
	project.Tasks = []*models.Task{task}

	model := NewTaskChecklistModel(task, project)
	model.editMode = true
	model.editIndex = 0
	model.editTextInput.SetValue("edited text")

	// saveEditedItem should mutate model state synchronously
	cmd := model.saveEditedItem()
	if cmd == nil {
		t.Fatal("expected non-nil command from saveEditedItem")
	}

	// Verify state was mutated before cmd() runs
	if model.editMode {
		t.Fatal("expected editMode to be false after saveEditedItem()")
	}
	if model.task.Checklist[0].Text != "edited text" {
		t.Fatalf("expected item text 'edited text', got %q", model.task.Checklist[0].Text)
	}
	// Done state should be preserved
	if !model.task.Checklist[0].Done {
		t.Fatal("expected Done state to be preserved after edit")
	}
}
