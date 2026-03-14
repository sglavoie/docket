package models

import (
	"fmt"
	"sort"
	"strings"
	"time"
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
	Interval  int            `json:"interval"`             // Every N periods (e.g., 2 = every 2 weeks)
	Unit      RecurrenceUnit `json:"unit"`                 // day, week, month, year
	Weekdays  []int          `json:"weekdays,omitempty"`   // For weekly: 0=Sun, 1=Mon, ..., 6=Sat
	Skipped   int            `json:"skipped,omitempty"`    // Number of times this recurrence has been skipped
	Streak    int            `json:"streak,omitempty"`     // Current consecutive on-time completions
	MaxStreak int            `json:"max_streak,omitempty"` // Best streak achieved
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
	return nil
}

// String returns a human-readable representation of the recurrence pattern.
func (r *RecurrencePattern) String() string {
	if r == nil {
		return ""
	}
	if r.Interval == 1 {
		switch r.Unit {
		case RecurrenceDaily:
			return "daily"
		case RecurrenceWeekly:
			if len(r.Weekdays) > 0 {
				return fmt.Sprintf("weekly on %s", weekdaysToString(r.Weekdays))
			}
			return "weekly"
		case RecurrenceMonthly:
			return "monthly"
		case RecurrenceYearly:
			return "yearly"
		}
	}
	return fmt.Sprintf("every %d %ss", r.Interval, r.Unit)
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
		return addMonths(base, r.Interval)

	case RecurrenceYearly:
		return base.AddDate(r.Interval, 0, 0)

	default:
		// Fallback to daily
		return base.AddDate(0, 0, r.Interval)
	}
}

// findNextWeekday finds the next occurrence on one of the specified weekdays.
// If interval > 1, it advances by that many weeks first.
func findNextWeekday(base time.Time, weekdays []int, interval int) time.Time {
	if len(weekdays) == 0 {
		return base.AddDate(0, 0, interval*7)
	}

	// Sort weekdays for predictable iteration
	sortedDays := make([]int, len(weekdays))
	copy(sortedDays, weekdays)
	sort.Ints(sortedDays)

	currentDay := int(base.Weekday())

	// Find the next weekday after today in the same week
	for _, wd := range sortedDays {
		if wd > currentDay {
			daysToAdd := wd - currentDay
			if interval > 1 {
				daysToAdd += (interval - 1) * 7
			}
			return base.AddDate(0, 0, daysToAdd)
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
func addMonths(base time.Time, months int) time.Time {
	year, month, day := base.Date()
	hour, min, sec := base.Clock()

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
// Returns nil if the string cannot be parsed.
func ParseRecurrence(s string) *RecurrencePattern {
	if s == "" {
		return nil
	}

	// Handle simple keywords
	switch s {
	case "daily":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}
	case "weekly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}
	case "biweekly":
		return &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}
	case "monthly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}
	case "yearly":
		return &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}
	}

	// Handle Nd, Nw, Nm, Ny format
	if len(s) >= 2 {
		// Check for weekly:days format first
		if len(s) > 7 && s[:7] == "weekly:" {
			weekdays := parseWeekdays(s[7:])
			if len(weekdays) > 0 {
				return &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: weekdays}
			}
		}

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
					return basePattern
				}
			}
		}

		// Simple interval format (2d, 3w, etc.)
		return parseIntervalUnit(s)
	}

	return nil
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
