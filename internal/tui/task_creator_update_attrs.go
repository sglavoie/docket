package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleAttrLeft handles left arrow key for attribute fields.
func (m *TaskCreatorModel) handleAttrLeft(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0: // Reproducible
			if m.bugReproducibleCursor > 0 {
				m.bugReproducibleCursor--
			}
		case 1: // Severity
			if m.bugSeverityCursor > 0 {
				m.bugSeverityCursor--
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0: // Category
			if m.ideaCategoryCursor > 0 {
				m.ideaCategoryCursor--
			}
		case 1: // Effort
			if m.ideaEffortCursor > 0 {
				m.ideaEffortCursor--
			}
		case 2: // Value
			if m.ideaValueCursor > 0 {
				m.ideaValueCursor--
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0: // Effort
			if m.taskEffortCursor > 0 {
				m.taskEffortCursor--
			}
		case 1: // WorkType
			if m.taskWorkTypeCursor > 0 {
				m.taskWorkTypeCursor--
			}
		}
	case models.TaskTypeStory:
		switch fieldIdx {
		case 0: // Story Points
			if m.storyPointsCursor > 0 {
				m.storyPointsCursor--
			}
		}
	case models.TaskTypeEpic:
		switch fieldIdx {
		case 0: // Scope
			if m.epicScopeCursor > 0 {
				m.epicScopeCursor--
			}
		}
	}
}

// handleAttrRight handles right arrow key for attribute fields.
func (m *TaskCreatorModel) handleAttrRight(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0: // Reproducible
			if m.bugReproducibleCursor < len(bugReproducibleOptions)-1 {
				m.bugReproducibleCursor++
			}
		case 1: // Severity
			if m.bugSeverityCursor < len(bugSeverityOptions)-1 {
				m.bugSeverityCursor++
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0: // Category
			if m.ideaCategoryCursor < len(ideaCategoryOptions)-1 {
				m.ideaCategoryCursor++
			}
		case 1: // Effort
			if m.ideaEffortCursor < len(effortOptions)-1 {
				m.ideaEffortCursor++
			}
		case 2: // Value
			if m.ideaValueCursor < len(ideaValueOptions)-1 {
				m.ideaValueCursor++
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0: // Effort
			if m.taskEffortCursor < len(effortOptions)-1 {
				m.taskEffortCursor++
			}
		case 1: // WorkType
			if m.taskWorkTypeCursor < len(workTypeOptions)-1 {
				m.taskWorkTypeCursor++
			}
		}
	case models.TaskTypeStory:
		switch fieldIdx {
		case 0: // Story Points
			if m.storyPointsCursor < len(storyPointsOptions)-1 {
				m.storyPointsCursor++
			}
		}
	case models.TaskTypeEpic:
		switch fieldIdx {
		case 0: // Scope
			if m.epicScopeCursor < len(epicScopeOptions)-1 {
				m.epicScopeCursor++
			}
		}
	}
}

// handleAttrEnter handles enter key for text attribute fields.
func (m *TaskCreatorModel) handleAttrEnter(fieldIdx int) tea.Cmd {
	switch m.taskType {
	case models.TaskTypeBug:
		if fieldIdx == 2 { // Workaround
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "bugWorkaround",
					CurrentValue: m.bugWorkaround,
				}
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 1 { // Persona
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "storyPersona",
					CurrentValue: m.storyPersona,
				}
			}
		}
	case models.TaskTypeEpic:
		if fieldIdx == 1 { // Milestone
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "epicMilestone",
					CurrentValue: m.epicMilestone,
				}
			}
		}
		if fieldIdx == 2 { // Success Metrics
			return func() tea.Msg {
				return ShowTextEditorMsg{
					FieldName:    "epicSuccessMetrics",
					CurrentValue: m.epicSuccessMetrics,
				}
			}
		}
	default:
	}
	return nil
}

// buildTaskAttrs creates the appropriate attrs struct from form state.
func (m *TaskCreatorModel) buildTaskAttrs() (bugAttrs *models.BugAttrs, ideaAttrs *models.IdeaAttrs, taskAttrs *models.TaskAttrs, storyAttrs *models.StoryAttrs, epicAttrs *models.EpicAttrs) {
	switch m.taskType {
	case models.TaskTypeBug:
		attrs := &models.BugAttrs{}
		if m.bugReproducibleCursor > 0 {
			attrs.Reproducible = models.ValidBugReproducible[m.bugReproducibleCursor-1]
		}
		if m.bugSeverityCursor > 0 {
			attrs.Severity = models.ValidBugSeverity[m.bugSeverityCursor-1]
		}
		attrs.Workaround = m.bugWorkaround
		if !attrs.IsEmpty() {
			bugAttrs = attrs
		}
	case models.TaskTypeIdea:
		attrs := &models.IdeaAttrs{}
		if m.ideaCategoryCursor > 0 {
			attrs.Category = models.ValidIdeaCategory[m.ideaCategoryCursor-1]
		}
		if m.ideaEffortCursor > 0 {
			attrs.Effort = models.ValidEffortEstimate[m.ideaEffortCursor-1]
		}
		if m.ideaValueCursor > 0 {
			attrs.Value = models.ValidIdeaValue[m.ideaValueCursor-1]
		}
		attrs.Validated = m.ideaValidated
		if !attrs.IsEmpty() {
			ideaAttrs = attrs
		}
	case models.TaskTypeTask:
		attrs := &models.TaskAttrs{}
		if m.taskEffortCursor > 0 {
			attrs.Effort = models.ValidEffortEstimate[m.taskEffortCursor-1]
		}
		if m.taskWorkTypeCursor > 0 {
			attrs.WorkType = models.ValidWorkType[m.taskWorkTypeCursor-1]
		}
		if !attrs.IsEmpty() {
			taskAttrs = attrs
		}
	case models.TaskTypeStory:
		attrs := &models.StoryAttrs{}
		if m.storyPointsCursor > 0 {
			attrs.StoryPoints = storyPointsValues[m.storyPointsCursor]
		}
		attrs.Persona = m.storyPersona
		if !attrs.IsEmpty() {
			storyAttrs = attrs
		}
	case models.TaskTypeEpic:
		attrs := &models.EpicAttrs{}
		if m.epicScopeCursor > 0 {
			attrs.Scope = models.ValidEpicScope[m.epicScopeCursor-1]
		}
		attrs.Milestone = m.epicMilestone
		attrs.SuccessMetrics = m.epicSuccessMetrics
		if !attrs.IsEmpty() {
			epicAttrs = attrs
		}
	}
	return
}
