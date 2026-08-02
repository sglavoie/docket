package models

import (
	"testing"
	"time"
)

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		from      WorkflowState
		to        WorkflowState
		wantAllow bool
	}{
		{"ready→in_progress", StateBacklog, StateDoing, true},
		{"ready→done", StateBacklog, StateDone, true},
		{"ready→waiting", StateBacklog, StateWaiting, true},
		{"ready→ready", StateBacklog, StateBacklog, false},
		{"waiting→paused", StateWaiting, StatePaused, true},
		{"waiting→canceled", StateWaiting, StateCanceled, true},
		{"done→inbox", StateDone, StateInbox, true},
		{"done→ready", StateDone, StateBacklog, false},
		{"canceled→inbox", StateCanceled, StateInbox, true},
		{"canceled→done", StateCanceled, StateDone, false},
		{"ready→invalid", StateBacklog, WorkflowState("invalid"), false},
		{"invalid→ready", WorkflowState("invalid"), StateBacklog, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := CanTransitionState(tt.from, tt.to)
			if tt.wantAllow && msg != "" {
				t.Errorf("expected transition allowed, got %q", msg)
			}
			if !tt.wantAllow && msg == "" {
				t.Error("expected transition rejected, got allowed")
			}
		})
	}
}

func TestApplyTransition(t *testing.T) {
	tests := []struct {
		name       string
		target     WorkflowState
		wantStatus WorkflowState
	}{
		{"to in_progress", StateDoing, StateDoing},
		{"to done", StateDone, StateDone},
		{"to blocked", StateWaiting, StateWaiting},
		{"to canceled", StateCanceled, StateCanceled},
		{"to todo (reopen)", StateBacklog, StateBacklog},
		{"to standby", StatePaused, StatePaused},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := newLifecycleTestTask(t, "t1", 1)
			if err := task.ApplyStateTransition(tt.target); err != nil {
				t.Fatalf("ApplyStateTransition(%q): %v", tt.target, err)
			}
			if got := task.State(); got != tt.wantStatus {
				t.Errorf("expected %s, got %s", tt.wantStatus, got)
			}
		})
	}
}

func TestApplyTransitionRejectsInvalidState(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	before := task.State()

	if err := task.ApplyStateTransition(WorkflowState("invalid")); err == nil {
		t.Fatal("expected invalid state error")
	}

	if task.State() != before {
		t.Fatalf("invalid legacy status changed state to %q, want %q", task.State(), before)
	}
}

func TestApplyStateTransition(t *testing.T) {
	for _, state := range AllWorkflowStates() {
		state := state
		t.Run(state.String(), func(t *testing.T) {
			task := newLifecycleTestTask(t, "t1", 1)
			if err := task.ApplyStateTransition(state); err != nil {
				t.Fatalf("ApplyStateTransition(%q): %v", state, err)
			}
			if got := task.State(); got != state {
				t.Errorf("ApplyStateTransition(%s): State()=%s, want %s", state, got, state)
			}
			// Terminal states carry a completion timestamp; nonterminal ones must not.
			if state.IsTerminal() {
				if task.CompletedAt == nil {
					t.Errorf("terminal state %s: expected CompletedAt set, got nil", state)
				}
			} else if task.CompletedAt != nil {
				t.Errorf("nonterminal state %s: expected CompletedAt nil, got %v", state, task.CompletedAt)
			}
		})
	}
}

func TestApplyStateTransitionAtOwnsLifecycleSideEffectsForEveryState(t *testing.T) {
	now := time.Date(2026, 7, 25, 9, 30, 0, 0, time.UTC)
	staleCompletion := now.Add(-24 * time.Hour)
	snoozedUntil := now.Add(24 * time.Hour)

	for _, state := range AllWorkflowStates() {
		state := state
		t.Run(state.String(), func(t *testing.T) {
			task := newLifecycleTestTask(t, "t1", 1)
			task.CompletedAt = &staleCompletion
			task.SnoozedUntil = &snoozedUntil

			if err := task.ApplyStateTransitionAt(state, now); err != nil {
				t.Fatalf("ApplyStateTransitionAt(%q): %v", state, err)
			}
			if task.State() != state {
				t.Errorf("State() = %q, want %q", task.State(), state)
			}
			if !task.UpdatedAt.Equal(now) {
				t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, now)
			}
			if task.SnoozedUntil != nil {
				t.Errorf("SnoozedUntil = %v, want nil", task.SnoozedUntil)
			}
			if state.IsTerminal() {
				if task.CompletedAt == nil || !task.CompletedAt.Equal(now) {
					t.Errorf("CompletedAt = %v, want %v", task.CompletedAt, now)
				}
			} else if task.CompletedAt != nil {
				t.Errorf("CompletedAt = %v, want nil", task.CompletedAt)
			}
		})
	}
}

func TestApplyStateTransitionAtRejectsInvalidWithoutMutation(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	now := time.Date(2026, 7, 25, 9, 30, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	snoozedUntil := now.Add(time.Hour)
	task.WorkflowState = StateBacklog
	task.CompletedAt = &completedAt
	task.SnoozedUntil = &snoozedUntil
	task.UpdatedAt = completedAt

	if err := task.ApplyStateTransitionAt(WorkflowState("todo"), now); err == nil {
		t.Fatal("ApplyStateTransitionAt accepted invalid state")
	}
	if task.State() != StateBacklog {
		t.Errorf("State() = %q, want unchanged next", task.State())
	}
	if task.CompletedAt != &completedAt || task.SnoozedUntil != &snoozedUntil {
		t.Error("invalid transition changed lifecycle timestamps")
	}
	if !task.UpdatedAt.Equal(completedAt) {
		t.Error("invalid transition changed task metadata")
	}
}

// TestApplyStateTransitionReopenClearsTerminal proves the inbox transition acts
// as reopen: it clears the terminal timestamp and lands the task in the Inbox.
func TestApplyStateTransitionReopenClearsTerminal(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.Complete()
	if task.CompletedAt == nil {
		t.Fatal("precondition: Complete should set CompletedAt")
	}
	if err := task.ApplyStateTransition(StateInbox); err != nil {
		t.Fatalf("ApplyStateTransition(inbox): %v", err)
	}
	if task.State() != StateInbox {
		t.Errorf("expected StateInbox after reopen, got %s", task.State())
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt cleared after reopen-to-inbox")
	}
}

func TestApplyTransitionUsesLifecycleMethods(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateWaiting
	if err := task.ApplyStateTransition(StateDoing); err != nil {
		t.Fatalf("ApplyStateTransition(in_progress): %v", err)
	}
	if task.State() != StateDoing {
		t.Errorf("expected in-progress state, got %q", task.State())
	}

	task2 := newLifecycleTestTask(t, "t2", 2)
	if err := task2.ApplyStateTransition(StateDone); err != nil {
		t.Fatalf("ApplyStateTransition(done): %v", err)
	}
	if task2.CompletedAt == nil {
		t.Error("expected CompletedAt set (Complete lifecycle), got nil")
	}
	if task2.State() != StateDone {
		t.Errorf("expected Completed section, got %q", task2.State())
	}

	task3 := newLifecycleTestTask(t, "t3", 3)
	task3.Complete()
	if err := task3.ApplyStateTransition(StateBacklog); err != nil {
		t.Fatalf("ApplyStateTransition(ready): %v", err)
	}
	if task3.State() != StateBacklog {
		t.Errorf("expected ready state, got %q", task3.State())
	}
	if task3.CompletedAt != nil {
		t.Error("expected CompletedAt cleared after Reopen")
	}
}
