package util

import "time"

// Standard date format constants for consistent formatting across the codebase.
const (
	DateFormat     = "2006-01-02"       // Date only (YYYY-MM-DD)
	DateTimeFormat = "2006-01-02 15:04" // Date and time (YYYY-MM-DD HH:MM)
)

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
