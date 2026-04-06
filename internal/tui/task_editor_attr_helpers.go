package tui

import "github.com/sglavoie/tt/internal/models"

// loadExistingAttrs loads type-specific attributes from the task into form state.
func (m *TaskEditorModel) loadExistingAttrs(task *models.Task) {
	if task.BugAttrs != nil {
		copy := *task.BugAttrs
		m.originalBugAttrs = &copy
	}
	if task.IdeaAttrs != nil {
		copy := *task.IdeaAttrs
		m.originalIdeaAttrs = &copy
	}
	if task.TaskAttrs != nil {
		copy := *task.TaskAttrs
		m.originalTaskAttrs = &copy
	}
	if task.StoryAttrs != nil {
		copy := *task.StoryAttrs
		m.originalStoryAttrs = &copy
	}
	if task.EpicAttrs != nil {
		copy := *task.EpicAttrs
		m.originalEpicAttrs = &copy
	}

	if task.BugAttrs != nil {
		m.bugReproducibleCursor = findIndexInSlice(models.ValidBugReproducible, task.BugAttrs.Reproducible) + 1
		m.bugSeverityCursor = findIndexInSlice(models.ValidBugSeverity, task.BugAttrs.Severity) + 1
		m.bugWorkaround = task.BugAttrs.Workaround
	}

	if task.IdeaAttrs != nil {
		m.ideaCategoryCursor = findIndexInSlice(models.ValidIdeaCategory, task.IdeaAttrs.Category) + 1
		m.ideaEffortCursor = findIndexInSlice(models.ValidEffortEstimate, task.IdeaAttrs.Effort) + 1
		m.ideaValueCursor = findIndexInSlice(models.ValidIdeaValue, task.IdeaAttrs.Value) + 1
		m.ideaValidated = task.IdeaAttrs.Validated
	}

	if task.TaskAttrs != nil {
		m.taskEffortCursor = findIndexInSlice(models.ValidEffortEstimate, task.TaskAttrs.Effort) + 1
		m.taskWorkTypeCursor = findIndexInSlice(models.ValidWorkType, task.TaskAttrs.WorkType) + 1
	}

	if task.StoryAttrs != nil {
		m.storyPointsCursor = findStoryPointsCursor(task.StoryAttrs.StoryPoints)
		m.storyPersona = task.StoryAttrs.Persona
	}

	if task.EpicAttrs != nil {
		m.epicScopeCursor = findIndexInSlice(models.ValidEpicScope, task.EpicAttrs.Scope) + 1
		m.epicMilestone = task.EpicAttrs.Milestone
		m.epicSuccessMetrics = task.EpicAttrs.SuccessMetrics
	}
}

// findIndexInSlice returns the index of value in slice, or -1 if not found.
func findIndexInSlice[T comparable](slice []T, value T) int {
	for index, current := range slice {
		if current == value {
			return index
		}
	}
	return -1
}

// findStoryPointsCursor returns the cursor index for a story points value.
func findStoryPointsCursor(points int) int {
	for index, value := range storyPointsValues {
		if value == points {
			return index
		}
	}
	return 0 // (none)
}

// getAttrFieldCount returns the number of focusable attribute fields for the current task type.
func (m *TaskEditorModel) getAttrFieldCount() int {
	switch m.taskType {
	case models.TaskTypeBug:
		return 3 // reproducible, severity, workaround
	case models.TaskTypeIdea:
		return 4 // category, effort, value, validated
	case models.TaskTypeTask:
		return 2 // effort, workType
	case models.TaskTypeStory:
		return 2 // storyPoints, persona
	case models.TaskTypeEpic:
		return 3 // scope, milestone, successMetrics
	}
	return 0
}

// attrsStartIndex returns the focus index where attribute fields begin.
func (m *TaskEditorModel) attrsStartIndex() int {
	return 10 // After source (index 9)
}

// totalFieldCount returns total number of focusable fields.
func (m *TaskEditorModel) totalFieldCount() int {
	return m.attrsStartIndex() + m.getAttrFieldCount()
}

// hasNonEmptyAttrs returns true if the current task type has any non-empty attribute values.
func (m *TaskEditorModel) hasNonEmptyAttrs() bool {
	switch m.taskType {
	case models.TaskTypeBug:
		return m.bugReproducibleCursor > 0 || m.bugSeverityCursor > 0 || m.bugWorkaround != ""
	case models.TaskTypeIdea:
		return m.ideaCategoryCursor > 0 || m.ideaEffortCursor > 0 || m.ideaValueCursor > 0 || m.ideaValidated
	case models.TaskTypeTask:
		return m.taskEffortCursor > 0 || m.taskWorkTypeCursor > 0
	case models.TaskTypeStory:
		return m.storyPointsCursor > 0 || m.storyPersona != ""
	case models.TaskTypeEpic:
		return m.epicScopeCursor > 0 || m.epicMilestone != "" || m.epicSuccessMetrics != ""
	}
	return false
}

// clearCurrentAttrs resets all attribute values for the current task type.
func (m *TaskEditorModel) clearCurrentAttrs() {
	switch m.taskType {
	case models.TaskTypeBug:
		m.bugReproducibleCursor = 0
		m.bugSeverityCursor = 0
		m.bugWorkaround = ""
	case models.TaskTypeIdea:
		m.ideaCategoryCursor = 0
		m.ideaEffortCursor = 0
		m.ideaValueCursor = 0
		m.ideaValidated = false
	case models.TaskTypeTask:
		m.taskEffortCursor = 0
		m.taskWorkTypeCursor = 0
	case models.TaskTypeStory:
		m.storyPointsCursor = 0
		m.storyPersona = ""
	case models.TaskTypeEpic:
		m.epicScopeCursor = 0
		m.epicMilestone = ""
		m.epicSuccessMetrics = ""
	}
}

// handleAttrLeft handles left arrow key for attribute fields.
func (m *TaskEditorModel) handleAttrLeft(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0:
			if m.bugReproducibleCursor > 0 {
				m.bugReproducibleCursor--
			}
		case 1:
			if m.bugSeverityCursor > 0 {
				m.bugSeverityCursor--
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0:
			if m.ideaCategoryCursor > 0 {
				m.ideaCategoryCursor--
			}
		case 1:
			if m.ideaEffortCursor > 0 {
				m.ideaEffortCursor--
			}
		case 2:
			if m.ideaValueCursor > 0 {
				m.ideaValueCursor--
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0:
			if m.taskEffortCursor > 0 {
				m.taskEffortCursor--
			}
		case 1:
			if m.taskWorkTypeCursor > 0 {
				m.taskWorkTypeCursor--
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 0 && m.storyPointsCursor > 0 {
			m.storyPointsCursor--
		}
	case models.TaskTypeEpic:
		if fieldIdx == 0 && m.epicScopeCursor > 0 {
			m.epicScopeCursor--
		}
	}
}

// handleAttrRight handles right arrow key for attribute fields.
func (m *TaskEditorModel) handleAttrRight(fieldIdx int) {
	switch m.taskType {
	case models.TaskTypeBug:
		switch fieldIdx {
		case 0:
			if m.bugReproducibleCursor < len(bugReproducibleOptions)-1 {
				m.bugReproducibleCursor++
			}
		case 1:
			if m.bugSeverityCursor < len(bugSeverityOptions)-1 {
				m.bugSeverityCursor++
			}
		}
	case models.TaskTypeIdea:
		switch fieldIdx {
		case 0:
			if m.ideaCategoryCursor < len(ideaCategoryOptions)-1 {
				m.ideaCategoryCursor++
			}
		case 1:
			if m.ideaEffortCursor < len(effortOptions)-1 {
				m.ideaEffortCursor++
			}
		case 2:
			if m.ideaValueCursor < len(ideaValueOptions)-1 {
				m.ideaValueCursor++
			}
		}
	case models.TaskTypeTask:
		switch fieldIdx {
		case 0:
			if m.taskEffortCursor < len(effortOptions)-1 {
				m.taskEffortCursor++
			}
		case 1:
			if m.taskWorkTypeCursor < len(workTypeOptions)-1 {
				m.taskWorkTypeCursor++
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 0 && m.storyPointsCursor < len(storyPointsOptions)-1 {
			m.storyPointsCursor++
		}
	case models.TaskTypeEpic:
		if fieldIdx == 0 && m.epicScopeCursor < len(epicScopeOptions)-1 {
			m.epicScopeCursor++
		}
	}
}

// handleAttrEnter handles enter key for text attribute fields.
func (m *TaskEditorModel) handleAttrEnter(fieldIdx int) func() any {
	switch m.taskType {
	case models.TaskTypeBug:
		if fieldIdx == 2 {
			return func() any {
				return ShowTextEditorMsg{
					FieldName:    "bugWorkaround",
					CurrentValue: m.bugWorkaround,
				}
			}
		}
	case models.TaskTypeStory:
		if fieldIdx == 1 {
			return func() any {
				return ShowTextEditorMsg{
					FieldName:    "storyPersona",
					CurrentValue: m.storyPersona,
				}
			}
		}
	case models.TaskTypeEpic:
		if fieldIdx == 1 {
			return func() any {
				return ShowTextEditorMsg{
					FieldName:    "epicMilestone",
					CurrentValue: m.epicMilestone,
				}
			}
		}
		if fieldIdx == 2 {
			return func() any {
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
func (m *TaskEditorModel) buildTaskAttrs() (bugAttrs *models.BugAttrs, ideaAttrs *models.IdeaAttrs, taskAttrs *models.TaskAttrs, storyAttrs *models.StoryAttrs, epicAttrs *models.EpicAttrs) {
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
