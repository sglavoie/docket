package models

import (
	"testing"
	"time"
)

// TestTaskAwaitsReviewAdmitsAndRefuses states the admission set once, for the
// one predicate every surface reads. The three ways in are independent, and the
// interesting rows are the ones where two axes disagree: a deferred task that
// was flagged is in, and a deferred task that is merely old is out.
func TestTaskAwaitsReviewAdmitsAndRefuses(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	fresh := now.AddDate(0, 0, -1)
	old := now.AddDate(0, 0, -(StaleTaskThresholdDays + 10))
	later := now.Add(72 * time.Hour)

	build := func(seed func(*Task)) *Task {
		task, err := NewGenericTask("t1", 1, "Chase the vendor")
		if err != nil {
			t.Fatalf("NewGenericTask: %v", err)
		}
		task.UpdatedAt = fresh
		seed(task)
		return task
	}

	for _, testCase := range []struct {
		name string
		task *Task
		want bool
	}{
		{"nil", nil, false},
		{"fresh open work", build(func(*Task) {}), false},
		{"flagged", build(func(task *Task) { task.Review = true }), true},
		{"stale", build(func(task *Task) { task.UpdatedAt = old }), true},
		{"waiting", build(func(task *Task) { task.ApplyWorkflowState(StateWaiting) }), true},
		{"one day short of stale", build(func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -(StaleTaskThresholdDays - 1))
		}), false},
		{"exactly stale", build(func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -StaleTaskThresholdDays)
		}), true},
		{"deferred and flagged", build(func(task *Task) {
			task.Review = true
			task.DeferredUntil = &later
		}), true},
		{"deferred and waiting", build(func(task *Task) {
			task.ApplyWorkflowState(StateWaiting)
			task.DeferredUntil = &later
		}), true},
		{"deferred and merely old", build(func(task *Task) {
			task.UpdatedAt = old
			task.DeferredUntil = &later
		}), false},
		{"expired deferral and old", build(func(task *Task) {
			task.UpdatedAt = old
			expired := now.Add(-time.Hour)
			task.DeferredUntil = &expired
		}), true},
		{"done and flagged", build(func(task *Task) {
			task.Review = true
			task.ApplyWorkflowState(StateDone)
		}), false},
		{"canceled and old", build(func(task *Task) {
			task.UpdatedAt = old
			task.ApplyWorkflowState(StateCanceled)
		}), false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := TaskAwaitsReview(testCase.task, now); got != testCase.want {
				t.Errorf("TaskAwaitsReview = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestAcknowledgeReviewTouchesEvenAnUnflaggedTask is why the acknowledgement is
// its own primitive: ApplyReview(false, now) writes nothing when the flag is
// already down, which is exactly the state a stale or Waiting candidate is in.
func TestAcknowledgeReviewTouchesEvenAnUnflaggedTask(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	task, err := NewGenericTask("t1", 1, "Old importer spike")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	task.UpdatedAt = now.AddDate(0, 0, -200)

	if task.ApplyReview(false, now) {
		t.Fatal("ApplyReview reported a change on an unflagged task")
	}
	if task.UpdatedAt.Equal(now) {
		t.Fatal("ApplyReview touched an unflagged task; the primitive under test would be redundant")
	}

	if ackErr := task.AcknowledgeReview(now); ackErr != nil {
		t.Fatalf("AcknowledgeReview: %v", ackErr)
	}
	if !task.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", task.UpdatedAt, now)
	}
}

// TestAcknowledgeReviewLeavesTheStateClock keeps the two clocks apart at the
// primitive level: acknowledging is a touch, and only a lifecycle transition may
// move the state clock.
func TestAcknowledgeReviewLeavesTheStateClock(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	task, err := NewGenericTask("t1", 1, "Chase the vendor")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	task.Review = true
	task.ApplyWorkflowState(StateWaiting)
	stateChanged := now.AddDate(0, 0, -30)
	task.StateChangedAt = stateChanged

	if ackErr := task.AcknowledgeReview(now); ackErr != nil {
		t.Fatalf("AcknowledgeReview: %v", ackErr)
	}

	if task.Review {
		t.Error("review flag survived the acknowledgement")
	}
	if !task.IsInState(StateWaiting) {
		t.Errorf("state = %s, want Waiting", task.State())
	}
	if !task.StateChangedAt.Equal(stateChanged) {
		t.Errorf("StateChangedAt = %v, want the untouched %v", task.StateChangedAt, stateChanged)
	}
}
