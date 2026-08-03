package tui

import (
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
