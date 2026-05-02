package tui

import (
	"testing"
)

// ── Validation error handling ───────────────────────────────────────────────

func TestTaskForm_Save_EmptyTitle_SetsErrorJumpsToBasics(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = ""
	m.titleInput.SetValue("")
	_, cmd := m.Update(syntheticCtrlKey('s'))
	if cmd != nil {
		t.Error("expected nil Cmd on validation failure")
	}
	if m.err == nil {
		t.Error("expected m.err to be set after empty title save")
	}
	if m.activeTab != formTabBasics {
		t.Errorf("expected activeTab=Basics on title error, got %d", m.activeTab)
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex=0 on title error, got %d", m.focusIndex)
	}
}

func TestTaskForm_Save_InvalidDueDate_SetsErrorJumpsToSchedule(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = "valid title"
	m.titleInput.SetValue("valid title")
	m.draft.dueInput = "not-a-date-xyz"
	m.dueTextInput.SetValue("not-a-date-xyz")
	cmd := m.handleSave()
	if cmd != nil {
		t.Error("expected nil Cmd on due date parse failure")
	}
	if m.err == nil {
		t.Error("expected m.err to be set after bad due date")
	}
	if m.activeTab != formTabSchedule {
		t.Errorf("expected activeTab=Schedule on due-date error, got %d", m.activeTab)
	}
	if m.focusIndex != 0 {
		t.Errorf("expected focusIndex=0 on due-date error, got %d", m.focusIndex)
	}
}

func TestTaskForm_Save_CommitsTextInputsBeforeSave(t *testing.T) {
	m := newTestTaskForm()
	m.titleInput.SetValue("committed title")
	m.handleSave()
	if m.draft.title != "committed title" {
		t.Errorf("handleSave should commit titleInput; got %q", m.draft.title)
	}
}
