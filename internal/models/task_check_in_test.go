package models

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/util"
)

// checkInClock is the fixed instant every test here works from. Nothing in this
// file reads a wall clock, because nothing in the code under test does either.
var checkInClock = time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)

func checkInDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func checkInDayPtr(year int, month time.Month, day int) *time.Time {
	day0 := checkInDay(year, month, day)
	return &day0
}

func checkInTask(t *testing.T, state WorkflowState) *Task {
	t.Helper()
	task := &Task{ID: "task-1", Title: "Check in on this", CreatedAt: checkInClock, UpdatedAt: checkInClock}
	if err := task.ApplyStateTransitionAt(state, checkInClock); err != nil {
		t.Fatalf("ApplyStateTransitionAt(%s): %v", state, err)
	}
	return task
}

// TestCheckInScheduleValueSemantics covers the pair as a value, since a caller
// that can only set or clear both halves at once cannot store a cadence with
// nothing to count from.
func TestCheckInScheduleValueSemantics(t *testing.T) {
	var absent CheckInSchedule
	if !absent.IsEmpty() || absent.Repeats() {
		t.Fatalf("the zero schedule is not absent: %+v", absent)
	}
	oneOff := CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20)}
	if oneOff.IsEmpty() || oneOff.Repeats() {
		t.Fatalf("a one-off schedule read as %v/%v", oneOff.IsEmpty(), oneOff.Repeats())
	}
	repeating := CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7}
	if !repeating.Repeats() {
		t.Fatal("a 7-day repeat does not report as repeating")
	}
}

// TestCheckInScheduleCloneDetachesItsDate is the immutable-copy rule at the
// value level: two schedules that shared a *time.Time would let an edit through
// one rewrite the other.
func TestCheckInScheduleCloneDetachesItsDate(t *testing.T) {
	original := CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7}
	cloned := original.Clone()
	if cloned.Next == original.Next {
		t.Fatal("Clone shared the date pointer")
	}
	*cloned.Next = checkInDay(2030, time.January, 1)
	if !original.Next.Equal(checkInDay(2026, time.March, 20)) {
		t.Fatalf("writing the clone moved the original to %s", original.Next)
	}
	if CloneInstant(nil) != nil {
		t.Fatal("CloneInstant(nil) minted an instant")
	}
}

// TestValidateCheckInScheduleRefusesTheThreeUnstorableShapes is the one gate
// every writing surface shares, so what it refuses is what the storage writer
// and every editing surface refuse.
func TestValidateCheckInScheduleRefusesTheThreeUnstorableShapes(t *testing.T) {
	for name, schedule := range map[string]CheckInSchedule{
		"absent":             {},
		"one-off":            {Next: checkInDayPtr(2026, time.March, 20)},
		"repeating":          {Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7},
		"longest cadence":    {Next: checkInDayPtr(2026, time.March, 20), RepeatDays: MaxCheckInRepeatDays},
		"shortest cadence":   {Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 1},
		"a date in the past": {Next: checkInDayPtr(2020, time.January, 1)},
	} {
		if err := ValidateCheckInSchedule(schedule); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
	for name, schedule := range map[string]CheckInSchedule{
		// A cadence needs an occurrence to count from; a schedule that can never
		// fire is not a schedule.
		"repeat with no date": {RepeatDays: 7},
		"negative repeat":     {Next: checkInDayPtr(2026, time.March, 20), RepeatDays: -1},
		"repeat past the bound": {
			Next: checkInDayPtr(2026, time.March, 20), RepeatDays: MaxCheckInRepeatDays + 1,
		},
		// The stored form has no place to put an hour, so accepting one would read
		// back as a different value.
		"date carrying an hour": {
			Next: func() *time.Time {
				at := time.Date(2026, time.March, 20, 9, 0, 0, 0, time.UTC)
				return &at
			}(),
		},
		"date carrying a second": {
			Next: func() *time.Time {
				at := time.Date(2026, time.March, 20, 0, 0, 1, 0, time.UTC)
				return &at
			}(),
		},
		"date the document cannot spell": {Next: checkInDayPtr(10000, time.January, 1)},
	} {
		if err := ValidateCheckInSchedule(schedule); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// TestValidateCheckInScheduleAcceptsADateWithNoMidnight keeps the gate and the
// arithmetic in agreement. In a zone whose transition happens at 00:00 the start
// of a date is 01:00, so a gate that asked whether the value equalled midnight
// would refuse the very date NextCheckInAfter computes.
func TestValidateCheckInScheduleAcceptsADateWithNoMidnight(t *testing.T) {
	havana, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Skipf("zone database unavailable: %v", err)
	}
	start, err := util.StartOfCalendarDate(2026, time.March, 8, havana)
	if err != nil {
		t.Fatalf("2026-03-08 in America/Havana: %v", err)
	}
	if start.Hour() != 1 {
		t.Skipf("America/Havana no longer skips midnight on that date (hour %d)", start.Hour())
	}
	if err := ValidateCheckInSchedule(CheckInSchedule{Next: &start, RepeatDays: 7}); err != nil {
		t.Fatalf("the start of a real date was refused: %v", err)
	}
}

// TestNextCheckInAfterUsesCalendarArithmetic is the recurrence rule: the next
// date is counted in whole days from the day the review actually happened, and
// month, leap-year, and daylight-saving boundaries pass under it unchanged.
func TestNextCheckInAfterUsesCalendarArithmetic(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("zone database unavailable: %v", err)
	}
	for name, tc := range map[string]struct {
		from   time.Time
		repeat int
		want   string
	}{
		"a week":            {checkInClock, 7, "2026-03-22"},
		"across a month":    {time.Date(2026, 1, 28, 23, 59, 0, 0, time.UTC), 7, "2026-02-04"},
		"onto a leap day":   {time.Date(2028, 2, 28, 8, 0, 0, 0, time.UTC), 1, "2028-02-29"},
		"over a leap day":   {time.Date(2028, 2, 27, 8, 0, 0, 0, time.UTC), 3, "2028-03-01"},
		"across a year end": {time.Date(2026, 12, 30, 8, 0, 0, 0, time.UTC), 3, "2027-01-02"},
		// The transition inside the span is exactly what a 24-hour multiple would
		// get wrong.
		"across a spring forward": {time.Date(2026, 3, 6, 9, 0, 0, 0, newYork), 3, "2026-03-09"},
		"across a fall back":      {time.Date(2026, 10, 30, 9, 0, 0, 0, newYork), 3, "2026-11-02"},
	} {
		t.Run(name, func(t *testing.T) {
			next, err := NextCheckInAfter(tc.from, tc.repeat)
			if err != nil {
				t.Fatalf("NextCheckInAfter: %v", err)
			}
			if next == nil {
				t.Fatal("a repeating schedule produced no next date")
			}
			if got := next.Format(util.DateFormat); got != tc.want {
				t.Fatalf("next = %s, want %s", got, tc.want)
			}
			// Whatever it computed has to be a schedule the gate accepts.
			if err := ValidateCheckInSchedule(CheckInSchedule{Next: next, RepeatDays: tc.repeat}); err != nil {
				t.Fatalf("the computed date is not a storable schedule: %v", err)
			}
		})
	}
}

// TestNextCheckInAfterOffProducesNoDate is what makes acknowledging a one-off
// check-in clear it rather than move it.
func TestNextCheckInAfterOffProducesNoDate(t *testing.T) {
	next, err := NextCheckInAfter(checkInClock, CheckInRepeatOff)
	if err != nil {
		t.Fatalf("NextCheckInAfter: %v", err)
	}
	if next != nil {
		t.Fatalf("an off repeat produced %s", next)
	}
	if _, err := NextCheckInAfter(checkInClock, -1); err == nil {
		t.Fatal("a negative repeat was accepted")
	}
	if _, err := NextCheckInAfter(checkInClock, MaxCheckInRepeatDays+1); err == nil {
		t.Fatal("a repeat past the bound was accepted")
	}
}

// TestCheckInHasArrivedComparesCalendarDates pins the arrival rule: a check-in
// is due when its day has come, not when some number of hours has elapsed, so
// the same date answers the same way at either end of a day.
func TestCheckInHasArrivedComparesCalendarDates(t *testing.T) {
	today := checkInDayPtr(2026, time.March, 15)
	for name, now := range map[string]time.Time{
		"first minute of the day": time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		"midday":                  checkInClock,
		"last minute of the day":  time.Date(2026, 3, 15, 23, 59, 59, 0, time.UTC),
	} {
		if !CheckInHasArrived(today, now) {
			t.Errorf("%s: today's check-in had not arrived", name)
		}
	}
	if !CheckInHasArrived(checkInDayPtr(2026, time.March, 1), checkInClock) {
		t.Error("an overdue check-in had not arrived")
	}
	if CheckInHasArrived(checkInDayPtr(2026, time.March, 16), checkInClock) {
		t.Error("tomorrow's check-in had already arrived")
	}
	if CheckInHasArrived(nil, checkInClock) {
		t.Error("an absent check-in arrived")
	}
}

// TestTaskCheckInAccessorsAreDetached is the immutable-copy rule at the task
// level: a reader holding what a task returned must not be able to reach back
// into the task with it.
func TestTaskCheckInAccessorsAreDetached(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	task.LastReviewedAt = &checkInClock

	read := task.CheckIn()
	if read.Next == task.NextCheckIn {
		t.Fatal("CheckIn returned the stored pointer")
	}
	// Only the pointer is rewritten here: RepeatDays is a plain int in a returned
	// struct, so the language already answers for it, and writing it would assert
	// nothing about the accessor.
	*read.Next = checkInDay(2030, time.January, 1)
	if !task.NextCheckIn.Equal(checkInDay(2026, time.March, 20)) || task.CheckInRepeatDays != 7 {
		t.Fatalf("writing the read copy changed the task: %s/%d", task.NextCheckIn, task.CheckInRepeatDays)
	}

	reviewed := task.LastReviewed()
	if reviewed == task.LastReviewedAt {
		t.Fatal("LastReviewed returned the stored pointer")
	}
	*reviewed = checkInDay(2030, time.January, 1)
	if !task.LastReviewedAt.Equal(checkInClock) {
		t.Fatalf("writing the read copy moved the review instant to %s", task.LastReviewedAt)
	}

	// The caller's own value is copied on the way in for the same reason.
	source := checkInDay(2026, time.April, 1)
	if err := task.SetCheckIn(CheckInSchedule{Next: &source}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	source = checkInDay(2031, time.January, 1)
	if !task.NextCheckIn.Equal(checkInDay(2026, time.April, 1)) {
		t.Fatalf("the task followed the caller's variable to %s", task.NextCheckIn)
	}

	var absent *Task
	if !absent.CheckIn().IsEmpty() || absent.LastReviewed() != nil {
		t.Fatal("a nil task answered with a schedule")
	}
}

// TestSetCheckInIsNotAnAcknowledgement is the distinction the product rests on:
// deciding when to look at something again says nothing about having looked at
// it now.
func TestSetCheckInIsNotAnAcknowledgement(t *testing.T) {
	task := checkInTask(t, StateWaiting)
	task.Review = true
	stateChangedAt, dueDate := task.StateChangedAt, checkInDay(2026, time.March, 18)
	task.DueDate = &dueDate

	later := checkInClock.Add(48 * time.Hour)
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.April, 1), RepeatDays: 30}, later); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if !task.Review {
		t.Error("rescheduling cleared the review flag")
	}
	if task.LastReviewedAt != nil {
		t.Errorf("rescheduling stamped a review at %s", task.LastReviewedAt)
	}
	if !task.IsInState(StateWaiting) || !task.StateChangedAt.Equal(stateChangedAt) {
		t.Errorf("rescheduling moved the lifecycle: %s at %s", task.State(), task.StateChangedAt)
	}
	if task.DueDate == nil || !task.DueDate.Equal(dueDate) {
		t.Errorf("rescheduling touched the deadline: %v", task.DueDate)
	}
	if !task.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %s, want %s", task.UpdatedAt, later)
	}
}

// TestSetCheckInIgnoresARedeclaredSchedule keeps a re-declaration from reading
// as an edit, which is what stops an outline round trip from touching every task
// it re-states.
func TestSetCheckInIgnoresARedeclaredSchedule(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	schedule := CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7}
	if err := task.SetCheckIn(schedule, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	later := checkInClock.Add(72 * time.Hour)
	if err := task.SetCheckIn(schedule.Clone(), later); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if !task.UpdatedAt.Equal(checkInClock) {
		t.Fatalf("re-declaring the same schedule moved UpdatedAt to %s", task.UpdatedAt)
	}
	// A refused schedule is not a partial write either.
	if err := task.SetCheckIn(CheckInSchedule{RepeatDays: 3}, later); err == nil {
		t.Fatal("a repeat with no date was accepted")
	}
	if !task.NextCheckIn.Equal(checkInDay(2026, time.March, 20)) || task.CheckInRepeatDays != 7 {
		t.Fatalf("a refusal changed the schedule: %s/%d", task.NextCheckIn, task.CheckInRepeatDays)
	}
}

// TestClearCheckInKeepsReviewHistory is the absent-versus-cleared distinction:
// what a clear removes is what is owed, never what happened.
func TestClearCheckInKeepsReviewHistory(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	reviewedAt := checkInClock.Add(-72 * time.Hour)
	task.LastReviewedAt = &reviewedAt

	later := checkInClock.Add(time.Hour)
	if !task.ClearCheckIn(later) {
		t.Fatal("ClearCheckIn reported no change")
	}
	if !task.CheckIn().IsEmpty() {
		t.Fatalf("the schedule survived the clear: %+v", task.CheckIn())
	}
	if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(reviewedAt) {
		t.Fatalf("the clear took the review history: %v", task.LastReviewedAt)
	}
	if !task.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %s, want %s", task.UpdatedAt, later)
	}
	// Clearing nothing is not a change, so it does not touch the task.
	if task.ClearCheckIn(later.Add(time.Hour)) {
		t.Error("clearing an absent schedule reported a change")
	}
	if !task.UpdatedAt.Equal(later) {
		t.Errorf("clearing nothing moved UpdatedAt to %s", task.UpdatedAt)
	}
}

// TestCheckInIsDueAtSuppressesDeferredAndTerminalWork covers the eligibility
// half of arrival: a deferred task was put out of sight deliberately, and a
// reminder reaching through that would make deferral mean less than it says.
func TestCheckInIsDueAtSuppressesDeferredAndTerminalWork(t *testing.T) {
	task := checkInTask(t, StateBacklog)
	if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 15)}, checkInClock); err != nil {
		t.Fatalf("SetCheckIn: %v", err)
	}
	if !task.CheckInIsDueAt(checkInClock) {
		t.Fatal("an arrived check-in on an open task is not due")
	}

	wake := checkInClock.Add(24 * time.Hour)
	task.DeferredUntil = &wake
	if task.CheckInIsDueAt(checkInClock) {
		t.Error("a deferred task raised its check-in")
	}
	// Deferral suppresses, it does not delete: the schedule is still there when
	// the deferral lapses.
	if task.CheckInIsDueAt(wake.Add(time.Minute)) != true {
		t.Error("the check-in did not return once the deferral lapsed")
	}
	task.DeferredUntil = nil

	var absent *Task
	if absent.CheckInIsDueAt(checkInClock) {
		t.Error("a nil task had a check-in due")
	}
}

// TestTerminalStateClearsTheScheduleAndReopeningInheritsNone is the terminal
// cleanup rule, including the part that is easy to lose: the schedule is dropped
// on the way in, so reopening cannot resurrect a cadence nobody asked for.
func TestTerminalStateClearsTheScheduleAndReopeningInheritsNone(t *testing.T) {
	for _, terminal := range []WorkflowState{StateDone, StateCanceled} {
		t.Run(string(terminal), func(t *testing.T) {
			task := checkInTask(t, StateDoing)
			if err := task.SetCheckIn(CheckInSchedule{Next: checkInDayPtr(2026, time.March, 20), RepeatDays: 7}, checkInClock); err != nil {
				t.Fatalf("SetCheckIn: %v", err)
			}
			reviewedAt := checkInClock.Add(-24 * time.Hour)
			task.LastReviewedAt = &reviewedAt

			finished := checkInClock.Add(time.Hour)
			if err := task.ApplyStateTransitionAt(terminal, finished); err != nil {
				t.Fatalf("ApplyStateTransitionAt(%s): %v", terminal, err)
			}
			if !task.CheckIn().IsEmpty() {
				t.Fatalf("%s kept the schedule: %+v", terminal, task.CheckIn())
			}
			if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(reviewedAt) {
				t.Fatalf("%s took the review history: %v", terminal, task.LastReviewedAt)
			}
			if task.CheckInIsDueAt(finished.Add(365 * 24 * time.Hour)) {
				t.Fatalf("%s work still raised a check-in", terminal)
			}

			reopened := finished.Add(48 * time.Hour)
			if err := task.ApplyStateTransitionAt(StateDoing, reopened); err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if !task.CheckIn().IsEmpty() {
				t.Fatalf("reopening inherited a schedule: %+v", task.CheckIn())
			}
			if task.LastReviewedAt == nil || !task.LastReviewedAt.Equal(reviewedAt) {
				t.Fatalf("reopening changed the review history: %v", task.LastReviewedAt)
			}
		})
	}
}
