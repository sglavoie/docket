package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Standard date format constants for consistent formatting across the codebase.
const (
	DateFormat        = "2006-01-02"       // Date only (YYYY-MM-DD)
	DateTimeFormat    = "2006-01-02 15:04" // Date and time (YYYY-MM-DD HH:MM)
	DateTimeDueFormat = "2006-01-02T15:04" // ISO-compatible due date+time (YYYY-MM-DDTHH:MM)
)

// Common time durations for date arithmetic and comparisons.
const (
	Day   = 24 * time.Hour
	Week  = 7 * Day
	Month = 30 * Day
	Year  = 365 * Day
)

// StartOfDay returns t truncated to midnight (00:00:00) in t's location.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// DaysBetween returns the number of calendar days from a to b (b - a).
// This is DST-safe unlike computing hours/24, which can be off by one
// around daylight saving transitions.
func DaysBetween(a, b time.Time) int {
	aDate := time.Date(a.Year(), a.Month(), a.Day(), 12, 0, 0, 0, time.UTC)
	bDate := time.Date(b.Year(), b.Month(), b.Day(), 12, 0, 0, 0, time.UTC)
	return int(bDate.Sub(aDate).Hours() / 24)
}

// Pre-compiled regexps for ParseRelativeDate to avoid recompilation on each call.
var (
	relativeDaysRe   = regexp.MustCompile(`^\+(\d+)d$`)
	relativeWeeksRe  = regexp.MustCompile(`^\+(\d+)w$`)
	relativeMonthsRe = regexp.MustCompile(`^\+(\d+)m$`)
	nextWeekdayRe    = regexp.MustCompile(`^next\s+(sun|sunday|mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday)$`)
	thisWeekdayRe    = regexp.MustCompile(`^this\s+(sun|sunday|mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday)$`)
	inDurationRe     = regexp.MustCompile(`^in\s+(\d+)\s+(day|days|week|weeks|month|months|year|years)$`)
	time12hRe        = regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?(am|pm)$`)
	time24hRe        = regexp.MustCompile(`^(\d{2}):(\d{2})$`)
	flexShorthandRe  = regexp.MustCompile(`^(\d+)(d|w|m)$`)
	monthDayRe       = regexp.MustCompile(`^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+(\d{1,2})$`)
	dayMonthRe       = regexp.MustCompile(`^(\d{1,2})\s+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)$`)
	nextYearRe       = regexp.MustCompile(`^next\s+year$`)
)

// months maps abbreviated month names to time.Month values.
var months = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March,
	"apr": time.April, "may": time.May, "jun": time.June,
	"jul": time.July, "aug": time.August, "sep": time.September,
	"oct": time.October, "nov": time.November, "dec": time.December,
}

// weekdays maps weekday name variations to time.Weekday values.
var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// isoWeekdayNum maps a time.Weekday to its ISO week position: Monday=1 … Sunday=7.
func isoWeekdayNum(wd time.Weekday) int {
	if wd == time.Sunday {
		return 7
	}
	return int(wd)
}

// ParseRelativeDate parses a natural language date string into a time.Time.
// Supported formats:
//   - YYYY-MM-DD (e.g., 2024-01-15)
//   - MM/DD/YYYY (e.g., 01/15/2024)
//   - DD.MM.YYYY (e.g., 15.01.2024)
//   - today, tomorrow, yesterday
//   - eod (end of day = today), eom (end of month), eoy (end of year)
//   - eow (end of week — uses Sunday by default; see ParseRelativeDateWithConfig)
//   - +Nd (N days from now), +Nw (N weeks), +Nm (N months)
//   - mon, tue, wed, thu, fri, sat, sun (next occurrence)
//   - this <weekday> (this ISO-week occurrence; error if already past)
//   - next <weekday> (skips this week)
//   - next week, next month, next year
//   - in N days/weeks/months/years
//   - dec 15, jan 3 (month day; next year if already past)
//   - 15 dec, 3 jan (day month; same logic)
//
// Uses Sunday as the configured week-end day for eow. Use
// ParseRelativeDateWithConfig to pass a user-configured week-end day.
func ParseRelativeDate(input string) (time.Time, error) {
	return ParseRelativeDateWithConfig(input, "sun")
}

// ParseRelativeDateWithConfig parses a natural language date string like
// ParseRelativeDate, but respects the weekEndDay setting for the eow token.
// weekEndDay must be "fri", "sat", or "sun" (invalid values default to "sun").
func ParseRelativeDateWithConfig(input, weekEndDay string) (time.Time, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	now := time.Now()
	today := StartOfDay(now)

	// Exact date: YYYY-MM-DD
	if date, err := time.ParseInLocation(DateFormat, input, time.Local); err == nil {
		return date, nil
	}

	// Exact date: MM/DD/YYYY
	if date, err := time.ParseInLocation("01/02/2006", input, time.Local); err == nil {
		return date, nil
	}

	// Exact date: DD.MM.YYYY (European format)
	if date, err := time.ParseInLocation("02.01.2006", input, time.Local); err == nil {
		return date, nil
	}

	// Keywords
	switch input {
	case "today", "eod":
		return today, nil
	case "tomorrow", "tmrw", "tom":
		return today.AddDate(0, 0, 1), nil
	case "yesterday", "ytd":
		return today.AddDate(0, 0, -1), nil
	case "eow":
		var targetDay time.Weekday
		switch weekEndDay {
		case "fri":
			targetDay = time.Friday
		case "sat":
			targetDay = time.Saturday
		default:
			targetDay = time.Sunday
		}
		daysUntil := int(targetDay) - int(today.Weekday())
		if daysUntil < 0 {
			daysUntil += 7
		}
		return today.AddDate(0, 0, daysUntil), nil
	case "eom":
		return time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()), nil
	case "eoy":
		return time.Date(today.Year(), 12, 31, 0, 0, 0, 0, today.Location()), nil
	}

	// Relative: +Nd (days)
	if match := relativeDaysRe.FindStringSubmatch(input); len(match) == 2 {
		days, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number of days in %q", input)
		}
		return today.AddDate(0, 0, days), nil
	}

	// Relative: +Nw (weeks)
	if match := relativeWeeksRe.FindStringSubmatch(input); len(match) == 2 {
		weeks, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number of weeks in %q", input)
		}
		return today.AddDate(0, 0, weeks*7), nil
	}

	// Relative: +Nm (months)
	if match := relativeMonthsRe.FindStringSubmatch(input); len(match) == 2 {
		months, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number of months in %q", input)
		}
		return today.AddDate(0, months, 0), nil
	}

	// "this <weekday>" - this ISO-week occurrence; error if already past.
	// Uses ISO week (Monday=1 … Sunday=7) so "this sun" on a Monday points
	// forward to the end of the week, and "this mon" on a Sunday is past.
	if match := thisWeekdayRe.FindStringSubmatch(input); len(match) == 2 {
		targetDay := weekdays[match[1]]
		daysOffset := isoWeekdayNum(targetDay) - isoWeekdayNum(today.Weekday())
		if daysOffset < 0 {
			dayName := match[1]
			return time.Time{}, fmt.Errorf("'this %s' is in the past; did you mean 'next %s'?", dayName, dayName)
		}
		return today.AddDate(0, 0, daysOffset), nil
	}

	// Weekday names (e.g., "mon", "friday") - next occurrence within 1-7 days
	if targetDay, ok := weekdays[input]; ok {
		daysUntil := int(targetDay) - int(today.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		return today.AddDate(0, 0, daysUntil), nil
	}

	// "next week" / "next month" / "next year"
	if input == "next week" {
		return today.AddDate(0, 0, 7), nil
	}
	if input == "next month" {
		return today.AddDate(0, 1, 0), nil
	}
	if nextYearRe.MatchString(input) {
		return today.AddDate(1, 0, 0), nil
	}

	// "next <weekday>" - always at least 7 days away (skips this week)
	if match := nextWeekdayRe.FindStringSubmatch(input); len(match) == 2 {
		targetDay := weekdays[match[1]]
		daysUntil := int(targetDay) - int(today.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		daysUntil += 7 // "next" means skip this week
		return today.AddDate(0, 0, daysUntil), nil
	}

	// "in N days/weeks/months/years"
	if match := inDurationRe.FindStringSubmatch(input); len(match) == 3 {
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration in %q", input)
		}
		switch match[2] {
		case "day", "days":
			return today.AddDate(0, 0, n), nil
		case "week", "weeks":
			return today.AddDate(0, 0, n*7), nil
		case "month", "months":
			return today.AddDate(0, n, 0), nil
		case "year", "years":
			return today.AddDate(n, 0, 0), nil
		}
	}

	// "dec 15" or "15 dec" — month name + day number
	if match := monthDayRe.FindStringSubmatch(input); len(match) == 3 {
		return parseMonthDay(match[1], match[2], today)
	}
	if match := dayMonthRe.FindStringSubmatch(input); len(match) == 3 {
		return parseMonthDay(match[2], match[1], today)
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s", input)
}

// parseMonthDay builds a date from a 3-letter month abbreviation and a day string.
// If the resulting date is in the past, the next occurrence (next year) is returned.
// Returns an error if the day is out of range for the given month.
func parseMonthDay(monthStr, dayStr string, today time.Time) (time.Time, error) {
	month, ok := months[monthStr]
	if !ok {
		return time.Time{}, fmt.Errorf("unrecognized month: %s", monthStr)
	}
	day, _ := strconv.Atoi(dayStr)
	year := today.Year()
	candidate := time.Date(year, month, day, 0, 0, 0, 0, today.Location())
	// Validate: time.Date normalises out-of-range days (e.g. Feb 30 → Mar 2).
	// If the resulting month/day differs, the input day is invalid for this month.
	if candidate.Month() != month || candidate.Day() != day {
		return time.Time{}, fmt.Errorf("invalid date: %s %d", monthStr, day)
	}
	if !candidate.After(today) {
		candidate = time.Date(year+1, month, day, 0, 0, 0, 0, today.Location())
	}
	return candidate, nil
}

// ParseFlexibleDate is like ParseRelativeDate but also accepts bare shorthand
// without the leading +: "3d" (3 days), "2w" (2 weeks), "1m" (1 month).
func ParseFlexibleDate(input string) (time.Time, error) {
	return ParseFlexibleDateWithConfig(input, "sun")
}

// ParseFlexibleDateWithConfig is like ParseRelativeDateWithConfig but also
// accepts bare shorthand: "3d", "2w", "1m".
func ParseFlexibleDateWithConfig(input, weekEndDay string) (time.Time, error) {
	t, err := ParseRelativeDateWithConfig(input, weekEndDay)
	if err == nil {
		return t, nil
	}
	if match := flexShorthandRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(input))); len(match) == 3 {
		n, atoiErr := strconv.Atoi(match[1])
		if atoiErr != nil {
			return time.Time{}, fmt.Errorf("invalid number in %q", input)
		}
		today := StartOfDay(time.Now())
		switch match[2] {
		case "d":
			return today.AddDate(0, 0, n), nil
		case "w":
			return today.AddDate(0, 0, n*7), nil
		case "m":
			return today.AddDate(0, n, 0), nil
		}
	}
	return time.Time{}, err
}

// looksLikeTime returns true if s matches a 12-hour or 24-hour time token.
func looksLikeTime(s string) bool {
	return time12hRe.MatchString(s) || time24hRe.MatchString(s)
}

// parseTimeOfDay parses a time-of-day token (e.g. "5pm", "17:00", "5:30pm")
// and returns (hour, minute, error). Hour is in 24-hour form (0-23).
func parseTimeOfDay(s string) (int, int, error) {
	if m := time12hRe.FindStringSubmatch(s); len(m) == 4 {
		h, _ := strconv.Atoi(m[1])
		min := 0
		if m[2] != "" {
			min, _ = strconv.Atoi(m[2])
		}
		if h < 1 || h > 12 {
			return 0, 0, fmt.Errorf("invalid 12-hour time %q: hour must be 1-12", s)
		}
		if min > 59 {
			return 0, 0, fmt.Errorf("invalid time %q: minute must be 0-59", s)
		}
		if m[3] == "pm" && h != 12 {
			h += 12
		} else if m[3] == "am" && h == 12 {
			h = 0
		}
		return h, min, nil
	}
	if m := time24hRe.FindStringSubmatch(s); len(m) == 3 {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if h > 23 {
			return 0, 0, fmt.Errorf("invalid 24-hour time %q: hour must be 0-23", s)
		}
		if min > 59 {
			return 0, 0, fmt.Errorf("invalid time %q: minute must be 0-59", s)
		}
		return h, min, nil
	}
	return 0, 0, fmt.Errorf("unrecognized time format: %q", s)
}

// splitDateTimeInput splits a lowercased input into (datePart, timePart).
// timePart is non-empty only when the last word of input is a recognized time
// token (12h or 24h). If the entire input is a time token, datePart is "".
func splitDateTimeInput(input string) (datePart, timePart string) {
	idx := strings.LastIndex(input, " ")
	if idx < 0 {
		if looksLikeTime(input) {
			return "", input
		}
		return input, ""
	}
	candidate := input[idx+1:]
	if looksLikeTime(candidate) {
		return input[:idx], candidate
	}
	return input, ""
}

// ParseRelativeDateWithTime is like ParseRelativeDateWithConfig but also accepts
// an optional time-of-day suffix ("5pm", "17:00", "5:30pm", "9am"). The suffix
// may appear standalone ("5pm") or combined with any date token ("today 5pm",
// "next mon 8am", "+2d 14:30").
//
// Returns (parsedTime, hasTime, error). When hasTime is false the time component
// is midnight. Use ParseRelativeDate / ParseRelativeDateWithConfig for callers
// that don't need time precision.
func ParseRelativeDateWithTime(input, weekEndDay string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(input)

	// Try ISO date+time format first (before lowercasing, since T separator is uppercase).
	if t, err := time.ParseInLocation(DateTimeDueFormat, trimmed, time.Local); err == nil {
		return t, true, nil
	}

	input = strings.ToLower(trimmed)

	datePart, timePart := splitDateTimeInput(input)

	if timePart != "" {
		var base time.Time
		if datePart == "" {
			base = StartOfDay(time.Now())
		} else {
			var err error
			base, err = ParseRelativeDateWithConfig(datePart, weekEndDay)
			if err != nil {
				return time.Time{}, false, fmt.Errorf("date part %q: %w", datePart, err)
			}
		}
		h, m, err := parseTimeOfDay(timePart)
		if err != nil {
			return time.Time{}, false, err
		}
		result := time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location())
		return result, true, nil
	}

	t, err := ParseRelativeDateWithConfig(input, weekEndDay)
	return t, false, err
}
