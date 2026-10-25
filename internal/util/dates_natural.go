package util

import (
	"fmt"
	"strconv"
	"time"
)

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
	return ParseRelativeDateWithConfigAt(input, weekEndDay, time.Now())
}

// ParseRelativeDateWithConfigAt is ParseRelativeDateWithConfig evaluated from
// now's calendar day rather than the wall clock's, so a caller holding an
// injected clock resolves "tomorrow" against the same day it judges with.
//
// The arithmetic runs on now's civil date copied into UTC, the same discipline
// ParseCheckInDateAt follows, and the start of the resulting date is rebuilt in
// now's location only once, at the end. Stepping a local day-start instead goes
// wrong in a zone whose daylight-saving change skips midnight: America/Havana
// has no 00:00 on 2026-03-08, so AddDate from 03-07 lands on 23:00 the 7th, and
// AddDate from that date's 01:00 start carries the hour onto the next day, where
// it reads as a due time nobody typed.
func ParseRelativeDateWithConfigAt(input, weekEndDay string, now time.Time) (time.Time, error) {
	input = normalizeDateInput(input)

	if date, ok := parseExactDate(input); ok {
		return date, nil
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	civil, err := parseRelativeDateFromDay(input, weekEndDay, today)
	if err != nil {
		return time.Time{}, err
	}
	return StartOfCalendarDate(civil.Year(), civil.Month(), civil.Day(), now.Location())
}

// parseRelativeDateFromDay evaluates normalized date expressions from the
// supplied day. Every caller hands it a UTC civil day, which keeps local
// midnight gaps out of the arithmetic; the caller rebuilds the result's start in
// the location it means.
func parseRelativeDateFromDay(input, weekEndDay string, today time.Time) (time.Time, error) {
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
	todayStart := StartOfDay(today)
	year := todayStart.Year()
	candidate := time.Date(year, month, day, 0, 0, 0, 0, todayStart.Location())
	// Advance the year until time.Date no longer normalises (i.e. the day
	// actually exists) and the result is not in the past.
	for candidate.Month() != month || candidate.Day() != day || candidate.Before(todayStart) {
		year++
		candidate = time.Date(year, month, day, 0, 0, 0, 0, todayStart.Location())
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
