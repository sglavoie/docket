package models

import (
	"testing"
	"time"
)

func TestNewGenericTaskSeedsTheStateClock(t *testing.T) {
	task, err := NewGenericTask("id", 1, "Seeded")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	if task.StateChangedAt.IsZero() || !task.StateChangedAt.Equal(task.CreatedAt) {
		t.Fatalf("state clock = %v, want the creation instant %v", task.StateChangedAt, task.CreatedAt)
	}
}

func TestApplyStateTransitionAdvancesTheStateClock(t *testing.T) {
	task, err := NewGenericTask("id", 1, "Advancing")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	moved := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	if err := task.ApplyStateTransitionAt(StateWaiting, moved); err != nil {
		t.Fatalf("ApplyStateTransitionAt: %v", err)
	}
	if !task.StateChangedAt.Equal(moved) {
		t.Fatalf("state clock = %v, want %v", task.StateChangedAt, moved)
	}
}

func TestReapplyingTheSameStatePreservesTheStateClock(t *testing.T) {
	task, err := NewGenericTask("id", 1, "Waiting twice")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	moved := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	if err := task.ApplyStateTransitionAt(StateWaiting, moved); err != nil {
		t.Fatalf("first transition: %v", err)
	}
	later := moved.AddDate(0, 0, 12)
	if err := task.ApplyStateTransitionAt(StateWaiting, later); err != nil {
		t.Fatalf("second transition: %v", err)
	}
	if !task.StateChangedAt.Equal(moved) {
		t.Fatalf("state clock = %v, want the original transition %v", task.StateChangedAt, moved)
	}
	if !task.UpdatedAt.Equal(later) {
		t.Fatalf("updated = %v, want %v", task.UpdatedAt, later)
	}
}

func TestTouchingOtherFieldsLeavesTheStateClockAlone(t *testing.T) {
	task, err := NewGenericTask("id", 1, "Touched")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	moved := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	if err := task.ApplyStateTransitionAt(StateWaiting, moved); err != nil {
		t.Fatalf("ApplyStateTransitionAt: %v", err)
	}
	task.Pin()
	if !task.StateChangedAt.Equal(moved) {
		t.Fatalf("pinning moved the state clock to %v", task.StateChangedAt)
	}
	if !task.UpdatedAt.After(moved) {
		t.Fatalf("pinning did not bump updated: %v", task.UpdatedAt)
	}
}
