package chrome

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
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

// DueRelativeWindowDays bounds where a row's due badge speaks relatively. Inside
// a week either way a reader thinks in days ("Yesterday", "In 3 days"); past it
// the calendar date is the shorter and the more useful spelling.
const DueRelativeWindowDays = 6

// ClassifyDueDate returns a due date's display text and badge style against now,
// or ("", theme.NormalStyle) for a nil date; it is the one place a calendar day
// becomes a tier, so every surface showing a due date inherits its answer. Its third result
// reports whether the text already says the day has passed ("Yesterday",
// "3 days ago"), so a caller adding an overdue word does not say it twice.
// Near days take DayGloss's vocabulary, sentence-cased, so a row badge, Peek's
// gloss, and capture's preview spell one relative day alike.
func ClassifyDueDate(dueDate *time.Time, now time.Time) (string, lipgloss.Style, bool) {
	if dueDate == nil {
		return "", theme.NormalStyle, false
	}

	dateText := dueDate.Format("Jan 2")
	if dueDate.Year() != now.Year() {
		dateText = dueDate.Format("Jan 2 '06")
	}

	days := util.DaysBetween(now, dueDate.In(now.Location()))
	switch {
	case days < -DueRelativeWindowDays:
		return dateText, theme.OverdueStyle, false
	case days < 0:
		return sentenceCase(DayGloss(*dueDate, now)), theme.OverdueStyle, true
	case days == 0:
		return "Today", theme.DueTodayStyle, false
	case days <= DueRelativeWindowDays:
		return sentenceCase(DayGloss(*dueDate, now)), theme.DueDateStyle, false
	default:
		return dateText, theme.DueDateStyle, false
	}
}

func sentenceCase(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
