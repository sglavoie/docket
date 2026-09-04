package taskform

import (
	"testing"
)

// ── Due date parsing ───────────────────────────────────────────────────────

func TestTaskForm_Save_ClearsDueDateOnNoneInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.title = "some task"
	m.draft.dueInput = "none"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.dueDate != nil {
		t.Error("parseDraftDueDate('none') should clear dueDate")
	}
}

func TestTaskForm_Save_ParsesDueDateFromInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing ISO date: %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if m.draft.dueDate.Year() != 2026 || int(m.draft.dueDate.Month()) != 6 || m.draft.dueDate.Day() != 15 {
		t.Errorf("parsed wrong date: got %v", m.draft.dueDate)
	}
}
