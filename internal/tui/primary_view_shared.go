package tui

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// This file hosts symbols formerly declared in the project_tree*.go family that
// are still consumed by surviving primary views (Outliner, cross-project
// outliner, shared primary-view helpers, due-date overview, etc.) after the
// ProjectTree view was retired.

// SortField represents the field to sort tasks by.
type SortField int

const (
	SortByDefault   SortField = iota // existing order (by type hierarchy)
	SortByPriority                   // high priority first (1 = highest)
	SortByDueDate                    // earliest due date first
	SortByCreatedAt                  // by creation date
	SortByStatus                     // by workflow status order
	SortByUpdatedAt                  // by last touch time
	SortByTitle                      // case-insensitive alphabetical
)

// ShortString returns an abbreviated label for compact display (e.g. column headers).
func (s SortField) ShortString() string {
	switch s {
	case SortByPriority:
		return "pri"
	case SortByDueDate:
		return "due"
	case SortByCreatedAt:
		return "new"
	case SortByStatus:
		return "sts"
	case SortByUpdatedAt:
		return "upd"
	case SortByTitle:
		return "ttl"
	default:
		return ""
	}
}

func (s SortField) String() string {
	switch s {
	case SortByPriority:
		return "priority"
	case SortByDueDate:
		return "due date"
	case SortByCreatedAt:
		return "created"
	case SortByStatus:
		return "status"
	case SortByUpdatedAt:
		return "updated"
	case SortByTitle:
		return "title"
	default:
		return "default"
	}
}

// DueDateFilter represents the due date filter options.
type DueDateFilter int

const (
	DueDateAll      DueDateFilter = iota // show all tasks
	DueDateOverdue                       // only overdue tasks
	DueDateToday                         // tasks due today
	DueDateThisWeek                      // tasks due within 7 days
	DueDateNone                          // tasks with no due date
)

// CompletionAgeFilter represents the local-calendar age threshold selected by
// the completion-cleanup facet. CompletionAgeAll removes the age bound while
// the facet remains active, so malformed records without a valid CompletedAt
// are still excluded from cleanup.
type CompletionAgeFilter int

const (
	CompletionAgeAll CompletionAgeFilter = iota
	CompletionAge7Days
	CompletionAge30Days
	CompletionAge90Days
)

func (f CompletionAgeFilter) days() int {
	switch f {
	case CompletionAge7Days:
		return 7
	case CompletionAge30Days:
		return 30
	case CompletionAge90Days:
		return 90
	default:
		return 0
	}
}

// formatDueDate returns a styled due date string for inline display.
func formatDueDate(task *models.Task) string {
	if task.DueDate == nil || task.IsComplete() {
		return ""
	}
	dateStr, style := ClassifyDueDate(task.DueDate)
	return style.Render(icons.Get("date") + " " + dateStr)
}

// snoozeWakeWindow bounds how long a lapsed snooze keeps announcing that the
// task came back. Past it the task is ordinary work again, not news.
const snoozeWakeWindow = 72 * time.Hour

// formatRecencyChip renders an age as a single-token indicator such as 5m, 2h,
// or 3d. Prose relative time is too wide for a row, and the day bucket header
// already carries the coarse grouping; the chip only disambiguates within it.
func formatRecencyChip(key, now time.Time) string {
	age := formatRecencyAge(key, now)
	if age == "" {
		return ""
	}
	return MutedStyle.Render(age)
}

// formatRecencyAge is the unstyled age token. Peek composes several of them into
// one styled line, so it needs the token without the chip's own styling; two
// formatters would let the same age read two ways on adjacent lines.
func formatRecencyAge(key, now time.Time) string {
	if key.IsZero() {
		return ""
	}
	elapsed := now.Sub(key)
	if elapsed < time.Minute {
		return "now"
	}
	switch {
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd", int(elapsed.Hours())/24)
	}
}
