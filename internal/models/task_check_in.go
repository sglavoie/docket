package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// This file owns the check-in schedule: the pair of stored values, the one gate
// they pass through, the calendar arithmetic that advances them, and the
// detached copies every reader gets. Nothing here reads a wall clock of its own
// — every answer takes the caller's now — so a check-in behaves the same in a
// test at a fixed instant and in a running app.

// CheckInRepeatOff is the repeat that does not repeat: a check-in scheduled
// once, which acknowledging clears rather than moves.
const CheckInRepeatOff = 0

// MaxCheckInRepeatDays bounds a custom repeat. Ten years of days is far beyond
// any cadence a person keeps and far below the point where day arithmetic on a
// calendar date starts losing precision, so the bound exists to refuse a typo
// and an overflow rather than to express a policy about cadences.
const MaxCheckInRepeatDays = 3650

// CheckInSchedule is the check-in pair as one value, so a caller that means to
// set or clear a schedule cannot write half of it. The zero value is no
// schedule.
type CheckInSchedule struct {
	// Next is the calendar date the check-in is due, or nil for none.
	Next *time.Time
	// RepeatDays is the cadence in whole calendar days, or CheckInRepeatOff.
	RepeatDays int
}

// IsEmpty reports whether the schedule is absent entirely.
func (s CheckInSchedule) IsEmpty() bool {
	return s.Next == nil && s.RepeatDays == CheckInRepeatOff
}

// Repeats reports whether acknowledging this schedule moves it forward rather
// than clearing it.
func (s CheckInSchedule) Repeats() bool {
	return s.RepeatDays > CheckInRepeatOff
}

// Clone returns a schedule sharing no pointer with the receiver, so a caller
// holding one can never reach back into the task it was read from.
func (s CheckInSchedule) Clone() CheckInSchedule {
	return CheckInSchedule{Next: CloneInstant(s.Next), RepeatDays: s.RepeatDays}
}

// CloneInstant returns a detached copy of an optional instant. Every accessor
// here returns one rather than the stored pointer: two tasks that shared a
// *time.Time would let an edit through one rewrite the other's schedule.
func CloneInstant(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// ValidateCheckInRepeatDays is the one bound on a repeat. Off is valid; a
// negative cadence is not a cadence, and one past the bound is a typo.
func ValidateCheckInRepeatDays(days int) error {
	if days < CheckInRepeatOff {
		return fmt.Errorf("check-in repeat cannot be negative (got %d)", days)
	}
	if days > MaxCheckInRepeatDays {
		return fmt.Errorf("check-in repeat too long (%d days, max %d)", days, MaxCheckInRepeatDays)
	}
	return nil
}

// ValidateCheckInSchedule is the one gate every surface that writes a check-in
// passes through, so the model, the storage writer, and every later editing
// surface refuse exactly the same values instead of each bounding them their
// own way.
//
// Three things are refused. A repeat with no date, because a cadence needs an
// occurrence to count from and a schedule that can never fire is not a schedule.
// A date carrying a time of day, because a check-in arrives when its day
// arrives; the stored form has no place to put an hour, so accepting one would
// read back as a different value. And a date the canonical YYYY-MM-DD layout
// cannot round-trip, which is refused here rather than discovered by the writer,
// so the refusal names the schedule rather than the document.
func ValidateCheckInSchedule(schedule CheckInSchedule) error {
	if err := ValidateCheckInRepeatDays(schedule.RepeatDays); err != nil {
		return err
	}
	if schedule.Next == nil {
		if schedule.Repeats() {
			return fmt.Errorf("a check-in repeat of %d days requires a check-in date", schedule.RepeatDays)
		}
		return nil
	}
	next := *schedule.Next
	if !util.IsStartOfCalendarDate(next) {
		return fmt.Errorf("check-in date %s carries a time of day; a check-in is a calendar day",
			next.Format(time.RFC3339))
	}
	return util.ValidateStorableCalendarDate(next)
}

// NextCheckInAfter returns the check-in date repeatDays calendar days after the
// day from falls on, or nil when the schedule does not repeat. The arithmetic is
// calendar arithmetic: a month, a leap day, and a daylight-saving transition all
// pass under it unchanged, because the date components are shifted and the
// instant is rebuilt once at the end.
//
// from is the day the review actually happened rather than the date that was
// owed, which is what keeps a cadence honest after a schedule is answered late:
// a weekly check-in acknowledged four days late is due a week from today, not
// three days from today.
func NextCheckInAfter(from time.Time, repeatDays int) (*time.Time, error) {
	if err := ValidateCheckInRepeatDays(repeatDays); err != nil {
		return nil, err
	}
	if repeatDays == CheckInRepeatOff {
		return nil, nil
	}
	next, err := util.AddCalendarDays(from, repeatDays)
	if err != nil {
		return nil, fmt.Errorf("next check-in: %w", err)
	}
	if err := util.ValidateStorableCalendarDate(next); err != nil {
		return nil, fmt.Errorf("next check-in: %w", err)
	}
	return &next, nil
}

// CheckInHasArrived reports whether a scheduled check-in date is today or
// earlier. The comparison is by calendar date on both sides, which is what
// "this check-in is due" means regardless of the zone either value was read in;
// util.DaysBetween is the shared authority for that and is daylight-saving safe
// where an hours-over-24 division is not.
func CheckInHasArrived(next *time.Time, now time.Time) bool {
	if next == nil {
		return false
	}
	return util.DaysBetween(*next, now) >= 0
}

// CheckIn returns this task's schedule as a detached value.
func (t *Task) CheckIn() CheckInSchedule {
	if t == nil {
		return CheckInSchedule{}
	}
	return CheckInSchedule{Next: CloneInstant(t.NextCheckIn), RepeatDays: t.CheckInRepeatDays}
}

// LastReviewed returns a detached copy of the review history instant.
func (t *Task) LastReviewed() *time.Time {
	if t == nil {
		return nil
	}
	return CloneInstant(t.LastReviewedAt)
}

// SetCheckIn writes a validated schedule and reports whether anything changed.
// The stored date is a copy of the caller's, so the caller keeps no handle on
// it. UpdatedAt moves only on a real change.
//
// Rescheduling is not an acknowledgement: the review flag, LastReviewedAt, the
// lifecycle state, and its clock are all left exactly as they were, because
// deciding when to look at something again says nothing about having looked at
// it now.
func (t *Task) SetCheckIn(schedule CheckInSchedule, now time.Time) error {
	if t == nil {
		return fmt.Errorf("cannot schedule a check-in on a nil task")
	}
	if err := ValidateCheckInSchedule(schedule); err != nil {
		return err
	}
	if t.sameCheckIn(schedule) {
		return nil
	}
	t.NextCheckIn = CloneInstant(schedule.Next)
	t.CheckInRepeatDays = schedule.RepeatDays
	t.UpdatedAt = now
	return nil
}

// ClearCheckIn removes the schedule whole and reports whether anything changed.
// Review history survives: what was cleared is what is owed, not what happened.
func (t *Task) ClearCheckIn(now time.Time) bool {
	if t == nil || t.CheckIn().IsEmpty() {
		return false
	}
	t.NextCheckIn = nil
	t.CheckInRepeatDays = CheckInRepeatOff
	t.UpdatedAt = now
	return true
}

// sameCheckIn compares a schedule with the stored one by value, so re-declaring
// the schedule a task already holds is not an edit and does not move UpdatedAt.
func (t *Task) sameCheckIn(schedule CheckInSchedule) bool {
	if t.CheckInRepeatDays != schedule.RepeatDays {
		return false
	}
	if t.NextCheckIn == nil || schedule.Next == nil {
		return t.NextCheckIn == nil && schedule.Next == nil
	}
	return t.NextCheckIn.Equal(*schedule.Next)
}

// CheckInIsDueAt reports whether this task's own check-in has arrived and is
// eligible to be raised. Deferral suppresses it: a deferred task has been put
// out of sight deliberately, and a scheduled reminder that reached through that
// would make deferral mean less than it says. Terminal work has no schedule to
// raise at all.
func (t *Task) CheckInIsDueAt(now time.Time) bool {
	if t == nil || t.IsTerminalState() || t.IsDeferredAt(now) {
		return false
	}
	return CheckInHasArrived(t.NextCheckIn, now)
}

// clearCheckInForTerminal drops the schedule a terminal task can no longer owe,
// leaving LastReviewedAt alone. It is called from the one lifecycle primitive
// rather than from each terminal transition, and it is why reopening a finished
// task inherits no schedule: the schedule was gone the moment it finished, and
// reopening mints nothing.
func (t *Task) clearCheckInForTerminal() {
	t.NextCheckIn = nil
	t.CheckInRepeatDays = CheckInRepeatOff
}
