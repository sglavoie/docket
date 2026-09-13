package models

import (
	"testing"
	"time"
)

// TestStalenessAndWaitingAreNoLongerReviewRoutes pins the retirement of
// TaskAwaitsReview's two extra clauses. Neither live predicate may grow them
// back: an untouched task and a task parked on someone else are what `fs` and
// `?stale` are for, and admitting them automatically is what made a review pass
// a list of everything. The stale predicate itself is untouched and still says
// yes about the same task, which is what makes this a boundary and not a
// deletion.
func TestStalenessAndWaitingAreNoLongerReviewRoutes(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -(StaleTaskThresholdDays + 10))

	build := func(seed func(*Task)) *Task {
		task, err := NewGenericTask("t1", 1, "Chase the vendor")
		if err != nil {
			t.Fatalf("NewGenericTask: %v", err)
		}
		task.UpdatedAt = now.AddDate(0, 0, -1)
		seed(task)
		return task
	}

	stale := build(func(task *Task) { task.UpdatedAt = old })
	if !QueryTaskIsStale(stale, now) {
		t.Fatal("fixture is not stale; the retirement boundary proves nothing")
	}

	for _, testCase := range []struct {
		name string
		task *Task
	}{
		{"merely stale", stale},
		{"merely waiting", build(func(task *Task) { task.ApplyWorkflowState(StateWaiting) })},
		{"stale and waiting", build(func(task *Task) {
			task.UpdatedAt = old
			task.ApplyWorkflowState(StateWaiting)
		})},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if TaskNeedsAttention(testCase.task, now) {
				t.Error("TaskNeedsAttention readmitted a retired review route")
			}
			if TaskAllowsExplicitReview(testCase.task, now) {
				t.Error("TaskAllowsExplicitReview readmitted a retired review route")
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
