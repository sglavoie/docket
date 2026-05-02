package tui

import (
	"testing"
)

// ── Estimated days parsing ─────────────────────────────────────────────────

func TestTaskForm_Save_ClearsEstimatedDaysOnNoneInput(t *testing.T) {
	m := newTestTaskForm()
	m.draft.estimatedDays = 5
	m.draft.estimatedDaysInput = "none"
	if err := m.parseDraftEstimatedDays(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.estimatedDays != 0 {
		t.Error("parseDraftEstimatedDays('none') should reset estimatedDays to 0")
	}
}

func TestTaskForm_Save_ParsesEstimatedDaysFloat(t *testing.T) {
	m := newTestTaskForm()
	m.draft.estimatedDaysInput = "3.5"
	if err := m.parseDraftEstimatedDays(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.draft.estimatedDays != 3.5 {
		t.Errorf("expected estimatedDays=3.5, got %v", m.draft.estimatedDays)
	}
}

func TestTaskForm_Save_NegativeEstimatedDays_ReturnsError(t *testing.T) {
	m := newTestTaskForm()
	m.draft.estimatedDaysInput = "-1"
	if err := m.parseDraftEstimatedDays(); err == nil {
		t.Error("expected error for negative estimated days")
	}
}

func TestTaskForm_Save_InvalidEstimatedDays_ReturnsError(t *testing.T) {
	m := newTestTaskForm()
	m.draft.estimatedDaysInput = "abc"
	if err := m.parseDraftEstimatedDays(); err == nil {
		t.Error("expected error for non-numeric estimated days input")
	}
}
