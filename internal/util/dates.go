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

// maxRelativeOffset bounds user-supplied numeric offsets in relative-date
// parsing (e.g. "+Nd", "in N years"). It prevents silent int64 overflow when
// the parsed number is later multiplied (years×12, weeks×7) or fed into
// time.Time arithmetic, both of which can wrap and yield wildly wrong dates
// without returning an error. One million is several orders of magnitude
// beyond any realistic task-manager input.
const maxRelativeOffset = 1_000_000

// StartOfDay returns t truncated to midnight (00:00:00) in t's location.
func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// AddMonthsClamped adds months to t, clamping the day to the target month's
// last day when t.Day() exceeds it. This avoids Go's default AddDate
// overflow (Jan 31 + 1 month → Mar 3) which is surprising for date-picker
// and snooze UX where users expect "+1m" to land in the next calendar month.
// Negative months are supported. Time-of-day, sub-second precision, and
// location are preserved.
func AddMonthsClamped(t time.Time, months int) time.Time {
	year, month, day := t.Date()
	targetMonth := int(month) + months
	targetYear := year + (targetMonth-1)/12
	targetMonth = ((targetMonth - 1) % 12) + 1
	if targetMonth <= 0 {
		targetMonth += 12
		targetYear--
	}
	lastDay := daysInMonth(targetYear, time.Month(targetMonth))
	if day > lastDay {
		day = lastDay
	}
	hour, minute, sec := t.Clock()
	return time.Date(targetYear, time.Month(targetMonth), day, hour, minute, sec, t.Nanosecond(), t.Location())
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
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

func normalizeDateInput(input string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(input))), " ")
}

func parseExactDate(input string) (time.Time, bool) {
	for _, layout := range []string{DateFormat, "01/02/2006", "02.01.2006"} {
		if date, err := time.ParseInLocation(layout, input, time.Local); err == nil {
			return date, true
		}
	}
	return time.Time{}, false
}

func daysUntilWeekday(today time.Time, targetDay time.Weekday, includeToday bool) int {
	daysUntil := int(targetDay) - int(today.Weekday())
	if includeToday {
		if daysUntil < 0 {
			daysUntil += 7
		}
		return daysUntil
	}
	if daysUntil <= 0 {
		daysUntil += 7
	}
	return daysUntil
}

func weekEndTargetDay(weekEndDay string) time.Weekday {
	switch weekEndDay {
	case "fri":
		return time.Friday
	case "sat":
		return time.Saturday
	default:
		return time.Sunday
	}
}

func parseKeywordDate(input, weekEndDay string, today time.Time) (time.Time, bool) {
	switch input {
	case "today", "eod":
		return today, true
	case "tomorrow", "tmrw", "tom":
		return today.AddDate(0, 0, 1), true
	case "yesterday", "ytd":
		return today.AddDate(0, 0, -1), true
	case "eow":
		targetDay := weekEndTargetDay(weekEndDay)
		return today.AddDate(0, 0, daysUntilWeekday(today, targetDay, true)), true
	case "eom":
		return time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()), true
	case "eoy":
		return time.Date(today.Year(), 12, 31, 0, 0, 0, 0, today.Location()), true
	}
	return time.Time{}, false
}

func parseMatchedOffset(match []string, input, unit string, apply func(int) time.Time) (time.Time, bool, error) {
	if len(match) != 2 {
		return time.Time{}, false, nil
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return time.Time{}, true, fmt.Errorf("invalid number of %s in %q", unit, input)
	}
	if value > maxRelativeOffset || value < -maxRelativeOffset {
		return time.Time{}, true, fmt.Errorf("number of %s in %q out of range", unit, input)
	}
	return apply(value), true, nil
}

func parseRelativeOffset(input string, today time.Time) (time.Time, bool, error) {
	if date, ok, err := parseMatchedOffset(relativeDaysRe.FindStringSubmatch(input), input, "days", func(n int) time.Time {
		return today.AddDate(0, 0, n)
	}); ok || err != nil {
		return date, ok, err
	}
	if date, ok, err := parseMatchedOffset(relativeWeeksRe.FindStringSubmatch(input), input, "weeks", func(n int) time.Time {
		return today.AddDate(0, 0, n*7)
	}); ok || err != nil {
		return date, ok, err
	}
	return parseMatchedOffset(relativeMonthsRe.FindStringSubmatch(input), input, "months", func(n int) time.Time {
		return AddMonthsClamped(today, n)
	})
}

func parseThisWeekday(input string, today time.Time) (time.Time, bool, error) {
	match := thisWeekdayRe.FindStringSubmatch(input)
	if len(match) != 2 {
		return time.Time{}, false, nil
	}
	targetDay := weekdays[match[1]]
	daysOffset := isoWeekdayNum(targetDay) - isoWeekdayNum(today.Weekday())
	if daysOffset < 0 {
		dayName := match[1]
		return time.Time{}, true, fmt.Errorf("'this %s' is in the past; did you mean 'next %s'?", dayName, dayName)
	}
	return today.AddDate(0, 0, daysOffset), true, nil
}

func parseNamedWeekday(input string, today time.Time) (time.Time, bool) {
	targetDay, ok := weekdays[input]
	if !ok {
		return time.Time{}, false
	}
	return today.AddDate(0, 0, daysUntilWeekday(today, targetDay, false)), true
}

func parseNextDate(input string, today time.Time) (time.Time, bool) {
	switch {
	case input == "next week":
		return today.AddDate(0, 0, 7), true
	case input == "next month":
		return AddMonthsClamped(today, 1), true
	case nextYearRe.MatchString(input):
		return AddMonthsClamped(today, 12), true
	}
	return time.Time{}, false
}

func parseNextWeekday(input string, today time.Time) (time.Time, bool) {
	match := nextWeekdayRe.FindStringSubmatch(input)
	if len(match) != 2 {
		return time.Time{}, false
	}
	targetDay := weekdays[match[1]]
	// Use includeToday=true so the helper returns 0 when today is the
	// target weekday; we then add a single week. Otherwise the helper
	// already bumps 0 → 7 and we'd double-count, landing on today+14.
	daysUntil := daysUntilWeekday(today, targetDay, true) + 7
	return today.AddDate(0, 0, daysUntil), true
}

func parseDurationOffset(input string, today time.Time) (time.Time, bool, error) {
	match := inDurationRe.FindStringSubmatch(input)
	if len(match) != 3 {
		return time.Time{}, false, nil
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return time.Time{}, true, fmt.Errorf("invalid duration in %q", input)
	}
	if n > maxRelativeOffset || n < -maxRelativeOffset {
		return time.Time{}, true, fmt.Errorf("duration in %q out of range", input)
	}
	switch match[2] {
	case "day", "days":
		return today.AddDate(0, 0, n), true, nil
	case "week", "weeks":
		return today.AddDate(0, 0, n*7), true, nil
	case "month", "months":
		return AddMonthsClamped(today, n), true, nil
	case "year", "years":
		return AddMonthsClamped(today, n*12), true, nil
	default:
		return time.Time{}, true, fmt.Errorf("invalid duration in %q", input)
	}
}

func parseNaturalMonthDay(input string, today time.Time) (time.Time, bool, error) {
	if match := monthDayRe.FindStringSubmatch(input); len(match) == 3 {
		date, err := parseMonthDay(match[1], match[2], today)
		return date, true, err
	}
	if match := dayMonthRe.FindStringSubmatch(input); len(match) == 3 {
		date, err := parseMonthDay(match[2], match[1], today)
		return date, true, err
	}
	return time.Time{}, false, nil
}

// ParseRelativeDateWithConfig parses a natural language date string like
// ParseRelativeDate, but respects the weekEndDay setting for the eow token.
// weekEndDay must be "fri", "sat", or "sun" (invalid values default to "sun").
func ParseRelativeDateWithConfig(input, weekEndDay string) (time.Time, error) {
	input = normalizeDateInput(input)
	today := StartOfDay(time.Now())

	if date, ok := parseExactDate(input); ok {
		return date, nil
	}
	if date, ok := parseKeywordDate(input, weekEndDay, today); ok {
		return date, nil
	}
	if date, ok, err := parseRelativeOffset(input, today); ok || err != nil {
		return date, err
	}
	if date, ok, err := parseThisWeekday(input, today); ok || err != nil {
		return date, err
	}
	if date, ok := parseNamedWeekday(input, today); ok {
		return date, nil
	}
	if date, ok := parseNextDate(input, today); ok {
		return date, nil
	}
	if date, ok := parseNextWeekday(input, today); ok {
		return date, nil
	}
	if date, ok, err := parseDurationOffset(input, today); ok || err != nil {
		return date, err
	}
	if date, ok, err := parseNaturalMonthDay(input, today); ok || err != nil {
		return date, err
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s", input)
}

// parseMonthDay builds a date from a 3-letter month abbreviation and a day string.
// If the resulting date is in the past, the next occurrence (next year) is returned.
// For month/day combinations that only exist in certain years (e.g. Feb 29),
// the function advances to the next valid year rather than accepting Go's
// date normalization (which would silently overflow into the next month).
// Returns an error if the day is out of range for the given month.
func parseMonthDay(monthStr, dayStr string, today time.Time) (time.Time, error) {
	month, ok := months[monthStr]
	if !ok {
		return time.Time{}, fmt.Errorf("unrecognized month: %s", monthStr)
	}
	day, _ := strconv.Atoi(dayStr)
	// Validate the day against the maximum possible day in the given month
	// across any year (29 for Feb, 30/31 otherwise). Using time.Date for
	// validation incorrectly rejects "feb 29" in non-leap base years because
	// time.Date silently normalises Feb 29 → Mar 1, hiding the otherwise-valid
	// input before the leap-year advance loop below ever runs.
	if day < 1 || day > maxDayInMonth(month) {
		return time.Time{}, fmt.Errorf("invalid date: %s %d", monthStr, day)
	}
	year := today.Year()
	candidate := time.Date(year, month, day, 0, 0, 0, 0, today.Location())
	// Advance the year until time.Date no longer normalises (i.e. the day
	// actually exists) AND the result is strictly after today.
	for candidate.Month() != month || candidate.Day() != day || !candidate.After(today) {
		year++
		candidate = time.Date(year, month, day, 0, 0, 0, 0, today.Location())
	}
	return candidate, nil
}

// maxDayInMonth returns the largest day number the given month may have in any
// year (29 for February to account for leap years).
func maxDayInMonth(m time.Month) int {
	switch m {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		return 31
	case time.April, time.June, time.September, time.November:
		return 30
	case time.February:
		return 29
	}
	return 31
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
		if n > maxRelativeOffset || n < -maxRelativeOffset {
			return time.Time{}, fmt.Errorf("number in %q out of range", input)
		}
		today := StartOfDay(time.Now())
		switch match[2] {
		case "d":
			return today.AddDate(0, 0, n), nil
		case "w":
			return today.AddDate(0, 0, n*7), nil
		case "m":
			return AddMonthsClamped(today, n), nil
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

	input = normalizeDateInput(trimmed)
	// Strip the preposition "at" between date and time tokens so natural
	// phrases like "friday at 3pm" or "tomorrow at 9am" are accepted.
	input = strings.ReplaceAll(input, " at ", " ")

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
