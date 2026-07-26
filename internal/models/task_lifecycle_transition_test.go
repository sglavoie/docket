package models

import (
	"testing"
	"time"
)

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		from      TaskStatus
		to        TaskStatus
		wantAllow bool
	}{
		{"todo→in_progress", StatusTodo, StatusInProgress, true},
		{"todo→done", StatusTodo, StatusDone, true},
		{"todo→blocked", StatusTodo, StatusBlocked, true},
		{"todo→canceled", StatusTodo, StatusCanceled, true},
		// A bare todo derives the ready state; the todo target maps to the
		// inbox (reopen) state, so this is a valid nonterminal move now.
		{"todo→todo", StatusTodo, StatusTodo, true},
		{"todo→review", StatusTodo, StatusReview, true},
		{"todo→standby", StatusTodo, StatusStandby, true},
		{"in_progress→done", StatusInProgress, StatusDone, true},
		{"in_progress→blocked", StatusInProgress, StatusBlocked, true},
		{"in_progress→canceled", StatusInProgress, StatusCanceled, true},
		{"in_progress→todo", StatusInProgress, StatusTodo, true},
		{"in_progress→review", StatusInProgress, StatusReview, true},
		{"in_progress→standby", StatusInProgress, StatusStandby, true},
		{"in_progress→in_progress", StatusInProgress, StatusInProgress, false},
		{"review→in_progress", StatusReview, StatusInProgress, true},
		{"review→done", StatusReview, StatusDone, true},
		{"review→blocked", StatusReview, StatusBlocked, true},
		{"review→canceled", StatusReview, StatusCanceled, true},
		{"review→todo", StatusReview, StatusTodo, true},
		{"review→standby", StatusReview, StatusStandby, true},
		{"review→review", StatusReview, StatusReview, false},
		{"standby→in_progress", StatusStandby, StatusInProgress, true},
		{"standby→done", StatusStandby, StatusDone, true},
		{"standby→blocked", StatusStandby, StatusBlocked, true},
		{"standby→canceled", StatusStandby, StatusCanceled, true},
		{"standby→todo", StatusStandby, StatusTodo, true},
		{"standby→review", StatusStandby, StatusReview, true},
		{"standby→standby", StatusStandby, StatusStandby, false},
		{"blocked→in_progress", StatusBlocked, StatusInProgress, true},
		{"blocked→done", StatusBlocked, StatusDone, true},
		{"blocked→blocked", StatusBlocked, StatusBlocked, false},
		{"blocked→canceled", StatusBlocked, StatusCanceled, true},
		{"blocked→todo", StatusBlocked, StatusTodo, true},
		{"blocked→review", StatusBlocked, StatusReview, true},
		{"blocked→standby", StatusBlocked, StatusStandby, true},
		{"done→todo", StatusDone, StatusTodo, true},
		{"done→in_progress", StatusDone, StatusInProgress, false},
		{"done→blocked", StatusDone, StatusBlocked, false},
		{"done→canceled", StatusDone, StatusCanceled, false},
		{"done→review", StatusDone, StatusReview, false},
		{"done→standby", StatusDone, StatusStandby, false},
		{"canceled→todo", StatusCanceled, StatusTodo, true},
		{"canceled→in_progress", StatusCanceled, StatusInProgress, false},
		{"canceled→done", StatusCanceled, StatusDone, false},
		{"canceled→blocked", StatusCanceled, StatusBlocked, false},
		{"canceled→review", StatusCanceled, StatusReview, false},
		{"canceled→standby", StatusCanceled, StatusStandby, false},
		{"todo→invalid", StatusTodo, TaskStatus("invalid"), false},
		// An unknown legacy status derives the fallback ready state, so the
		// transition is judged from ready and allowed.
		{"invalid→todo", TaskStatus("invalid"), StatusTodo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := newLifecycleTestTask(t, "t1", 1)
			task.WorkflowState = DeriveWorkflowState(tt.from, "", "")
			msg := task.CanTransitionTo(tt.to)
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
		target     TaskStatus
		wantStatus TaskStatus
	}{
		{"to in_progress", StatusInProgress, StatusInProgress},
		{"to done", StatusDone, StatusDone},
		{"to blocked", StatusBlocked, StatusBlocked},
		{"to canceled", StatusCanceled, StatusCanceled},
		{"to todo (reopen)", StatusTodo, StatusTodo},
		{"to review", StatusReview, StatusReview},
		{"to standby", StatusStandby, StatusStandby},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := newLifecycleTestTask(t, "t1", 1)
			task.ApplyTransition(tt.target)
			if got := LegacyStatusForState(task.State()); got != tt.wantStatus {
				t.Errorf("expected %s, got %s", tt.wantStatus, got)
			}
		})
	}
}

func TestApplyTransitionRejectsInvalidLegacyStatus(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	before := task.State()

	task.ApplyTransition(TaskStatus(StateNext))

	if task.State() != before {
		t.Fatalf("invalid legacy status changed state to %q, want %q", task.State(), before)
	}
}

func TestApplyStateTransition(t *testing.T) {
	for _, state := range AllWorkflowStates() {
		state := state
		t.Run(state.String(), func(t *testing.T) {
			task := newLifecycleTestTask(t, "t1", 1)
			task.ApplyStateTransition(state)
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
			task.Dimmed = true
			if state == StateDone {
				parkedAt := now.Add(-time.Hour)
				task.ParkedAt = &parkedAt
				task.ParkedNote = "resume here"
			}

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
			if state == StateInbox && task.Dimmed {
				t.Error("inbox transition should clear Dimmed")
			}
			if state != StateInbox && !task.Dimmed {
				t.Errorf("%q transition unexpectedly cleared Dimmed", state)
			}
			if state == StateDone && task.IsParked() {
				t.Error("done transition should clear parking")
			}
		})
	}
}

func TestApplyStateTransitionAtRejectsInvalidWithoutMutation(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	now := time.Date(2026, 7, 25, 9, 30, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	snoozedUntil := now.Add(time.Hour)
	task.WorkflowState = StateNext
	task.CompletedAt = &completedAt
	task.SnoozedUntil = &snoozedUntil
	task.Dimmed = true
	task.UpdatedAt = completedAt

	if err := task.ApplyStateTransitionAt(WorkflowState("todo"), now); err == nil {
		t.Fatal("ApplyStateTransitionAt accepted invalid state")
	}
	if task.State() != StateNext {
		t.Errorf("State() = %q, want unchanged next", task.State())
	}
	if task.CompletedAt != &completedAt || task.SnoozedUntil != &snoozedUntil {
		t.Error("invalid transition changed lifecycle timestamps")
	}
	if !task.UpdatedAt.Equal(completedAt) || !task.Dimmed {
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
	task.ApplyStateTransition(StateInbox)
	if task.State() != StateInbox {
		t.Errorf("expected StateInbox after reopen, got %s", task.State())
	}
	if task.CompletedAt != nil {
		t.Error("expected CompletedAt cleared after reopen-to-inbox")
	}
}

func TestApplyTransitionUsesLifecycleMethods(t *testing.T) {
	task := newLifecycleTestTask(t, "t1", 1)
	task.WorkflowState = StateReview
	task.ApplyTransition(StatusInProgress)
	if SectionForState(task.State()) != "" {
		t.Errorf("expected section cleared (Start auto-triage), got %q", SectionForState(task.State()))
	}

	task2 := newLifecycleTestTask(t, "t2", 2)
	task2.ApplyTransition(StatusDone)
	if task2.CompletedAt == nil {
		t.Error("expected CompletedAt set (Complete lifecycle), got nil")
	}
	if SectionForState(task2.State()) != SectionCompleted {
		t.Errorf("expected Completed section, got %q", SectionForState(task2.State()))
	}

	task3 := newLifecycleTestTask(t, "t3", 3)
	task3.Complete()
	task3.ApplyTransition(StatusTodo)
	if SectionForState(task3.State()) != SectionInbox {
		t.Errorf("expected Inbox section (Reopen lifecycle), got %q", SectionForState(task3.State()))
	}
	if task3.CompletedAt != nil {
		t.Error("expected CompletedAt cleared after Reopen")
	}
}
