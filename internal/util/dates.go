package util

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Standard date format constants for consistent formatting across the codebase.
const (
	DateFormat = "2006-01-02" // Date only (YYYY-MM-DD)
)

// Common time durations for date arithmetic and comparisons.
const (
	Day = 24 * time.Hour
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
// and defer UX where users expect "+1m" to land in the next calendar month.
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
	relativeHoursRe  = regexp.MustCompile(`^\+(\d+)h$`)
	nextWeekdayRe    = regexp.MustCompile(`^next\s+(sun|sunday|mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday)$`)
	thisWeekdayRe    = regexp.MustCompile(`^this\s+(sun|sunday|mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday)$`)
	inDurationRe     = regexp.MustCompile(`^in\s+(\d+)\s+(day|days|week|weeks|month|months|year|years)$`)
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

// ParseDeferInstantAt parses a deferral target relative to now. It accepts
// everything ParseRelativeDate accepts plus the hour form "+Nh", which is the
// only form that can resolve later the same day; every other form still lands
// on local midnight of a calendar day.
func ParseDeferInstantAt(input string, now time.Time) (time.Time, error) {
	input = normalizeDateInput(input)
	if instant, ok, err := parseMatchedOffset(relativeHoursRe.FindStringSubmatch(input), input, "hours", func(n int) time.Time {
		return now.Add(time.Duration(n) * time.Hour)
	}); ok || err != nil {
		return instant, err
	}
	return ParseRelativeDate(input)
}

// ParseDeferInstant parses a deferral target relative to the current time.
func ParseDeferInstant(input string) (time.Time, error) {
	return ParseDeferInstantAt(input, time.Now())
}

// ParseHourWindowAt parses only the "+Nh" hour-offset form and reports whether
// the input matched that shape. It is the filter-only counterpart to the
// defer-only hour offset: search uses it to build a "due within N hours" window
// while leaving every other value (buckets, calendar days) to fall through to
// day-granular parsing. The offset bound is the same one ParseDeferInstantAt
// applies, so "+Nh" means the same span on both surfaces.
func ParseHourWindowAt(input string, now time.Time) (time.Time, bool, error) {
	input = normalizeDateInput(input)
	if !relativeHoursRe.MatchString(input) {
		return time.Time{}, false, nil
	}
	instant, _, err := parseMatchedOffset(relativeHoursRe.FindStringSubmatch(input), input, "hours", func(n int) time.Time {
		return now.Add(time.Duration(n) * time.Hour)
	})
	return instant, true, err
}

// ParseCalendarDate parses a day-granular date. It is ParseRelativeDate with
// the hour form refused by name, so a surface that stores a calendar day says
// why "+4h" is rejected instead of reporting an unrecognized format.
func ParseCalendarDate(input string) (time.Time, error) {
	input = normalizeDateInput(input)
	if relativeHoursRe.MatchString(input) {
		return time.Time{}, errHoursAreDeferOnly
	}
	return ParseRelativeDate(input)
}

// clockTimeRe matches an explicit hh:mm clock time (00:00–23:59 is enforced by
// ParseDueDate, not the pattern, so the caller can name an out-of-range value).
var clockTimeRe = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)

// ParseDueDate parses a due-date value that may carry an optional trailing
// clock time (hh:mm), for example "tomorrow 15:30" or "2026-08-25 09:00". It
// returns the resolved local time and whether an explicit time was present. The
// date portion is parsed with ParseCalendarDate, so the hour-offset form (+Nh)
// is still refused: hours are defer-only. A value without a trailing hh:mm
// resolves to local midnight with hasTime false. An explicit 00:00 collapses to
// a date-only due date because a midnight clock is the date-only signal.
func ParseDueDate(input string) (time.Time, bool, error) {
	datePart, hour, minute, hasTime, err := ExtractDueClockSuffix(input)
	if err != nil {
		return time.Time{}, false, err
	}
	day, err := ParseCalendarDate(datePart)
	if err != nil {
		return time.Time{}, false, err
	}
	if !hasTime {
		return day, false, nil
	}
	due := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
	return due, true, nil
}

// ExtractDueClockSuffix splits an optional trailing hh:mm clock time off a due
// value, returning the remaining date text plus the parsed hour and minute. The
// hasTime result is false for a missing suffix and for an explicit 00:00, since
// a midnight clock is the date-only signal. An out-of-range hh:mm is an error so
// the caller can name it.
func ExtractDueClockSuffix(input string) (string, int, int, bool, error) {
	trimmed := strings.TrimSpace(input)
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return trimmed, 0, 0, false, nil
	}
	match := clockTimeRe.FindStringSubmatch(fields[len(fields)-1])
	if match == nil {
		return trimmed, 0, 0, false, nil
	}
	hour, _ := strconv.Atoi(match[1])
	minute, _ := strconv.Atoi(match[2])
	if hour > 23 || minute > 59 {
		return "", 0, 0, false, fmt.Errorf("invalid due time %q: hh:mm must be 00:00-23:59", fields[len(fields)-1])
	}
	return strings.Join(fields[:len(fields)-1], " "), hour, minute, hour != 0 || minute != 0, nil
}

var errHoursAreDeferOnly = errors.New("hours are defer-only; a due date is a calendar day — try +1d")

func normalizeDateInput(input string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(input))), " ")
}

func parseExactDate(input string) (time.Time, bool) {
	return parseExactDateIn(input, time.Local)
}

// parseExactDateIn is parseExactDate with the location named, so a caller that
// wants the civil date a spelling denotes rather than an instant in the local
// zone can read the components out of a UTC parse. The layout list is shared
// deliberately: two readers of the same spellings that disagreed about which
// ones they accept would be two date grammars.
func parseExactDateIn(input string, loc *time.Location) (time.Time, bool) {
	for _, layout := range []string{DateFormat, "01/02/2006", "02.01.2006"} {
		if date, err := time.ParseInLocation(layout, input, loc); err == nil {
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

// NextWeekdayAfter is the start of the first day strictly after now's calendar
// day that falls on weekday, so it never answers today: on a Monday the next
// Monday is a week away, and on a Saturday the coming Saturday is too.
func NextWeekdayAfter(now time.Time, weekday time.Weekday) time.Time {
	return StartOfDay(now).AddDate(0, 0, daysUntilWeekday(now, weekday, false))
}
