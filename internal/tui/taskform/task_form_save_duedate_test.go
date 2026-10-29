package taskform

import (
	"strings"
	"testing"
	"time"
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

func TestTaskForm_Save_RefusesSkippedDueClockWithoutLosingDraft(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	saved := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = saved })

	m := newTestTaskForm()
	m.titleInput.SetValue("Keep this draft")
	input := "2026-03-08 02:30"
	m.dueTextInput.SetValue(input)
	previous := time.Date(2026, time.March, 7, 9, 0, 0, 0, loc)
	m.draft.dueDate = &previous
	if cmd := m.handleSave(); cmd != nil {
		t.Fatal("an impossible due time dispatched a creation")
	}
	if m.err == nil || !strings.Contains(m.err.Error(), input) {
		t.Fatalf("expected a refusal naming %s, got %v", input, m.err)
	}
	if m.draft.title != "Keep this draft" || m.dueTextInput.Value() != input || m.draft.dueDate != &previous {
		t.Fatal("refusal changed the title, due input, or last resolved date")
	}
	if m.activeTab != formTabSchedule || m.focusIndex != 0 || m.submissionInFlight {
		t.Fatal("refusal must leave the due field focused and ready to correct")
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
