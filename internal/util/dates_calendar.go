package util

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// This file owns whole-day calendar arithmetic, as distinct from elapsed
// duration. A calendar day is one day whether the zone spent 23, 24, or 25
// hours in it, so a value that means "a date" is shifted by rewriting its date
// components and never by adding a multiple of Day to an instant. StartOfDay,
// DaysBetween, and AddMonthsClamped in dates.go are the other half of the same
// discipline; what is new here is shifting by whole days and refusing a result
// the durable document could not spell back.

// AddCalendarDays returns the start of the calendar day days whole days after
// t, in t's own location. The shift is computed by handing time.Date the
// unnormalized day number in UTC, which is what carries month, year, and
// leap-day boundaries correctly; the instant is rebuilt in t's location only
// once, at the end, where a zone that skips the destination date outright can
// be refused instead of silently answering with the surviving neighbor.
//
// The returned instant is the earliest one belonging to that date rather than
// literally midnight: a zone whose daylight-saving transition happens at 00:00
// has no 00:00 on that date, and the start of the date is 01:00 there. That is
// the date that was asked for, so it is returned; only a date the location has
// none of at all is an error.
func AddCalendarDays(t time.Time, days int) (time.Time, error) {
	if days > maxRelativeOffset || days < -maxRelativeOffset {
		return time.Time{}, fmt.Errorf("calendar offset of %d days is out of range (max %d)", days, maxRelativeOffset)
	}
	year, month, day := time.Date(t.Year(), t.Month(), t.Day()+days, 0, 0, 0, 0, time.UTC).Date()
	return StartOfCalendarDate(year, month, day, t.Location())
}

// StartOfCalendarDaysAfter is AddCalendarDays for a caller with no error to
// report, such as a picker row computing its value: the start of the calendar
// day days whole days after t's date, in t's location, never an instant stepped
// by AddDate from a day-start (which a zone skipping midnight turns into the
// wrong date or a time of day). Where AddCalendarDays refuses — the offset is
// out of its range, or the zone has no such date at all — it answers with the
// start of the date holding noon of that civil day, which is the surviving
// neighbor the zone itself resolves to.
func StartOfCalendarDaysAfter(t time.Time, days int) time.Time {
	if start, err := AddCalendarDays(t, days); err == nil {
		return start
	}
	return StartOfDay(time.Date(t.Year(), t.Month(), t.Day()+days, 12, 0, 0, 0, t.Location()))
}

// StartOfCalendarDate rebuilds the start of one named calendar date in loc, and
// refuses a date loc does not have. time.Date normalizes rather than fails, so
// asking whether the date exists has to be a separate question from asking where
// it starts: Pacific/Apia lost 2011-12-30 outright, every hour of it, and
// normalizing there substitutes a neighboring date with nothing downstream able
// to tell.
//
// Existence is decided at noon, which no daylight-saving transition has ever
// moved, so the answer does not depend on which of the two zones time.Date
// happened to resolve a gap in. Midnight is only then consulted, and only to
// learn whether it is in the date: a zone whose transition happens at 00:00 has
// no midnight on that date, and the start of the date is the transition instant
// itself — America/Havana starts 2026-03-08 at 01:00, and Go answers a midnight
// there with 23:00 on the 7th, which is the right instant under the wrong
// question. The search below asks the right one.
func StartOfCalendarDate(year int, month time.Month, day int, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	noon := time.Date(year, month, day, 12, 0, 0, 0, loc)
	if !isCalendarDate(noon, year, month, day) {
		return time.Time{}, fmt.Errorf("%04d-%02d-%02d does not exist in %s", year, int(month), day, loc)
	}
	if start := time.Date(year, month, day, 0, 0, 0, 0, loc); isCalendarDate(start, year, month, day) {
		return start, nil
	}
	// Midnight fell outside the date, so the date starts somewhere between the
	// previous noon and this one. Local date is nondecreasing in that window, so
	// the earliest instant belonging to it is a plain bisection; zone offsets are
	// whole seconds, so a second is the finest step worth taking.
	before, after := noon.Add(-24*time.Hour), noon
	// A day before noon is outside the date in every zone that has ever existed,
	// but the bisection below is only correct while it is, so widen rather than
	// assume: Pacific/Apia once moved a local clock a whole day at once.
	for span := 2; isCalendarDate(before, year, month, day) && span <= 8; span *= 2 {
		before = noon.Add(time.Duration(-span) * 24 * time.Hour)
	}
	// The search runs over whole Unix seconds so the answer is the transition
	// second itself: halving an instant span would leave a fractional remainder
	// that is inside the date but not its start.
	lo, hi := before.Unix(), after.Unix()
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if isCalendarDate(time.Unix(mid, 0).In(loc), year, month, day) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return time.Unix(hi, 0).In(loc), nil
}

// IsStartOfCalendarDate reports whether t is the earliest instant of its own
// calendar date in its own location. It is the question "does this value carry a
// time of day" asked correctly: comparing against StartOfDay answers it with
// time.Date's gap resolution, which says no for the legitimate 01:00 start of a
// date whose zone has no midnight on it.
func IsStartOfCalendarDate(t time.Time) bool {
	start, err := StartOfCalendarDate(t.Year(), t.Month(), t.Day(), t.Location())
	return err == nil && start.Equal(t)
}

// isCalendarDate reports whether an instant falls on one named calendar date in
// its own location.
func isCalendarDate(t time.Time, year int, month time.Month, day int) bool {
	gotYear, gotMonth, gotDay := t.Date()
	return gotYear == year && gotMonth == month && gotDay == day
}

// ValidateStorableCalendarDate holds a calendar date to the canonical spelling
// the durable document stores, so a date the fixed-width YYYY-MM-DD layout
// cannot round-trip is refused before it is written rather than after it is read
// back as something else. A year outside 0001-9999 is the practical case: it
// renders with a sign or an extra digit and no longer parses as the layout it
// was written under.
func ValidateStorableCalendarDate(t time.Time) error {
	value := t.Format(DateFormat)
	parsed, err := time.Parse(DateFormat, value)
	if err != nil || parsed.Format(DateFormat) != value {
		return fmt.Errorf("date %q is not a canonical %s value", value, DateFormat)
	}
	return nil
}

// ErrCheckInsAreDateOnly is the refusal a check-in value earns for carrying a
// time of day. It is exported because the surfaces that collect the value name
// it to tell a time-bearing input apart from an unparseable one.
var ErrCheckInsAreDateOnly = errors.New("a check-in is a calendar day; drop the time of day")

// ParseCheckInDateAt parses a check-in value using ParseCalendarDate's grammar
// with calendar-safe arithmetic. The hour form (+Nh) stays defer-only and a
// trailing clock is refused by name: a check-in may not carry an hour. A caller
// that typed one gets told which value it is setting rather than a generic
// format error.
//
// An explicit "00:00" is refused with the same message even though it resolves
// to the same instant the bare date does. ExtractDueClockSuffix reads a midnight
// clock as the date-only signal, which is right for a due date whose storage has
// a place to put an hour; here it would mean silently accepting the one spelling
// of a time that happens to be harmless, and the value a surface reads back
// would not be the value it was given.
//
// It resolves a check-in relative to now's calendar date and
// location. All date arithmetic happens on civil components before resolving
// the start of the result in that location, including dates without midnight.
func ParseCheckInDateAt(input string, now time.Time) (time.Time, error) {
	datePart, _, _, _, err := ExtractDueClockSuffix(input)
	if err != nil {
		return time.Time{}, err
	}
	if datePart != strings.TrimSpace(input) {
		return time.Time{}, ErrCheckInsAreDateOnly
	}
	datePart = normalizeDateInput(datePart)
	if relativeHoursRe.MatchString(datePart) {
		return time.Time{}, errHoursAreDeferOnly
	}
	// Copy the local date components, rather than converting now to UTC (which
	// can change the date). Resolve both written and relative forms in UTC so a
	// nonexistent local midnight cannot silently move the requested date back.
	civil, exact := parseExactDateIn(datePart, time.UTC)
	if !exact {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		civil, err = parseRelativeDateFromDay(datePart, "sun", today)
		if err != nil {
			return time.Time{}, err
		}
	}
	day, err := StartOfCalendarDate(civil.Year(), civil.Month(), civil.Day(), now.Location())
	if err != nil {
		return time.Time{}, err
	}
	if err := ValidateStorableCalendarDate(day); err != nil {
		return time.Time{}, err
	}
	return day, nil
}
