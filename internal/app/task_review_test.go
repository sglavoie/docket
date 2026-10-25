package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// markReviewedNow is the instant every test below acknowledges at, and
// markReviewedStale is a touch time old enough to be stale against it. They are
// constants of the fixture rather than of the suite so the eligibility boundary
// is stated once.
var (
	markReviewedNow   = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	markReviewedStale = markReviewedNow.AddDate(0, 0, -200)
	markReviewedFresh = markReviewedNow.AddDate(0, 0, -1)
	// markReviewedToday is the calendar day markReviewedNow falls on. A check-in
	// is a calendar date, so every scheduled fixture is built from this rather
	// than from the instant, which the model's own gate would refuse.
	markReviewedToday = time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
)

// seedCheckIn writes a schedule through the model's gate, so a fixture that the
// production path would refuse fails here as a fixture error rather than as a
// misleading assertion about the service.
func seedCheckIn(t *testing.T, task *models.Task, next time.Time, repeatDays int) {
	t.Helper()
	scheduled := next
	if err := task.SetCheckIn(
		models.CheckInSchedule{Next: &scheduled, RepeatDays: repeatDays}, markReviewedFresh,
	); err != nil {
		t.Fatalf("seed check-in: %v", err)
	}
}

// seedMarkReviewedTask wires one task into a service whose clock is fixed at
// markReviewedNow, so a committed acknowledgement is recognizable by UpdatedAt
// landing exactly there rather than merely moving.
func seedMarkReviewedTask(t *testing.T, task *models.Task) (*app.Service, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	return newTestServiceWithClock(repo, fixedClock{t: markReviewedNow}), repo
}

// TestMarkReviewedAdmitsEveryCandidateKind walks the independent ways a task can
// have a review to acknowledge plus the overlap of them. Each is admitted on its
// own terms, so none of them may be expressed through another: a scheduled task
// is not silently flagged first, and the early case is admitted without its day
// having arrived.
func TestMarkReviewedAdmitsEveryCandidateKind(t *testing.T) {
	for _, testCase := range []struct {
		name string
		seed func(*models.Task)
	}{
		{"flagged", func(task *models.Task) {
			task.Review = true
			task.UpdatedAt = markReviewedFresh
		}},
		{"check-in due today", func(task *models.Task) {
			seedCheckIn(t, task, markReviewedToday, models.CheckInRepeatOff)
			task.UpdatedAt = markReviewedFresh
		}},
		{"check-in overdue", func(task *models.Task) {
			seedCheckIn(t, task, markReviewedToday.AddDate(0, 0, -4), models.CheckInRepeatOff)
			task.UpdatedAt = markReviewedFresh
		}},
		// The early review: the day has not come, and naming the task by hand is
		// what admits it anyway.
		{"check-in still ahead", func(task *models.Task) {
			seedCheckIn(t, task, markReviewedToday.AddDate(0, 0, 6), 7)
			task.UpdatedAt = markReviewedFresh
		}},
		{"flagged and scheduled at once", func(task *models.Task) {
			task.Review = true
			seedCheckIn(t, task, markReviewedToday, 14)
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

// TestMarkReviewedMovesARepeatingCheckInOffTheAttentionList is what makes a pass
// terminate: a repeating schedule survives its acknowledgement, but the date it
// now carries is counted from the review day, so the same task stops asking for
// attention even though it is still explicitly reviewable. A repeat that
// re-raised itself immediately would make a pass circle forever.
func TestMarkReviewedMovesARepeatingCheckInOffTheAttentionList(t *testing.T) {
	task := newTask("t1", 1, "Chase the vendor")
	task.ApplyWorkflowState(models.StateWaiting)
	seedCheckIn(t, task, markReviewedToday.AddDate(0, 0, -3), 7)
	task.UpdatedAt = markReviewedFresh
	svc, repo := seedMarkReviewedTask(t, task)

	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}
	want := markReviewedToday.AddDate(0, 0, 7)
	if task.NextCheckIn == nil || !task.NextCheckIn.Equal(want) {
		t.Fatalf("NextCheckIn = %v, want %v counted from the review day", task.NextCheckIn, want)
	}
	if task.CheckInRepeatDays != 7 {
		t.Errorf("repeat = %d, want the cadence to survive its own occurrence", task.CheckInRepeatDays)
	}
	if models.TaskNeedsAttention(task, markReviewedNow) {
		t.Error("an answered check-in is still asking for attention")
	}
	// Still explicitly reviewable, because reviewing scheduled work early is
	// allowed — which is also what makes a second acknowledgement legal.
	if !models.TaskAllowsExplicitReview(task, markReviewedNow) {
		t.Error("a scheduled task refuses an early review")
	}
	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("second MarkReviewed: %v", err)
	}
}

// TestMarkReviewedClearsAOneOffCheckIn is the other half of the cadence: a
// schedule with no repeat is answered once and gone, so the second
// acknowledgement has nothing to take and is refused.
func TestMarkReviewedClearsAOneOffCheckIn(t *testing.T) {
	task := newTask("t1", 1, "Old importer spike")
	seedCheckIn(t, task, markReviewedToday, models.CheckInRepeatOff)
	task.UpdatedAt = markReviewedFresh
	svc, repo := seedMarkReviewedTask(t, task)

	if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}
	if !task.CheckIn().IsEmpty() {
		t.Fatalf("check-in = %v, want an answered one-off cleared whole", task.CheckIn())
	}
	// The history survives what it recorded: what was cleared is what is owed.
	if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(markReviewedNow) {
		t.Errorf("LastReviewedAt = %v, want the operation clock %v", task.LastReviewedAt, markReviewedNow)
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
		// Staleness and Waiting are the two retired routes. Neither may come
		// back through this door: they are what `fs` and `?stale` are for, and
		// admitting them here is what made the old pass a list of everything.
		{"merely stale", func(task *models.Task) {
			task.UpdatedAt = markReviewedStale
		}},
		{"merely waiting", func(task *models.Task) {
			task.ApplyWorkflowState(models.StateWaiting)
			task.UpdatedAt = markReviewedFresh
		}},
		{"stale and waiting at once", func(task *models.Task) {
			task.ApplyWorkflowState(models.StateWaiting)
			task.UpdatedAt = markReviewedStale
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
		// Hiding suspends the review: even named, a hidden task is refused.
		{"hidden and flagged", func(task *models.Task) {
			task.Review = true
			task.Hidden = true
			task.UpdatedAt = markReviewedFresh
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

// TestMarkReviewedReadsCurrentTime keeps the admission test, the stamps, and the
// successor date on one reading of the service's clock rather than on the wall.
// The same fixture acknowledged at two instants must answer differently in all
// three, and the successor must be counted from the day the review happened —
// not from the date that was owed, which is what keeps a cadence honest when a
// check-in is answered late.
func TestMarkReviewedReadsCurrentTime(t *testing.T) {
	owed := markReviewedToday.AddDate(0, 0, -30)
	for _, testCase := range []struct {
		name string
		now  time.Time
		day  time.Time
	}{
		{"acknowledged today", markReviewedNow, markReviewedToday},
		{"acknowledged tomorrow", markReviewedNow.AddDate(0, 0, 1), markReviewedToday.AddDate(0, 0, 1)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			task := newTask("t1", 1, "Old importer spike")
			seedCheckIn(t, task, owed, 7)
			task.UpdatedAt = markReviewedFresh
			repo := newFakeRepo(newProject("proj-a", "Alpha", task))

			svc := newTestServiceWithClock(repo, fixedClock{t: testCase.now})
			if _, err := svc.MarkReviewed(repo.loc("t1")); err != nil {
				t.Fatalf("MarkReviewed: %v", err)
			}
			if !task.UpdatedAt.Equal(testCase.now) || task.LastReviewedAt == nil ||
				!task.LastReviewedAt.Equal(testCase.now) {
				t.Errorf("stamps = %v / %v, want the service clock %v",
					task.UpdatedAt, task.LastReviewedAt, testCase.now)
			}
			want := testCase.day.AddDate(0, 0, 7)
			if task.NextCheckIn == nil || !task.NextCheckIn.Equal(want) {
				t.Errorf("NextCheckIn = %v, want %v counted from the review day",
					task.NextCheckIn, want)
			}
		})
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
