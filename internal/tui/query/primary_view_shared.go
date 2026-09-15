package query

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
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

// HierarchyFilter represents the hierarchy-position filter options. The parent
// and leaf axes read the scope's open-direct-child census rather than a task's
// own Children, which the All-mode promotion pass rewrites.
type HierarchyFilter int

const (
	HierarchyAll    HierarchyFilter = iota // show all tasks
	HierarchyParent                        // tasks with at least one open direct child
	HierarchyChild                         // tasks that sit under a parent
	HierarchyRoot                          // tasks with no parent
	HierarchyLeaf                          // tasks with no open direct child
)

// DependencyFilter represents the soft-reference-position filter options. Both
// axes are structural and neither is a readiness verdict: a soft reference gates
// nothing, so a blocked task is perfectly workable. Blocking reads the loaded
// population's reference index, because dependents are computed rather than
// stored on the task.
type DependencyFilter int

const (
	DependencyFilterAll      DependencyFilter = iota // show all tasks
	DependencyFilterBlocked                          // tasks naming at least one other task
	DependencyFilterBlocking                         // tasks at least one other task names
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

func (f CompletionAgeFilter) Days() int {
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

// FormatDueDate returns a styled due date string for inline display.
func FormatDueDate(task *models.Task) string {
	if task.DueDate == nil || task.IsComplete() {
		return ""
	}
	dateStr, style := chrome.ClassifyDueDate(task.DueDate)
	if clock := chrome.FormatDueClock(task); clock != "" {
		dateStr += " " + clock
		if task.IsOverdue() {
			style = theme.OverdueStyle
		}
	}
	return style.Render(icons.Get("date") + " " + dateStr)
}

// DeferWakeWindow bounds how long a lapsed deferral keeps announcing that the
// task came back. Past it the task is ordinary work again, not news.
const DeferWakeWindow = 72 * time.Hour

// FormatRecencyAge is the one age token in the product. Peek composes several of
// them into one styled line and supplies that line's own styling, so the token is
// minted unstyled; the task row carries no age at all now that an elapsed
// duration is read beside the instants it is measured from rather than alone.
func FormatRecencyAge(key, now time.Time) string {
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
