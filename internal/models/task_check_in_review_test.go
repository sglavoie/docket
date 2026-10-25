package models

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/util"
)

// TestAcknowledgeReviewSchedulesFromTheDayItHappened is the recurrence rule at
// the task level, and the reason the next date is counted from the review rather
// than from the date that was owed: a weekly check-in answered four days late is
// due a week from today, not three days from today.
func TestAcknowledgeReviewSchedulesFromTheDayItHappened(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	task.Review = true
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 11), RepeatDays: 7}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	stateChangedAt := task.StateChangedAt

	// Four days after the check-in was owed.
	if err := task.AcknowledgeReview(checkInClock); err != nil {
		t.Fatalf("AcknowledgeReview: %v", err)
	}
	if task.NextCheckIn == nil || task.NextCheckIn.Format(util.DateFormat) != "2026-03-22" {
		t.Fatalf("next check-in = %v, want 2026-03-22", task.NextCheckIn)
	}
	if task.CheckInRepeatDays != 7 {
		t.Errorf("the cadence changed to %d", task.CheckInRepeatDays)
	}
	if task.Review {
		t.Error("acknowledging left the review flag set")
	}
	if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(checkInClock) {
		t.Errorf("review instant = %v, want %s", task.LastReviewedAt, checkInClock)
	}
	if !task.UpdatedAt.Equal(checkInClock) {
		t.Errorf("UpdatedAt = %s, want %s", task.UpdatedAt, checkInClock)
	}
	if !task.StateChangedAt.Equal(stateChangedAt) || !task.IsInState(StateBacklog) {
		t.Errorf("acknowledging moved the lifecycle: %s at %s", task.State(), task.StateChangedAt)
	}
	if task.CheckInIsDueAt(checkInClock) {
		t.Error("the answered check-in is still due today")
	}
}

// TestAcknowledgeReviewClearsAOneOff covers the other cadence: a check-in
// scheduled once is answered once.
func TestAcknowledgeReviewClearsAOneOff(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 15)}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if err := task.AcknowledgeReview(checkInClock); err != nil {
		t.Fatalf("AcknowledgeReview: %v", err)
	}
	if !task.CheckIn().IsEmpty() {
		t.Fatalf("a one-off check-in survived being answered: %+v", task.CheckIn())
	}
	if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(checkInClock) {
		t.Fatalf("review instant = %v", task.LastReviewedAt)
	}
}

// TestAcknowledgeReviewWithoutAScheduleOnlyRecordsIt keeps the flag and the
// schedule independent: acknowledging a flagged task that was never scheduled
// must not mint a cadence.
func TestAcknowledgeReviewWithoutAScheduleOnlyRecordsIt(t *testing.T) {
	task := checkInTask(t, StateWaiting)
	task.Review = true
	stateChangedAt, waitingSince := task.StateChangedAt, task.StateChangedAt

	if err := task.AcknowledgeReview(checkInClock); err != nil {
		t.Fatalf("AcknowledgeReview: %v", err)
	}
	if !task.CheckIn().IsEmpty() {
		t.Fatalf("acknowledging minted a schedule: %+v", task.CheckIn())
	}
	if task.Review {
		t.Error("the review flag survived")
	}
	// Acknowledging is not a lifecycle transition, so a waiting task keeps its
	// state and its unreset waiting age.
	if !task.IsInState(StateWaiting) || !task.StateChangedAt.Equal(stateChangedAt) {
		t.Errorf("state = %s at %s, want waiting at %s", task.State(), task.StateChangedAt, waitingSince)
	}
}

// TestAcknowledgeReviewIsRefusedWholeOrNotAtAll keeps the fallible half from
// half-writing: a next date the calendar cannot produce leaves the task exactly
// as it was, flag included.
func TestAcknowledgeReviewIsRefusedWholeOrNotAtAll(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	task.Review = true
	task.NextCheckIn = checkInDayPtr(2026, time.March, 15)
	// Past the bound the one validator allows, so computing the successor fails.
	task.CheckInRepeatDays = MaxCheckInRepeatDays + 1
	before := *task

	if err := task.AcknowledgeReview(checkInClock); err == nil {
		t.Fatal("an impossible cadence was acknowledged")
	}
	if !task.Review || task.LastReviewedAt != nil || task.CheckInRepeatDays != before.CheckInRepeatDays {
		t.Fatalf("a refusal wrote part of the acknowledgement: %+v", task)
	}
	if !task.NextCheckIn.Equal(*before.NextCheckIn) || !task.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("a refusal moved the schedule or the touch time: %+v", task)
	}
}

// TestAcknowledgeReviewIsClockIndependent is the injected-clock rule stated as a
// property: the same task answered at two different instants differs only by
// those instants, so nothing here reads a wall clock of its own.
func TestAcknowledgeReviewIsClockIndependent(t *testing.T) {
	build := func() *Task {
		task := checkInTask(t, StateBacklog)
		task.Review = true
		if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 11), RepeatDays: 7}, checkInClock); err != nil {
			t.Fatalf("SetCheckIn: %v", err)
		}
		return task
	}
	past, future := build(), build()
	pastNow := time.Date(2021, 6, 1, 8, 0, 0, 0, time.UTC)
	futureNow := time.Date(2031, 6, 1, 8, 0, 0, 0, time.UTC)
	if err := past.AcknowledgeReview(pastNow); err != nil {
		t.Fatalf("AcknowledgeReview: %v", err)
	}
	if err := future.AcknowledgeReview(futureNow); err != nil {
		t.Fatalf("AcknowledgeReview: %v", err)
	}
	if got := past.NextCheckIn.Format(util.DateFormat); got != "2021-06-08" {
		t.Errorf("next check-in = %s, want 2021-06-08", got)
	}
	if got := future.NextCheckIn.Format(util.DateFormat); got != "2031-06-08" {
		t.Errorf("next check-in = %s, want 2031-06-08", got)
	}
	if !past.LastReviewedAt.Equal(pastNow) || !future.LastReviewedAt.Equal(futureNow) {
		t.Errorf("review instants = %s / %s", past.LastReviewedAt, future.LastReviewedAt)
	}
	if past.Review || future.Review || past.CheckInRepeatDays != future.CheckInRepeatDays {
		t.Errorf("the two answers differ by more than their instants: %+v / %+v", past, future)
	}
}

// TestTaskNeedsAttentionIsTheAutomaticHalf covers the predicate the product
// raises work with: exactly two ways for a task to ask, and terminal work has
// none.
func TestTaskNeedsAttentionIsTheAutomaticHalf(t *testing.T) {
	flagged := checkInTask(t, StateBacklog)
	flagged.Review = true
	if !TaskNeedsAttention(flagged, checkInClock) {
		t.Error("a flagged task does not need attention")
	}

	scheduled := checkInTask(t, StateBacklog)
	if err := scheduled.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 15)}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if !TaskNeedsAttention(scheduled, checkInClock) {
		t.Error("an arrived check-in does not need attention")
	}
	// A check-in that has not arrived is not asking yet.
	if TaskNeedsAttention(scheduled, checkInClock.Add(-48*time.Hour)) {
		t.Error("a future check-in asked early")
	}
	// Nor is a deferred one, which is CheckInIsDueAt's rule showing through.
	wake := checkInClock.Add(24 * time.Hour)
	scheduled.DeferredUntil = &wake
	if TaskNeedsAttention(scheduled, checkInClock) {
		t.Error("a deferred task asked for attention")
	}
	scheduled.DeferredUntil = nil

	// Staleness and Waiting belong to the explicit queue, not to this one.
	stale := checkInTask(t, StateWaiting)
	stale.UpdatedAt = checkInClock.Add(-365 * 24 * time.Hour)
	stale.StateChangedAt = stale.UpdatedAt
	if TaskNeedsAttention(stale, checkInClock) {
		t.Error("stale waiting work raised itself automatically")
	}

	done := checkInTask(t, StateBacklog)
	done.Review = true
	if err := done.ApplyStateTransitionAt(StateDone, checkInClock); err != nil {
		t.Fatalf("complete: %v", err)
	}
	done.Review = true
	if TaskNeedsAttention(done, checkInClock) {
		t.Error("terminal work asked for attention")
	}
	if TaskNeedsAttention(nil, checkInClock) {
		t.Error("a nil task asked for attention")
	}
}

// TestTaskAllowsExplicitReviewAdmitsEarlyReview is the second predicate's whole
// reason for existing: going looking for scheduled work is allowed before its
// day arrives, which the automatic half must never do on its own.
func TestTaskAllowsExplicitReviewAdmitsEarlyReview(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.April, 30), RepeatDays: 30}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if TaskNeedsAttention(task, checkInClock) {
		t.Fatal("a check-in six weeks out asked for attention")
	}
	if !TaskAllowsExplicitReview(task, checkInClock) {
		t.Fatal("scheduled work refused an early review")
	}
	// Anything asking automatically is reviewable explicitly too.
	flagged := checkInTask(t, StateBacklog)
	flagged.Review = true
	if !TaskAllowsExplicitReview(flagged, checkInClock) {
		t.Error("a flagged task refused an explicit review")
	}
	// Unscheduled, unflagged work has nothing to review early.
	plain := checkInTask(t, StateBacklog)
	if TaskAllowsExplicitReview(plain, checkInClock) {
		t.Error("unscheduled work allowed an explicit review")
	}
	// A deferred scheduled task is still reachable deliberately: deferral
	// suppresses the reminder, not the ability to go look.
	deferred := checkInTask(t, StateBacklog)
	if err := deferred.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 15)}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	wake := checkInClock.Add(24 * time.Hour)
	deferred.DeferredUntil = &wake
	if !TaskAllowsExplicitReview(deferred, checkInClock) {
		t.Error("a deferred scheduled task refused an explicit review")
	}

	done := checkInTask(t, StateBacklog)
	if err := done.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.April, 30)}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if err := done.ApplyStateTransitionAt(StateDone, checkInClock); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if TaskAllowsExplicitReview(done, checkInClock) {
		t.Error("terminal work allowed an explicit review")
	}
	if TaskAllowsExplicitReview(nil, checkInClock) {
		t.Error("a nil task allowed an explicit review")
	}
}
