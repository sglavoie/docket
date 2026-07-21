package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
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
	SortByScore                      // by composite scoring profile score (highest first)
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
	case SortByScore:
		return "scr"
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
	case SortByScore:
		return "score"
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

// ViewMode represents the view mode filter for primary task views.
type ViewMode int

const (
	ViewModeActive    ViewMode = iota // Tasks not in Inbox, Next, Waiting, or Someday sections
	ViewModeNext                      // Only Next section tasks (GTD "Next Actions")
	ViewModeInbox                     // Only Inbox section tasks
	ViewModeWaiting                   // Only Waiting section tasks (GTD "Waiting For")
	ViewModeSomeday                   // Only Someday section tasks (GTD "Someday/Maybe")
	ViewModeCompleted                 // Only completed tasks (done or canceled)
	ViewModeReview                    // Only Review section tasks
	ViewModeStandby                   // Only Standby section tasks
	ViewModeSnoozed                   // Only snoozed tasks
	ViewModeAll                       // All non-completed, non-snoozed tasks from all sections
)

// captureTaskStates creates a snapshot of task states for event trigger detection.
func captureTaskStates(tasks []*models.Task) map[string]*models.Task {
	return workflow.CaptureTaskStates(tasks)
}

// projectArchiveRequestCmd emits a project archive/unarchive request message.
func projectArchiveRequestCmd(project *models.Project, archive bool) tea.Cmd {
	if project == nil {
		return nil
	}
	return func() tea.Msg {
		return ProjectArchiveRequestMsg{Project: project, Archive: archive}
	}
}

// taskIDsFromSlice returns the IDs of the given tasks in order.
func taskIDsFromSlice(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

// formatDueDate returns a styled due date string for inline display.
// timeFormat is a Go time layout string (e.g. "15:04" or "3:04pm").
func formatDueDate(task *models.Task, timeFormat string) string {
	if task.DueDate == nil || task.IsComplete() {
		return ""
	}
	dateStr, style := ClassifyDueDate(task.DueDate)
	dateStr = AppendTimeIfHasTime(task, dateStr, timeFormat)
	return style.Render(icons.Get("date") + " " + dateStr)
}

// typeChangeOpForKey maps a movement key to a type-change operation.
func typeChangeOpForKey(key string) (TypeChangeOp, string, bool) {
	switch key {
	case "<":
		return TypePromote, "Promote Task", true
	case ">":
		return TypeDemote, "Demote Task", true
	case "H":
		return TypeCycleLeft, "Change Type", true
	case "L":
		return TypeCycleRight, "Change Type", true
	}
	return 0, "", false
}

// sectionLimitWarning formats the over-section-limit warning banner.
func sectionLimitWarning(over, limit int) string {
	word := "tasks"
	if over == 1 {
		word = "task"
	}
	return fmt.Sprintf("%s %d %s over section limit (%d)", icons.Get("warning"), over, word, limit)
}

// passesAttrFilters reports whether a task satisfies the given type-attribute filters.
func passesAttrFilters(task *models.Task, attrs map[string]string) bool { //nolint:gocyclo
	if len(attrs) == 0 {
		return true
	}

	for key, value := range attrs {
		switch key {
		case "bug.severity":
			if task.Type != models.TaskTypeBug || task.BugAttrs == nil {
				return false
			}
			if string(task.BugAttrs.Severity) != value {
				return false
			}
		case "bug.reproducible":
			if task.Type != models.TaskTypeBug || task.BugAttrs == nil {
				return false
			}
			if string(task.BugAttrs.Reproducible) != value {
				return false
			}
		case "idea.category":
			if task.Type != models.TaskTypeIdea || task.IdeaAttrs == nil {
				return false
			}
			if string(task.IdeaAttrs.Category) != value {
				return false
			}
		case "idea.value":
			if task.Type != models.TaskTypeIdea || task.IdeaAttrs == nil {
				return false
			}
			if string(task.IdeaAttrs.Value) != value {
				return false
			}
		case "idea.validated":
			if task.Type != models.TaskTypeIdea || task.IdeaAttrs == nil {
				return false
			}
			if !task.IdeaAttrs.Validated {
				return false
			}
		case "effort":
			// Effort can apply to Task or Idea types
			switch task.Type {
			case models.TaskTypeTask:
				if task.TaskAttrs == nil || string(task.TaskAttrs.Effort) != value {
					return false
				}
			case models.TaskTypeIdea:
				if task.IdeaAttrs == nil || string(task.IdeaAttrs.Effort) != value {
					return false
				}
			default:
				return false
			}
		case "task.worktype":
			if task.Type != models.TaskTypeTask || task.TaskAttrs == nil {
				return false
			}
			if string(task.TaskAttrs.WorkType) != value {
				return false
			}
		case "epic.scope":
			if task.Type != models.TaskTypeEpic || task.EpicAttrs == nil {
				return false
			}
			if string(task.EpicAttrs.Scope) != value {
				return false
			}
		case "story.points":
			if task.Type != models.TaskTypeStory || task.StoryAttrs == nil {
				return false
			}
			// Parse expected points value
			var expectedPoints int
			var _, errScan = fmt.Sscanf(value, "%d", &expectedPoints)
			if errScan != nil {
				return false
			}
			if task.StoryAttrs.StoryPoints != expectedPoints {
				return false
			}
		}
	}
	return true
}
