package chrome

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// FormatTimestampBracket returns "[2006-01-02 15:04] 3 hours ago" in local time.
// Stored instants load in UTC, so formatting t as found would show a comment's
// wall clock shifting between the session that wrote it and the next launch.
func FormatTimestampBracket(t time.Time) string {
	t = t.Local()
	return fmt.Sprintf("[%s] %s", t.Format("2006-01-02 15:04"), storage.FormatRelativeTime(t))
}

// FormatTimestampLabeled returns "Created: 2006-01-02 15:04 (3 hours ago)" in
// local time, for the same reason as FormatTimestampBracket.
func FormatTimestampLabeled(label string, t time.Time) string {
	t = t.Local()
	return fmt.Sprintf("%s: %s (%s)", label, t.Format("2006-01-02 15:04"), storage.FormatRelativeTime(t))
}

// ClassifyDueDate returns a display string and style for a due date relative to today.
// Returns ("", lipgloss.NewStyle()) for nil dates. Used by formatDueDate in
// project_tree_filters.go and due_date_overview_render.go.
func ClassifyDueDate(dueDate *time.Time) (string, lipgloss.Style) {
	if dueDate == nil {
		return "", theme.NormalStyle
	}

	now := time.Now()
	today := util.StartOfDay(now)
	dueDay := util.StartOfDay(*dueDate)
	dateText := dueDate.Format("Jan 2")
	if dueDate.Year() != now.Year() {
		dateText = dueDate.Format("Jan 2 '06")
	}

	switch {
	case dueDay.Before(today):
		return dateText, theme.OverdueStyle
	case dueDay.Equal(today):
		return "Today", theme.DueTodayStyle
	case dueDay.Equal(today.AddDate(0, 0, 1)):
		return "Tomorrow", theme.DueDateStyle
	default:
		return dateText, theme.DueDateStyle
	}
}
