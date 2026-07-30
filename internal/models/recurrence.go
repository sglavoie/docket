package models

import (
	"fmt"
	"strconv"
	"time"
)

// RecurrenceUnit is a calendar unit supported by canonical recurrence.
type RecurrenceUnit string

const (
	RecurrenceDaily   RecurrenceUnit = "day"
	RecurrenceWeekly  RecurrenceUnit = "week"
	RecurrenceMonthly RecurrenceUnit = "month"
)

// RecurrencePattern defines the canonical recurrence interval.
type RecurrencePattern struct {
	Interval int            `json:"interval"`
	Unit     RecurrenceUnit `json:"unit"`
}

var validRecurrenceUnits = map[RecurrenceUnit]bool{
	RecurrenceDaily:   true,
	RecurrenceWeekly:  true,
	RecurrenceMonthly: true,
}

// MaxRecurrenceInterval bounds an interval before calendar arithmetic.
const MaxRecurrenceInterval = 3650

// Validate reports whether the pattern is canonical and safe to schedule.
func (r *RecurrencePattern) Validate() error {
	if r == nil {
		return fmt.Errorf("recurrence pattern is nil")
	}
	if r.Interval <= 0 {
		return fmt.Errorf("recurrence interval must be > 0, got %d", r.Interval)
	}
	if r.Interval > MaxRecurrenceInterval {
		return fmt.Errorf("recurrence interval too large (%d), maximum is %d", r.Interval, MaxRecurrenceInterval)
	}
	if !validRecurrenceUnits[r.Unit] {
		return fmt.Errorf("invalid recurrence unit %q, must be one of: day, week, month", r.Unit)
	}
	return nil
}

// String returns a human-readable canonical recurrence expression.
func (r *RecurrencePattern) String() string {
	if r == nil {
		return ""
	}
	if r.Interval <= 0 || r.Interval > MaxRecurrenceInterval || !validRecurrenceUnits[r.Unit] {
		return ""
	}
	if r.Interval == 1 {
		switch r.Unit {
		case RecurrenceDaily:
			return "daily"
		case RecurrenceWeekly:
			return "weekly"
		case RecurrenceMonthly:
			return "monthly"
		}
	}
	return "every " + strconv.Itoa(r.Interval) + " " + pluralRecurrenceUnit(r.Unit)
}

func pluralRecurrenceUnit(unit RecurrenceUnit) string {
	switch unit {
	case RecurrenceDaily:
		return "days"
	case RecurrenceWeekly:
		return "weeks"
	case RecurrenceMonthly:
		return "months"
	default:
		return ""
	}
}

// calculateNextDate advances one scheduled interval from base.
func calculateNextDate(base time.Time, recurrence *RecurrencePattern) time.Time {
	switch recurrence.Unit {
	case RecurrenceDaily:
		return base.AddDate(0, 0, recurrence.Interval)
	case RecurrenceWeekly:
		return base.AddDate(0, 0, recurrence.Interval*7)
	case RecurrenceMonthly:
		return addMonths(base, recurrence.Interval)
	default:
		return base
	}
}

// addMonths advances by whole calendar months. A due date on the last day of a
// month stays on the last day, so Jan 31 -> Feb 29 -> Mar 31 does not drift.
func addMonths(base time.Time, months int) time.Time {
	year, month, day := base.Date()
	hour, minute, second := base.Clock()
	wasEndOfMonth := day == daysInMonth(year, month)

	targetMonth := int(month) + months
	targetYear := year + (targetMonth-1)/12
	targetMonth = ((targetMonth - 1) % 12) + 1
	if targetMonth <= 0 {
		targetMonth += 12
		targetYear--
	}

	lastDay := daysInMonth(targetYear, time.Month(targetMonth))
	if wasEndOfMonth || day > lastDay {
		day = lastDay
	}
	return time.Date(targetYear, time.Month(targetMonth), day, hour, minute, second, base.Nanosecond(), base.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// calendarDate returns the date component at midnight in the value's location.
func calendarDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

// calendarDateAfter compares calendar components rather than instants. Due dates
// are date-only values, so location offsets must not change catch-up behavior.
func calendarDateAfter(left, right time.Time) bool {
	leftYear, leftMonth, leftDay := left.Date()
	rightYear, rightMonth, rightDay := right.Date()
	leftDate := time.Date(leftYear, leftMonth, leftDay, 0, 0, 0, 0, time.UTC)
	rightDate := time.Date(rightYear, rightMonth, rightDay, 0, 0, 0, 0, time.UTC)
	return leftDate.After(rightDate)
}

// ParseRecurrence parses canonical day/week/month expressions.
//
// Supported forms are daily, weekly, biweekly, monthly, and a positive integer
// followed by d, w, or m. Legacy yearly, weekday, and mode syntax is rejected
// and remains migration input only.
func ParseRecurrence(value string) *RecurrencePattern {
	switch value {
	case "daily":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}
	case "weekly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}
	case "biweekly":
		return &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}
	case "monthly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	}
	return parseIntervalUnit(value)
}

func parseIntervalUnit(value string) *RecurrencePattern {
	if len(value) < 2 {
		return nil
	}
	interval, err := strconv.Atoi(value[:len(value)-1])
	if err != nil || interval <= 0 || interval > MaxRecurrenceInterval {
		return nil
	}
	var unit RecurrenceUnit
	switch value[len(value)-1] {
	case 'd':
		unit = RecurrenceDaily
	case 'w':
		unit = RecurrenceWeekly
	case 'm':
		unit = RecurrenceMonthly
	default:
		return nil
	}
	return &RecurrencePattern{Interval: interval, Unit: unit}
}
