package tui

import "time"

// dueDateIsToday compares calendar fields rather than instants: task due dates
// are date-only values and may retain a location different from the TUI clock.
func dueDateIsToday(due *time.Time) bool {
	if due == nil {
		return false
	}
	now := time.Now()
	dueYear, dueMonth, dueDay := due.Date()
	nowYear, nowMonth, nowDay := now.Date()
	return dueYear == nowYear && dueMonth == nowMonth && dueDay == nowDay
}
