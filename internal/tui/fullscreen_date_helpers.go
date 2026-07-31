package tui

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// formatRelativeDueVerbose returns a verbose relative due date string
// (e.g. "in 3 weeks", "in 2 months") for use in detail panels.
func formatRelativeDueVerbose(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	daysUntil := util.DaysBetween(now, *dueDate)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return fmt.Sprintf("in %d days", daysUntil)
	case daysUntil < 14:
		return "next week"
	case daysUntil < 30:
		weeks := daysUntil / 7
		if weeks == 1 {
			return "in 1 week"
		}
		return fmt.Sprintf("in %d weeks", weeks)
	default:
		months := daysUntil / 30
		if months == 1 {
			return "in 1 month"
		}
		return fmt.Sprintf("in %d months", months)
	}
}

// FormatTimestampBracket returns "[2006-01-02 15:04] 3 hours ago".
func FormatTimestampBracket(t time.Time) string {
	return fmt.Sprintf("[%s] %s", t.Format("2006-01-02 15:04"), storage.FormatRelativeTime(t))
}

// FormatTimestampLabeled returns "Created: 2006-01-02 15:04 (3 hours ago)".
func FormatTimestampLabeled(label string, t time.Time) string {
	return fmt.Sprintf("%s: %s (%s)", label, t.Format("2006-01-02 15:04"), storage.FormatRelativeTime(t))
}

// ClassifyDueDate returns a display string and style for a due date relative to today.
// Returns ("", lipgloss.NewStyle()) for nil dates. Used by formatDueDate in
// project_tree_filters.go and due_date_overview_render.go.
func ClassifyDueDate(dueDate *time.Time) (string, lipgloss.Style) {
	if dueDate == nil {
		return "", NormalStyle
	}

	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*dueDate)

	switch {
	case dueDay.Before(today):
		return dueDate.Format("Jan 2"), OverdueStyle
	case dueDay.Equal(today):
		return "Today", DueTodayStyle
	case dueDay.Equal(today.AddDate(0, 0, 1)):
		return "Tomorrow", DueDateStyle
	default:
		if dueDate.Year() != now.Year() {
			return dueDate.Format("Jan 2 '06"), DueDateStyle
		}
		return dueDate.Format("Jan 2"), DueDateStyle
	}
}
