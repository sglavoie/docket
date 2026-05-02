package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// ── Edit happy path ─────────────────────────────────────────────────────────

func TestTaskForm_Save_EditHappyPath_EmitsTaskUpdatedMsg(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	m.draft.title = "Updated Title"
	m.titleInput.SetValue("Updated Title")
	m.draft.priority = 1
	m.draft.desc = "Updated description"

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd on successful edit; err=%v", m.err)
	}
	msg := cmd()
	updated, ok := msg.(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", msg)
	}
	if updated.Task == nil {
		t.Fatal("TaskUpdatedMsg.Task should be non-nil")
	}
}

func TestTaskForm_Save_EditHappyPath_MutatesTaskInProject(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	taskID := m.task.ID
	m.draft.title = "Mutated Title"
	m.titleInput.SetValue("Mutated Title")
	m.draft.priority = 5
	m.draft.tags = []string{"mutated"}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	_ = cmd()

	var found *models.Task
	for _, t2 := range m.project.Tasks {
		if t2.ID == taskID {
			found = t2
			break
		}
	}
	if found == nil {
		t.Fatal("task not found in project after edit save")
	}
	if found.Title != "Mutated Title" {
		t.Errorf("expected Title='Mutated Title', got %q", found.Title)
	}
	if found.Priority != 5 {
		t.Errorf("expected Priority=5, got %d", found.Priority)
	}
}

func TestTaskForm_Save_EditFiltersBlankChecklistItems(t *testing.T) {
	m, cleanup := newSaveableEditForm(t)
	defer cleanup()

	m.draft.title = "Edited task"
	m.titleInput.SetValue("Edited task")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "keep me", Done: false},
		{Text: "   ", Done: false},
		{Text: "", Done: true},
		{Text: "also keep", Done: true},
	}

	cmd := m.handleSave()
	if cmd == nil {
		t.Fatalf("expected non-nil Cmd; err=%v", m.err)
	}
	msg := cmd()
	updated, ok := msg.(TaskUpdatedMsg)
	if !ok {
		t.Fatalf("expected TaskUpdatedMsg, got %T", msg)
	}
	if len(updated.Task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items (blanks filtered), got %d", len(updated.Task.Checklist))
	}
	if updated.Task.Checklist[0].Text != "keep me" {
		t.Errorf("expected first item 'keep me', got %q", updated.Task.Checklist[0].Text)
	}
	if updated.Task.Checklist[1].Text != "also keep" {
		t.Errorf("expected second item 'also keep', got %q", updated.Task.Checklist[1].Text)
	}
}
