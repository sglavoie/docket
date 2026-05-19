package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// formatDueDate returns a styled due date string for inline display.
// timeFormat is a Go time layout string (e.g. "15:04" or "3:04pm").
func formatDueDate(task *models.Task, timeFormat string) string {
	if task.DueDate == nil || task.Status.IsComplete() {
		return ""
	}
	dateStr, style := ClassifyDueDate(task.DueDate)
	dateStr = AppendTimeIfHasTime(task, dateStr, timeFormat)
	return style.Render(icons.Get("date") + " " + dateStr)
}

// getVisibleSiblings returns siblings of the given task in the order they
// appear in the rendered tree, after filters and sort have been applied.
// Sourcing order from m.nodes (not m.project.GetSiblings) ensures J/K reorder
// swaps with the visually adjacent sibling even when sortNodes has reordered
// the display (Active/Snoozed view modes, explicit sort).
func (m *ProjectTreeModel) getVisibleSiblings(task *models.Task) []*models.Task {
	var parentID string
	if task.ParentID != nil {
		parentID = *task.ParentID
	}
	var siblings []*models.Task
	for _, node := range m.nodes {
		if node == nil || node.Task == nil {
			continue
		}
		if !m.passesFilters(node) {
			continue
		}
		var nodeParentID string
		if node.Task.ParentID != nil {
			nodeParentID = *node.Task.ParentID
		}
		if nodeParentID != parentID {
			continue
		}
		siblings = append(siblings, node.Task)
	}
	return siblings
}

// findVisibleSiblingIndex returns the index of task among its visible siblings, or -1 if not found.
func (m *ProjectTreeModel) findVisibleSiblingIndex(task *models.Task, visibleSiblings []*models.Task) int {
	for i, s := range visibleSiblings {
		if s.ID == task.ID {
			return i
		}
	}
	return -1
}

// cycleFilter cycles through filter values, returning the next value in the sequence.
// When current is the zero value, returns the first value. When current is the last value,
// returns the zero value. If current is not found, returns the zero value.
func cycleFilter[T comparable](current T, values []T) T {
	var zero T
	if current == zero {
		if len(values) > 0 {
			return values[0]
		}
		return zero
	}
	for i, v := range values {
		if v == current {
			if i+1 < len(values) {
				return values[i+1]
			}
			return zero
		}
	}
	return zero
}

// passesAttrFilters checks if a task passes all attribute filters.
// Attribute filter keys: "bug.severity", "bug.reproducible", "idea.category", "idea.value",
// "idea.validated", "effort" (task/idea), "task.worktype", "epic.scope", "story.points"
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
