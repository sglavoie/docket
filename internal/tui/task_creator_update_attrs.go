package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleAttrNav handles left/right arrow keys for attribute fields.
// direction is -1 for left, +1 for right.
func (m *TaskCreatorModel) handleAttrNav(fieldIdx, direction int) {
	clamp := func(cursor *int, maxLen int) {
		*cursor += direction
		if *cursor < 0 {
			*cursor = 0
		} else if *cursor >= maxLen {
			*cursor = maxLen - 1
		}
	}

	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0:
			clamp(&m.bugReproducibleCursor, len(bugReproducibleOptions))
		case 1:
			clamp(&m.bugSeverityCursor, len(bugSeverityOptions))
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0:
			clamp(&m.ideaCategoryCursor, len(ideaCategoryOptions))
		case 1:
			clamp(&m.ideaEffortCursor, len(effortOptions))
		case 2:
			clamp(&m.ideaValueCursor, len(ideaValueOptions))
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0:
			clamp(&m.taskEffortCursor, len(effortOptions))
		case 1:
			clamp(&m.taskWorkTypeCursor, len(workTypeOptions))
		}
	case models.TaskTypeStory:
		if fieldIdx == 0 {
			clamp(&m.storyPointsCursor, len(storyPointsOptions))
		}
	case models.TaskTypeEpic:
		if fieldIdx == 0 {
			clamp(&m.epicScopeCursor, len(epicScopeOptions))
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
