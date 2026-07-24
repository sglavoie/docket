package tui

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// FormatRelativeDue formats a due date relative to today.
// When styled is true, overdue/today/normal ranges are wrapped in color styles.
func FormatRelativeDue(dueDate *time.Time, styled bool) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	daysUntil := util.DaysBetween(now, *dueDate)

	var text string
	var style lipgloss.Style

	switch {
	case daysUntil < -1:
		text = fmt.Sprintf("%d days ago", -daysUntil)
		style = OverdueStyle
	case daysUntil == -1:
		text = "yesterday"
		style = OverdueStyle
	case daysUntil == 0:
		text = "today"
		style = DueTodayStyle
	case daysUntil == 1:
		text = "tomorrow"
		style = DueDateStyle
	case daysUntil < 7:
		text = dueDate.Format("Mon")
		style = DueDateStyle
	default:
		if dueDate.Year() != now.Year() {
			text = dueDate.Format("Jan 2 '06")
		} else {
			text = dueDate.Format("Jan 2")
		}
		style = DueDateStyle
	}

	if styled {
		return style.Render(text)
	}
	return text
}

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

// AppendTimeIfHasTime appends a formatted time suffix to s when task.HasTime is true.
// goFmt is a Go time layout string, e.g. "15:04" (24h) or "3:04pm" (12h).
func AppendTimeIfHasTime(task *models.Task, s string, goFmt string) string {
	if task.HasTime && task.DueDate != nil {
		return s + " " + task.DueDate.Format(goFmt)
	}
	return s
}

// FormatRelativeDueTask is like FormatRelativeDue but also appends a time suffix
// when task.HasTime is true, formatted according to goFmt.
func FormatRelativeDueTask(task *models.Task, styled bool, goFmt string) string {
	s := FormatRelativeDue(task.DueDate, styled)
	if task.HasTime && task.DueDate != nil {
		s += " " + task.DueDate.Format(goFmt)
	}
	return s
}
