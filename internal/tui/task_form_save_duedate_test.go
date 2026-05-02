package tui

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

func TestTaskForm_Save_ParsesDueDateWithTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15T14:30"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing date+time: %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if !m.draft.hasTime {
		t.Error("expected hasTime=true for input with time component")
	}
	if m.draft.dueDate.Hour() != 14 || m.draft.dueDate.Minute() != 30 {
		t.Errorf("expected 14:30, got %02d:%02d", m.draft.dueDate.Hour(), m.draft.dueDate.Minute())
	}
}

func TestTaskForm_Save_ParsesDueDateWithTimeSuffix(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "tomorrow 5pm"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error parsing 'tomorrow 5pm': %v", err)
	}
	if m.draft.dueDate == nil {
		t.Fatal("expected dueDate to be set")
	}
	if !m.draft.hasTime {
		t.Error("expected hasTime=true for 'tomorrow 5pm'")
	}
	if m.draft.dueDate.Hour() != 17 {
		t.Errorf("expected hour=17, got %d", m.draft.dueDate.Hour())
	}
}

func TestTaskForm_Save_DateOnlyDoesNotSetHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.dueInput = "2026-06-15"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.hasTime {
		t.Error("expected hasTime=false for date-only input")
	}
}

func TestTaskForm_Save_ClearDueDateClearsHasTime(t *testing.T) {
	m := newTestTaskForm()
	m.draft.hasTime = true
	m.draft.dueInput = "none"
	if err := m.parseDraftDueDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.dueDate != nil {
		t.Error("expected dueDate=nil")
	}
	if m.draft.hasTime {
		t.Error("expected hasTime=false after clearing due date")
	}
}
