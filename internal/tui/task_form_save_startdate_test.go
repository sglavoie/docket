package tui

import (
	"testing"
)

// ── Start date parsing ─────────────────────────────────────────────────────

func TestTaskForm_Save_ClearsStartDateOnNoneInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.startDateInput = "none"
	if err := m.parseDraftStartDate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.startDate != nil {
		t.Error("parseDraftStartDate('none') should clear startDate")
	}
}

func TestTaskForm_Save_ParsesStartDateFromISO(t *testing.T) {
	m := newTestTaskForm()
	m.draft.startDateInput = "2026-08-01"
	if err := m.parseDraftStartDate(); err != nil {
		t.Fatalf("unexpected error parsing ISO start date: %v", err)
	}
	if m.draft.startDate == nil {
		t.Fatal("expected startDate to be set")
	}
	if m.draft.startDate.Year() != 2026 || int(m.draft.startDate.Month()) != 8 || m.draft.startDate.Day() != 1 {
		t.Errorf("parsed wrong start date: got %v", m.draft.startDate)
	}
}

func TestTaskForm_Save_InvalidStartDate_ReturnsError(t *testing.T) {
	m := newTestTaskForm()
	m.draft.startDateInput = "not-a-date-xyz"
	if err := m.parseDraftStartDate(); err == nil {
		t.Error("expected error for invalid start date input")
	}
}
