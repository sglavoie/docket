package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// markReviewedNow is the instant every test below acknowledges at, and
// markReviewedStale is a touch time old enough to be stale against it. They are
// constants of the fixture rather than of the suite so the eligibility boundary
// is stated once.
var (
	markReviewedNow   = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	markReviewedStale = markReviewedNow.AddDate(0, 0, -200)
	markReviewedFresh = markReviewedNow.AddDate(0, 0, -1)
)

// seedMarkReviewedTask wires one task into a service whose clock is fixed at
// markReviewedNow, so a committed acknowledgement is recognizable by UpdatedAt
// landing exactly there rather than merely moving.
func seedMarkReviewedTask(t *testing.T, task *models.Task) (*app.Service, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	return newTestServiceWithClock(repo, fixedClock{t: markReviewedNow}), repo
}

// TestMarkReviewedAdmitsEveryCandidateKind walks the three independent ways a
// task can be awaiting review plus the overlap of all three. Each is admitted
// on its own terms, so none of them may be expressed through another: a stale
// task is not silently flagged first, and a Waiting one is not made stale.
func TestMarkReviewedAdmitsEveryCandidateKind(t *testing.T) {
	for _, testCase := range []struct {
		name string
		seed func(*models.Task)
	}{
		{"flagged", func(task *models.Task) {
			task.Review = true
			task.UpdatedAt = markReviewedFresh
		}},
		{"stale", func(task *models.Task) {
			task.UpdatedAt = markReviewedStale
		}},
		{"waiting", func(task *models.Task) {
			task.ApplyWorkflowState(models.StateWaiting)
			task.UpdatedAt = markReviewedFresh
		}},
		{"all three at once", func(task *models.Task) {
			task.Review = true
			task.ApplyWorkflowState(models.StateWaiting)
			task.UpdatedAt = markReviewedStale
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			task := newTask("t1", 1, "Chase the vendor")
			testCase.seed(task)
			svc, repo := seedMarkReviewedTask(t, task)

			result, err := svc.MarkReviewed(repo.loc("t1"))
			if err != nil {
				t.Fatalf("MarkReviewed: %v", err)
			}
			if result.TransactionID == "" {
				t.Fatal("acknowledgement committed no transaction")
			}
			if task.Review {
				t.Error("review flag survived the acknowledgement")
			}
			if !task.UpdatedAt.Equal(markReviewedNow) {
				t.Errorf("UpdatedAt = %v, want the operation clock %v", task.UpdatedAt, markReviewedNow)
			}
			// One acknowledgement is one ledger entry, named for what it did
			// rather than for the flag it happened to clear.
			committed, ok := repo.lastCommit()
			if !ok || committed.UndoAction != "Mark Reviewed" {
				t.Fatalf("undo action = %q, want Mark Reviewed", committed.UndoAction)
			}
			if committed.UndoContext != "#1 Chase the vendor" {
				t.Errorf("undo context = %q, want the task it named", committed.UndoContext)
			}
		})
	}
}

// TestMarkReviewedPreservesEverythingButTheFlagAndTouch is the acknowledgement's
// whole write contract stated as one assertion set. The plausible
// implementation — routing this through a lifecycle transition — would clear
// deferral and completion data on the way past, so every field a transition
// would disturb is checked here rather than assumed.
func TestMarkReviewedPreservesEverythingButTheFlagAndTouch(t *testing.T) {
	stateChanged := markReviewedNow.AddDate(0, 0, -30)
	due := markReviewedNow.AddDate(0, 0, 5)
	deferred := markReviewedNow.Add(48 * time.Hour)
	pinned := markReviewedNow.AddDate(0, 0, -10)

	task := newTask("t1", 1, "Chase the vendor")
	task.Review = true
	task.ApplyWorkflowState(models.StateWaiting)
	task.StateChangedAt = stateChanged
	task.DueDate = &due
	task.DeferredUntil = &deferred
	task.Pinned = true
	task.PinnedAt = &pinned
	task.ResumeNote = "vendor promised a schema by Friday"
	task.Description = "The importer cannot land without it."
	task.Priority = models.PriorityHigh
	task.Context = "office"
	task.Tags = []string{"importer"}
	task.UpdatedAt = markReviewedFresh

	svc, repo := seedMarkReviewedTask(t, task)
	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}

	if !task.IsInState(models.StateWaiting) {
		t.Errorf("state = %s, want the Waiting it was left in", task.State())
	}
	// The state clock is what the row's waiting age counts from, so leaving it
	// alone is what keeps "waiting 30 days" true across an acknowledgement.
	if !task.StateChangedAt.Equal(stateChanged) {
		t.Errorf("StateChangedAt = %v, want the untouched %v", task.StateChangedAt, stateChanged)
	}
	if task.DueDate == nil || !task.DueDate.Equal(due) {
		t.Errorf("check-back date = %v, want the untouched %v", task.DueDate, due)
	}
	if task.DeferredUntil == nil || !task.DeferredUntil.Equal(deferred) {
		t.Errorf("DeferredUntil = %v, want the untouched %v", task.DeferredUntil, deferred)
	}
	if !task.Pinned || task.PinnedAt == nil || !task.PinnedAt.Equal(pinned) {
		t.Errorf("pin = %v %v, want the untouched pin at %v", task.Pinned, task.PinnedAt, pinned)
	}
	if task.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nothing minted", task.CompletedAt)
	}
	if task.ResumeNote != "vendor promised a schema by Friday" {
		t.Errorf("ResumeNote = %q, want it untouched", task.ResumeNote)
	}
	if task.Description == "" || task.Priority != models.PriorityHigh ||
		task.Context != "office" || len(task.Tags) != 1 {
		t.Errorf("content changed: %q %d %q %v",
			task.Description, task.Priority, task.Context, task.Tags)
	}
}

// TestMarkReviewedKeepsWaitingACandidate is the consequence of preserving the
// state: acknowledging a Waiting task says "I have seen it", not "I have heard
// back", so the same task is still a candidate for the next pass.
func TestMarkReviewedKeepsWaitingACandidate(t *testing.T) {
	task := newTask("t1", 1, "Chase the vendor")
	task.ApplyWorkflowState(models.StateWaiting)
	task.UpdatedAt = markReviewedFresh
	svc, repo := seedMarkReviewedTask(t, task)

	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}
	if !models.TaskAwaitsReview(task, markReviewedNow) {
		t.Fatal("an acknowledged Waiting task stopped being a review candidate")
	}
	// And acknowledging it again is legal, because nothing about it changed.
	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("second MarkReviewed: %v", err)
	}
}

// TestMarkReviewedStopsStaleness is the other consequence: the touch is the
// answer to staleness, so a stale task acknowledged once is not a candidate the
// second time around.
func TestMarkReviewedStopsStaleness(t *testing.T) {
	task := newTask("t1", 1, "Old importer spike")
	task.UpdatedAt = markReviewedStale
	svc, repo := seedMarkReviewedTask(t, task)

	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}
	if models.TaskAwaitsReview(task, markReviewedNow) {
		t.Fatal("an acknowledged stale task is still a candidate")
	}
	if _, err := svc.MarkReviewed(repo.loc("t1")); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("second MarkReviewed = %v, want ErrInvalidInput", err)
	}
}

// TestMarkReviewedRefusesIneligibleTasks is the admission boundary, and it is
// checked through the service rather than the predicate because the refusal has
// to happen before anything is written: a rejected acknowledgement mints no
// transaction for `u` to spend.
func TestMarkReviewedRefusesIneligibleTasks(t *testing.T) {
	for _, testCase := range []struct {
		name string
		seed func(*models.Task)
	}{
		{"fresh open work", func(task *models.Task) {
			task.UpdatedAt = markReviewedFresh
		}},
		// Deferral is an answer to "is this still relevant", so an old task
		// hidden until next week is not asking to be looked at.
		{"deferred and old", func(task *models.Task) {
			task.UpdatedAt = markReviewedStale
			until := markReviewedNow.Add(72 * time.Hour)
			task.DeferredUntil = &until
		}},
		{"done and flagged", func(task *models.Task) {
			task.Review = true
			task.ApplyWorkflowState(models.StateDone)
			task.UpdatedAt = markReviewedStale
		}},
		{"canceled and old", func(task *models.Task) {
			task.ApplyWorkflowState(models.StateCanceled)
			task.UpdatedAt = markReviewedStale
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			task := newTask("t1", 1, "Chase the vendor")
			testCase.seed(task)
			svc, repo := seedMarkReviewedTask(t, task)
			before := len(repo.commits)
			updated, review := task.UpdatedAt, task.Review

			_, err := svc.MarkReviewed(repo.loc("t1"))
			if !errors.Is(err, app.ErrInvalidInput) {
				t.Fatalf("MarkReviewed = %v, want ErrInvalidInput", err)
			}
			if len(repo.commits) != before {
				t.Errorf("commits = %d, want the %d it started with", len(repo.commits), before)
			}
			if !task.UpdatedAt.Equal(updated) || task.Review != review {
				t.Errorf("refused acknowledgement still wrote %v / review %v", task.UpdatedAt, task.Review)
			}
		})
	}
}

// TestMarkReviewedAdmitsADeferredFlaggedTask is the one deferral that is
// admitted, and it is admitted for a reason rather than by accident: flagging
// something is the caller saying out loud that they want it back, which outranks
// the deferral they set themselves.
func TestMarkReviewedAdmitsADeferredFlaggedTask(t *testing.T) {
	task := newTask("t1", 1, "Revisit the spike")
	task.Review = true
	task.UpdatedAt = markReviewedFresh
	until := markReviewedNow.Add(72 * time.Hour)
	task.DeferredUntil = &until
	svc, repo := seedMarkReviewedTask(t, task)

	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}
	if task.Review {
		t.Error("review flag survived the acknowledgement")
	}
	if task.DeferredUntil == nil || !task.DeferredUntil.Equal(until) {
		t.Errorf("DeferredUntil = %v, want the untouched %v", task.DeferredUntil, until)
	}
}

// TestMarkReviewedReadsCurrentTime keeps the admission test and the write on one
// reading of the clock. A task one day short of stale is refused, and the same
// task is admitted once the clock alone has moved past the threshold — no other
// input changes between the two calls.
func TestMarkReviewedReadsCurrentTime(t *testing.T) {
	task := newTask("t1", 1, "Old importer spike")
	task.UpdatedAt = markReviewedNow.AddDate(0, 0, -(models.StaleTaskThresholdDays - 1))
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))

	early := newTestServiceWithClock(repo, fixedClock{t: markReviewedNow})
	if _, err := early.MarkReviewed(repo.loc("t1")); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("a day short of stale = %v, want ErrInvalidInput", err)
	}

	later := markReviewedNow.AddDate(0, 0, 1)
	if _, err := newTestServiceWithClock(repo, fixedClock{t: later}).MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("stale by the clock alone: %v", err)
	}
	if !task.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %v, want the clock that admitted it %v", task.UpdatedAt, later)
	}
}

// TestMarkReviewedRefusesAMissingTask keeps the resolution failure a failure:
// the recheck is about eligibility, and a locator naming nothing must still come
// back as a not-found rather than as an ineligible task.
func TestMarkReviewedRefusesAMissingTask(t *testing.T) {
	task := newTask("t1", 1, "Chase the vendor")
	task.Review = true
	svc, _ := seedMarkReviewedTask(t, task)

	_, err := svc.MarkReviewed(app.TaskLocator{ProjectID: "proj-a", TaskID: "gone"})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("MarkReviewed = %v, want ErrTaskNotFound", err)
	}
}
