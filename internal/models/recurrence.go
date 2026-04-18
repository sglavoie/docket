package models

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// RecurMode determines the base date used when computing the next occurrence
// after a task is completed.
type RecurMode string

const (
	// RecurModeFromDue computes the next due date relative to the previous due
	// date (default behavior; zero value is treated as this mode).
	RecurModeFromDue RecurMode = "from_due"
	// RecurModeFromCompletion computes the next due date relative to the actual
	// completion timestamp, so late completions push the next due date forward.
	RecurModeFromCompletion RecurMode = "from_completion"
)

// RecurrenceUnit represents the time unit for recurrence patterns.
type RecurrenceUnit string

const (
	RecurrenceDaily   RecurrenceUnit = "day"
	RecurrenceWeekly  RecurrenceUnit = "week"
	RecurrenceMonthly RecurrenceUnit = "month"
	RecurrenceYearly  RecurrenceUnit = "year"
)

// RecurrencePattern defines how a task recurs.
type RecurrencePattern struct {
	Interval    int            `json:"interval"`               // Every N periods (e.g., 2 = every 2 weeks)
	Unit        RecurrenceUnit `json:"unit"`                   // day, week, month, year
	Weekdays    []int          `json:"weekdays,omitempty"`     // For weekly: 0=Sun, 1=Mon, ..., 6=Sat
	Skipped     int            `json:"skipped,omitempty"`      // Number of times this recurrence has been skipped
	Streak      int            `json:"streak,omitempty"`       // Current consecutive on-time completions
	MaxStreak   int            `json:"max_streak,omitempty"`   // Best streak achieved
	OriginalDay int            `json:"original_day,omitempty"` // For monthly: the intended day-of-month (e.g. 31)
	Mode        RecurMode      `json:"mode,omitempty"`         // Base date for next-due computation (empty = FromDue)
}

// validRecurrenceUnits is the set of recognized recurrence time units.
var validRecurrenceUnits = map[RecurrenceUnit]bool{
	RecurrenceDaily:   true,
	RecurrenceWeekly:  true,
	RecurrenceMonthly: true,
	RecurrenceYearly:  true,
}

// MaxRecurrenceInterval is the maximum allowed recurrence interval.
// 3650 covers ~10 years for daily recurrence, which is a reasonable upper bound.
const MaxRecurrenceInterval = 3650

// Validate checks that the RecurrencePattern has valid field values.
// Returns an error describing the first invalid field found, or nil if valid.
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
		return fmt.Errorf("invalid recurrence unit %q, must be one of: day, week, month, year", r.Unit)
	}
	for _, wd := range r.Weekdays {
		if wd < 0 || wd > 6 {
			return fmt.Errorf("invalid weekday %d, must be 0-6 (Sun-Sat)", wd)
		}
	}
	if r.Mode != "" && r.Mode != RecurModeFromDue && r.Mode != RecurModeFromCompletion {
		return fmt.Errorf("invalid recurrence mode %q, must be %q or %q", r.Mode, RecurModeFromDue, RecurModeFromCompletion)
	}
	return nil
}

// String returns a human-readable representation of the recurrence pattern.
// A trailing "~" is appended when Mode is RecurModeFromCompletion.
func (r *RecurrencePattern) String() string {
	if r == nil {
		return ""
	}
	var base string
	if r.Interval == 1 {
		switch r.Unit {
		case RecurrenceDaily:
			base = "daily"
		case RecurrenceWeekly:
			if len(r.Weekdays) > 0 {
				base = fmt.Sprintf("weekly on %s", weekdaysToString(r.Weekdays))
			} else {
				base = "weekly"
			}
		case RecurrenceMonthly:
			base = "monthly"
		case RecurrenceYearly:
			base = "yearly"
		}
	}
	if base == "" {
		base = fmt.Sprintf("every %d %ss", r.Interval, r.Unit)
		if r.Unit == RecurrenceWeekly && len(r.Weekdays) > 0 {
			base = fmt.Sprintf("every %d %ss on %s", r.Interval, r.Unit, weekdaysToString(r.Weekdays))
		}
	}
	if r.Mode == RecurModeFromCompletion {
		return base + "~"
	}
	return base
}

// weekdaysToString converts weekday numbers to abbreviated names.
func weekdaysToString(weekdays []int) string {
	names := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	var result []string
	for _, d := range weekdays {
		if d >= 0 && d < 7 {
			result = append(result, names[d])
		}
	}
	if len(result) == 0 {
		return ""
	}
	if len(result) == 1 {
		return result[0]
	}
	// Join with commas, last one with "and"
	if len(result) == 2 {
		return result[0] + " and " + result[1]
	}
	return fmt.Sprintf("%s, and %s", strings.Join(result[:len(result)-1], ", "), result[len(result)-1])
}

// calculateNextDate computes the next occurrence date from a base date.
func calculateNextDate(base time.Time, r *RecurrencePattern) time.Time {
	switch r.Unit {
	case RecurrenceDaily:
		return base.AddDate(0, 0, r.Interval)

	case RecurrenceWeekly:
		if len(r.Weekdays) == 0 {
			// Simple weekly: add N weeks
			return base.AddDate(0, 0, r.Interval*7)
		}
		// Weekly with specific days: find next matching weekday
		return findNextWeekday(base, r.Weekdays, r.Interval)

	case RecurrenceMonthly:
		return addMonths(base, r.Interval, r.OriginalDay)

	case RecurrenceYearly:
		return addYears(base, r.Interval)

	default:
		// Fallback to daily
		return base.AddDate(0, 0, r.Interval)
	}
}

// findNextWeekday finds the next occurrence on one of the specified weekdays.
// Same-week occurrences are returned without applying the interval. The interval
// (N weeks) is only applied when wrapping past the end of the current week to
// the first matching day of a future week. For example, "every 2 weeks on
// Mon,Fri" from Monday returns Friday of the same week, then Monday 2 weeks
// after that Friday's week.
func findNextWeekday(base time.Time, weekdays []int, interval int) time.Time {
	if len(weekdays) == 0 {
		return base.AddDate(0, 0, interval*7)
	}

	// Sort weekdays for predictable iteration
	sortedDays := make([]int, len(weekdays))
	copy(sortedDays, weekdays)
	sort.Ints(sortedDays)

	currentDay := int(base.Weekday())

	// Find the next weekday after today in the same week (no interval skip)
	for _, wd := range sortedDays {
		if wd > currentDay {
			return base.AddDate(0, 0, wd-currentDay)
		}
	}

	// No more days this week; go to first day of next interval
	daysToFirstDay := (7 - currentDay) + sortedDays[0]
	if interval > 1 {
		daysToFirstDay += (interval - 1) * 7
	}
	return base.AddDate(0, 0, daysToFirstDay)
}

// addMonths adds N months to a date, handling end-of-month edge cases.
// If the target month has fewer days, clamps to the last day of that month.
// originalDay, when > 0, is the intended day-of-month (e.g. 31 for Jan 31);
// it is used instead of base.Day() when larger, so that months with 31 days
// snap back to the 31st rather than drifting to 28/30 permanently.
func addMonths(base time.Time, months int, originalDay int) time.Time {
	year, month, day := base.Date()
	hour, min, sec := base.Clock()

	// Restore the intended day if OriginalDay is set and larger than base.Day().
	if originalDay > day {
		day = originalDay
	}

	targetMonth := int(month) + months
	targetYear := year + (targetMonth-1)/12
	targetMonth = ((targetMonth - 1) % 12) + 1
	if targetMonth <= 0 {
		targetMonth += 12
		targetYear--
	}

	// Find the last day of the target month
	lastDay := daysInMonth(targetYear, time.Month(targetMonth))
	if day > lastDay {
		day = lastDay
	}

	return time.Date(targetYear, time.Month(targetMonth), day, hour, min, sec, base.Nanosecond(), base.Location())
}

// addYears adds N years to a date, clamping to the last day of the target month
// when the original day doesn't exist (e.g. Feb 29 in a non-leap year → Feb 28).
func addYears(base time.Time, years int) time.Time {
	year, month, day := base.Date()
	hour, min, sec := base.Clock()

	targetYear := year + years
	lastDay := daysInMonth(targetYear, month)
	if day > lastDay {
		day = lastDay
	}

	return time.Date(targetYear, month, day, hour, min, sec, base.Nanosecond(), base.Location())
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	// Go to the first day of the next month, then subtract one day
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// ParseRecurrence parses a recurrence string into a RecurrencePattern.
// Supported formats:
//   - "daily", "weekly", "monthly", "yearly"
//   - "2d" (every 2 days), "3w" (every 3 weeks), "2m" (every 2 months), "1y" (every year)
//   - "weekly:mon,wed,fri" (weekly on specific days)
//
// A trailing "~" on any form sets Mode to RecurModeFromCompletion:
// "daily~", "2d~", "weekly:mon,fri~", "3w:mon~", etc.
//
// Returns nil if the string cannot be parsed.
func ParseRecurrence(s string) *RecurrencePattern {
	if s == "" {
		return nil
	}

	// Strip trailing ~ and record mode.
	mode := RecurModeFromDue
	if strings.HasSuffix(s, "~") {
		mode = RecurModeFromCompletion
		s = s[:len(s)-1]
		if s == "" {
			return nil
		}
	}

	var p *RecurrencePattern

	// Handle simple keywords
	switch s {
	case "daily":
		p = &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}
	case "weekly":
		p = &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}
	case "biweekly":
		p = &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}
	case "monthly":
		p = &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	case "yearly":
		p = &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}
	}

	if p == nil && len(s) >= 2 {
		// Check for weekly:days format first
		if len(s) > 7 && s[:7] == "weekly:" {
			weekdays := parseWeekdays(s[7:])
			if len(weekdays) > 0 {
				p = &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: weekdays}
			}
		}

		if p == nil {
			// Check for Nw:days format (e.g., "2w:mon,fri")
			colonIdx := -1
			for i, c := range s {
				if c == ':' {
					colonIdx = i
					break
				}
			}
			if colonIdx > 1 {
				basePattern := parseIntervalUnit(s[:colonIdx])
				if basePattern != nil && basePattern.Unit == RecurrenceWeekly {
					weekdays := parseWeekdays(s[colonIdx+1:])
					if len(weekdays) > 0 {
						basePattern.Weekdays = weekdays
						p = basePattern
					}
				}
			}
		}

		if p == nil {
			// Simple interval format (2d, 3w, etc.)
			p = parseIntervalUnit(s)
		}
	}

	if p != nil && mode == RecurModeFromCompletion {
		p.Mode = RecurModeFromCompletion
	}
	return p
}

// parseIntervalUnit parses formats like "2d", "3w", "1m", "2y".
func parseIntervalUnit(s string) *RecurrencePattern {
	if len(s) < 2 {
		return nil
	}

	// Parse the number part
	numStr := s[:len(s)-1]
	unitChar := s[len(s)-1]

	interval := 0
	for _, c := range numStr {
		if c < '0' || c > '9' {
			return nil
		}
		// Guard against overflow before multiplying: if already above max/10,
		// multiplying by 10 could overflow before the MaxRecurrenceInterval check.
		if interval > MaxRecurrenceInterval/10 {
			return nil
		}
		interval = interval*10 + int(c-'0')
		if interval > MaxRecurrenceInterval {
			return nil // Reject absurdly large intervals early
		}
	}
	if interval <= 0 {
		return nil
	}

	var unit RecurrenceUnit
	switch unitChar {
	case 'd':
		unit = RecurrenceDaily
	case 'w':
		unit = RecurrenceWeekly
	case 'm':
		unit = RecurrenceMonthly
	case 'y':
		unit = RecurrenceYearly
	default:
		return nil
	}

	return &RecurrencePattern{Interval: interval, Unit: unit}
}

// parseWeekdays parses a comma-separated list of weekday abbreviations.
// Returns slice of weekday numbers (0=Sun, 1=Mon, ..., 6=Sat).
func parseWeekdays(s string) []int {
	dayMap := map[string]int{
		"sun": 0, "sunday": 0,
		"mon": 1, "monday": 1,
		"tue": 2, "tuesday": 2,
		"wed": 3, "wednesday": 3,
		"thu": 4, "thursday": 4,
		"fri": 5, "friday": 5,
		"sat": 6, "saturday": 6,
	}

	var result []int
	seen := make(map[int]bool)

	// Split by comma
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			// Trim spaces and convert to lowercase
			part = strings.TrimSpace(strings.ToLower(part))
			if day, ok := dayMap[part]; ok {
				if !seen[day] {
					result = append(result, day)
					seen[day] = true
				}
			}
			start = i + 1
		}
	}

	return result
}
