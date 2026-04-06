package tui

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// checkChanges detects if any field has been modified.
func (m *TaskEditorModel) checkChanges() bool {
	// Normalize title by replacing newlines with spaces (same as saveTask)
	currentTitle := strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	if currentTitle != m.originalTitle {
		return true
	}
	if strings.Join(m.tags, ", ") != m.originalTags {
		return true
	}
	if m.taskType != m.originalType {
		return true
	}
	if m.priority != m.originalPriority {
		return true
	}
	if parentIDsChanged(m.parentID, m.originalParentID) {
		return true
	}
	if m.section != m.originalSection {
		return true
	}
	if dueDatesChanged(m.dueDate, m.originalDueDate) {
		return true
	}
	if m.pinned != m.originalPinned {
		return true
	}
	if m.active != m.originalActive {
		return true
	}
	if m.dimmed != m.originalDimmed {
		return true
	}
	if !recurrenceEqual(m.recurrence, m.originalRecurrence) {
		return true
	}
	if m.urlInput != m.originalURL {
		return true
	}
	if m.sourceInput != m.originalSource {
		return true
	}
	if m.attrsChanged() {
		return true
	}
	return false
}

func parentIDsChanged(current, original *string) bool {
	if current == nil && original == nil {
		return false
	}
	if current == nil || original == nil {
		return true
	}
	return *current != *original
}

func dueDatesChanged(current, original *time.Time) bool {
	if current == nil && original == nil {
		return false
	}
	if current == nil || original == nil {
		return true
	}
	return !current.Equal(*original)
}

// attrsChanged returns true if any type-specific attribute has been modified.
func (m *TaskEditorModel) attrsChanged() bool {
	switch m.taskType {
	case models.TaskTypeBug:
		current := &models.BugAttrs{}
		if m.bugReproducibleCursor > 0 {
			current.Reproducible = models.ValidBugReproducible[m.bugReproducibleCursor-1]
		}
		if m.bugSeverityCursor > 0 {
			current.Severity = models.ValidBugSeverity[m.bugSeverityCursor-1]
		}
		current.Workaround = m.bugWorkaround
		return !bugAttrsEqual(current, m.originalBugAttrs)
	case models.TaskTypeIdea:
		current := &models.IdeaAttrs{}
		if m.ideaCategoryCursor > 0 {
			current.Category = models.ValidIdeaCategory[m.ideaCategoryCursor-1]
		}
		if m.ideaEffortCursor > 0 {
			current.Effort = models.ValidEffortEstimate[m.ideaEffortCursor-1]
		}
		if m.ideaValueCursor > 0 {
			current.Value = models.ValidIdeaValue[m.ideaValueCursor-1]
		}
		current.Validated = m.ideaValidated
		return !ideaAttrsEqual(current, m.originalIdeaAttrs)
	case models.TaskTypeTask:
		current := &models.TaskAttrs{}
		if m.taskEffortCursor > 0 {
			current.Effort = models.ValidEffortEstimate[m.taskEffortCursor-1]
		}
		if m.taskWorkTypeCursor > 0 {
			current.WorkType = models.ValidWorkType[m.taskWorkTypeCursor-1]
		}
		return !taskAttrsEqual(current, m.originalTaskAttrs)
	case models.TaskTypeStory:
		current := &models.StoryAttrs{}
		if m.storyPointsCursor > 0 {
			current.StoryPoints = storyPointsValues[m.storyPointsCursor]
		}
		current.Persona = m.storyPersona
		return !storyAttrsEqual(current, m.originalStoryAttrs)
	case models.TaskTypeEpic:
		current := &models.EpicAttrs{}
		if m.epicScopeCursor > 0 {
			current.Scope = models.ValidEpicScope[m.epicScopeCursor-1]
		}
		current.Milestone = m.epicMilestone
		current.SuccessMetrics = m.epicSuccessMetrics
		return !epicAttrsEqual(current, m.originalEpicAttrs)
	}
	return false
}

func bugAttrsEqual(a, b *models.BugAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Reproducible == b.Reproducible && a.Severity == b.Severity && a.Workaround == b.Workaround
}

func ideaAttrsEqual(a, b *models.IdeaAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Category == b.Category && a.Effort == b.Effort && a.Value == b.Value && a.Validated == b.Validated
}

func taskAttrsEqual(a, b *models.TaskAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Effort == b.Effort && a.WorkType == b.WorkType
}

func storyAttrsEqual(a, b *models.StoryAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.StoryPoints == b.StoryPoints && a.Persona == b.Persona
}

func epicAttrsEqual(a, b *models.EpicAttrs) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return b.IsEmpty()
		}
		return a.IsEmpty()
	}
	return a.Scope == b.Scope && a.Milestone == b.Milestone && a.SuccessMetrics == b.SuccessMetrics
}

// recurrenceEqual compares two recurrence patterns for equality.
func recurrenceEqual(a, b *models.RecurrencePattern) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Interval != b.Interval || a.Unit != b.Unit {
		return false
	}
	if len(a.Weekdays) != len(b.Weekdays) {
		return false
	}
	for i, weekday := range a.Weekdays {
		if weekday != b.Weekdays[i] {
			return false
		}
	}
	return true
}
