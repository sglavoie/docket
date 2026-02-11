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
	DateFormat     = "2006-01-02"       // Date only (YYYY-MM-DD)
	DateTimeFormat = "2006-01-02 15:04" // Date and time (YYYY-MM-DD HH:MM)
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

// FormatDate formats a time as date only (YYYY-MM-DD).
func FormatDate(t time.Time) string {
	return t.Format(DateFormat)
}

// FormatDateTime formats a time as date and time (YYYY-MM-DD HH:MM).
func FormatDateTime(t time.Time) string {
	return t.Format(DateTimeFormat)
}

// ParseDate attempts to parse a date string in standard formats.
// It tries datetime format first, then date-only format.
// Returns the parsed time and true if successful, zero time and false otherwise.
func ParseDate(value string) (time.Time, bool) {
	if t, err := time.ParseInLocation(DateTimeFormat, value, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation(DateFormat, value, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
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

// ParseRelativeDate parses a natural language date string into a time.Time.
// Supported formats:
//   - YYYY-MM-DD (e.g., 2024-01-15)
//   - MM/DD/YYYY (e.g., 01/15/2024)
//   - DD.MM.YYYY (e.g., 15.01.2024)
//   - today, tomorrow, yesterday
//   - +Nd (N days from now), +Nw (N weeks), +Nm (N months)
//   - mon, tue, wed, thu, fri, sat, sun (next occurrence)
//   - next <weekday> (skips this week)
//   - next week, next month
//   - in N days/weeks/months
func ParseRelativeDate(input string) (time.Time, error) {
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
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}

	// Relative: +Nd (days)
	if match := regexp.MustCompile(`^\+(\d+)d$`).FindStringSubmatch(input); len(match) == 2 {
		days, _ := strconv.Atoi(match[1])
		return today.AddDate(0, 0, days), nil
	}

	// Relative: +Nw (weeks)
	if match := regexp.MustCompile(`^\+(\d+)w$`).FindStringSubmatch(input); len(match) == 2 {
		weeks, _ := strconv.Atoi(match[1])
		return today.AddDate(0, 0, weeks*7), nil
	}

	// Relative: +Nm (months)
	if match := regexp.MustCompile(`^\+(\d+)m$`).FindStringSubmatch(input); len(match) == 2 {
		months, _ := strconv.Atoi(match[1])
		return today.AddDate(0, months, 0), nil
	}

	// Weekday names (e.g., "mon", "friday") - next occurrence within 1-7 days
	if targetDay, ok := weekdays[input]; ok {
		daysUntil := int(targetDay) - int(today.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		return today.AddDate(0, 0, daysUntil), nil
	}

	// "next week" / "next month"
	if input == "next week" {
		return today.AddDate(0, 0, 7), nil
	}
	if input == "next month" {
		return today.AddDate(0, 1, 0), nil
	}

	// "next <weekday>" - always at least 7 days away (skips this week)
	if match := regexp.MustCompile(`^next\s+(sun|sunday|mon|monday|tue|tuesday|wed|wednesday|thu|thursday|fri|friday|sat|saturday)$`).FindStringSubmatch(input); len(match) == 2 {
		targetDay := weekdays[match[1]]
		daysUntil := int(targetDay) - int(today.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		daysUntil += 7 // "next" means skip this week
		return today.AddDate(0, 0, daysUntil), nil
	}

	// "in N days/weeks/months"
	if match := regexp.MustCompile(`^in\s+(\d+)\s+(day|days|week|weeks|month|months)$`).FindStringSubmatch(input); len(match) == 3 {
		n, _ := strconv.Atoi(match[1])
		switch match[2] {
		case "day", "days":
			return today.AddDate(0, 0, n), nil
		case "week", "weeks":
			return today.AddDate(0, 0, n*7), nil
		case "month", "months":
			return today.AddDate(0, n, 0), nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s", input)
}
