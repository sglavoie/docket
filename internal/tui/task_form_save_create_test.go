package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// ── Create happy path ───────────────────────────────────────────────────────

func TestTaskForm_Save_CreateHappyPath_EmitsTaskCreatedMsg(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "My New Task"
	m.titleInput.SetValue("My New Task")
	m.draft.priority = 2
	m.draft.tags = []string{"go", "tui"}
	m.draft.desc = "Task description"

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful create; err=%v", m.err)
	}
	msg := cmd()
	created, ok := msg.(TaskCreatedMsg)
	if !ok {
		t.Fatalf("expected TaskCreatedMsg, got %T", msg)
	}
	if created.Task == nil {
		t.Fatal("TaskCreatedMsg.Task should be non-nil")
	}
	if created.Task.Title != "My New Task" {
		t.Errorf("expected Title='My New Task', got %q", created.Task.Title)
	}
	if created.Task.Priority != 2 {
		t.Errorf("expected Priority=2, got %d", created.Task.Priority)
	}
	if len(created.Task.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", created.Task.Tags)
	}
	if created.Task.Description != "Task description" {
		t.Errorf("expected Description='Task description', got %q", created.Task.Description)
	}
}

func TestTaskForm_Save_CreateInitializesCollectionFields(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "Field init test"
	m.titleInput.SetValue("Field init test")

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	created := cmd().(TaskCreatedMsg)
	task := created.Task

	if task.DependsOn == nil {
		t.Error("DependsOn should be initialized to empty slice, got nil")
	}
	if task.RelatedTo == nil {
		t.Error("RelatedTo should be initialized to empty slice, got nil")
	}
	if task.ScoreDimensions == nil {
		t.Error("ScoreDimensions should be initialized to empty map, got nil")
	}
	if task.Children == nil {
		t.Error("Children should be initialized to empty slice, got nil")
	}
}

func TestTaskForm_Save_CreateSubtasksInitializeCollectionFields(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{{title: "child one", taskType: models.TaskTypeTask}}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	_ = cmd()

	var subtask *models.Task
	for _, t2 := range m.project.Tasks {
		if t2.Title == "child one" {
			subtask = t2
			break
		}
	}
	if subtask == nil {
		t.Fatal("subtask 'child one' not found in project tasks")
	}

	if subtask.DependsOn == nil {
		t.Error("subtask DependsOn should be initialized to empty slice, got nil")
	}
	if subtask.RelatedTo == nil {
		t.Error("subtask RelatedTo should be initialized to empty slice, got nil")
	}
	if subtask.ScoreDimensions == nil {
		t.Error("subtask ScoreDimensions should be initialized to empty map, got nil")
	}
	if subtask.Children == nil {
		t.Error("subtask Children should be initialized to empty slice, got nil")
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsChecklistItems(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.title = "Task with checklist"
	m.titleInput.SetValue("Task with checklist")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "step one", Done: false},
		{Text: "  ", Done: false},
		{Text: "step two", Done: true},
	}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	created := cmd().(TaskCreatedMsg)
	if len(created.Task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items (blank filtered), got %d", len(created.Task.Checklist))
	}
	if created.Task.Checklist[0].Text != "step one" {
		t.Errorf("expected first item 'step one', got %q", created.Task.Checklist[0].Text)
	}
	if !created.Task.Checklist[1].Done {
		t.Error("expected second checklist item Done=true")
	}
}

func TestTaskForm_Save_CreateHappyPath_AddsSubtasksForEpic(t *testing.T) {
	m, cleanup := newSaveableTestForm(t)
	defer cleanup()

	m.draft.taskType = models.TaskTypeEpic
	m.draft.title = "Parent Epic"
	m.titleInput.SetValue("Parent Epic")
	m.draft.subtasks = []formSubtaskDraft{
		{title: "child one", taskType: models.TaskTypeTask},
		{title: "", taskType: models.TaskTypeTask},
		{title: "child two", taskType: models.TaskTypeBug},
	}

	initialCount := len(m.project.Tasks)
	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	got := len(m.project.Tasks) - initialCount
	if got != 3 {
		t.Errorf("expected 3 new tasks (1 epic + 2 subtasks), got %d", got)
	}
}
