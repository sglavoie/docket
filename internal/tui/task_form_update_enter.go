package tui

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleScheduleEnter dispatches Enter key events for focused Schedule-tab rows.
func (m *TaskFormModel) handleScheduleEnter() tea.Cmd {
	switch m.focusIndex {
	case 0:
		// Due: textinput is already focused; Enter commits and moves to Recurrence.
		m.commitDueInput()
		m.focusNext()
		m.syncInputFocus()
		return nil

	case 1:
		// Start Date: commit input and move to Est. Days.
		m.commitStartDateInput()
		m.focusNext()
		m.syncInputFocus()
		return nil

	case 2:
		// Est. Days: commit input and move to Recurrence.
		m.commitEstimatedDaysInput()
		m.focusNext()
		m.syncInputFocus()
		return nil

	case 3:
		// Recurrence: open the recurrence editor as a full-screen overlay.
		pattern := m.draft.recurrence
		dueDate := m.draft.dueDate
		return func() tea.Msg {
			return ShowRecurrenceEditorMsg{
				CurrentPattern: pattern,
				DueDate:        dueDate,
			}
		}
	}
	return nil
}

// handleBasicsEnter dispatches Enter key events for focused Basics-tab rows.
func (m *TaskFormModel) handleBasicsEnter() tea.Cmd {
	switch m.focusIndex {
	case 0:
		// Title: focus textarea (already focused by focusTitleInput, no-op here)
		return nil

	case 1:
		// Description: open TextEditorModel.
		currentDesc := m.draft.desc
		return func() tea.Msg {
			return ShowTextEditorMsg{FieldName: "formDesc", CurrentValue: currentDesc}
		}

	case 2:
		// Type: open inline type picker
		m.typePicker = newFormTypePicker(m.draft.taskType)
		m.activeOverlay = formOverlayType
		return nil

	case 3:
		// Priority: open inline priority picker
		m.priorityPicker = newFormPriorityPicker(m.draft.priority)
		m.activeOverlay = formOverlayPriority
		return nil

	case 4:
		// Parent: open full parent selector (disabled for Epics)
		if m.draft.taskType == models.TaskTypeEpic {
			return nil
		}
		parents := m.buildValidParents()
		return func() tea.Msg {
			return OpenParentSelectorMsg{
				ValidParents: parents,
				CurrentID:    m.draft.parentID,
				AllowNone:    true,
				Source:       ViewTaskForm,
			}
		}

	case 5:
		// Section: open section picker (Task=nil signals form context)
		project := m.project
		return func() tea.Msg {
			return ShowSectionPickerMsg{
				Task:    nil,
				Project: project,
			}
		}

	case 6:
		// Flags: open inline flags picker
		m.flagsPicker = newFormFlagsPicker(m.draft.pinned, m.draft.active, m.draft.dimmed)
		m.activeOverlay = formOverlayFlags
		return nil

	case 7:
		// Tags: open tag selector
		tags := append([]string(nil), m.draft.tags...)
		project := m.project
		return func() tea.Msg {
			return ShowTagSelectorMsg{
				Task:        nil,
				Project:     project,
				CurrentTags: tags,
			}
		}

	default:
		// Attr rows (focusIndex >= formBasicsFieldCount).
		if m.focusIndex >= formBasicsFieldCount {
			return m.openAttrOverlayForFocus()
		}
	}
	return nil
}

// handleDetailsEnter dispatches Enter key events for focused Details-tab rows.
func (m *TaskFormModel) handleDetailsEnter() tea.Cmd {
	switch m.focusIndex {
	case 0:
		// URL: textinput is focused; Enter commits and moves to Source.
		m.commitURLInput()
		m.focusNext()
		m.syncInputFocus()
		return nil
	case 1:
		// Source: textinput is focused; Enter commits and wraps.
		m.commitSourceInput()
		m.focusNext()
		m.syncInputFocus()
		return nil
	}
	return nil
}

// openAttrOverlayForFocus opens the appropriate overlay for the focused attr row.
// localIdx = m.focusIndex - formBasicsFieldCount.
func (m *TaskFormModel) openAttrOverlayForFocus() tea.Cmd {
	localIdx := m.focusIndex - formBasicsFieldCount

	switch m.draft.taskType {
	case models.TaskTypeBug:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Reproducible", "bugReproducible",
				attrOptsBugReproducible, string(m.draft.bugAttrs.Reproducible))
			m.activeOverlay = formOverlayAttr
		case 1:
			m.attrPicker = newFormAttrPicker("Severity", "bugSeverity",
				attrOptsBugSeverity, string(m.draft.bugAttrs.Severity))
			m.activeOverlay = formOverlayAttr
		case 2:
			current := m.draft.bugAttrs.Workaround
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "bugWorkaround", CurrentValue: current}
			}
		}

	case models.TaskTypeIdea:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Category", "ideaCategory",
				attrOptsIdeaCategory, string(m.draft.ideaAttrs.Category))
			m.activeOverlay = formOverlayAttr
		case 1:
			m.attrPicker = newFormAttrPicker("Effort", "ideaEffort",
				attrOptsEffort, string(m.draft.ideaAttrs.Effort))
			m.activeOverlay = formOverlayAttr
		case 2:
			m.attrPicker = newFormAttrPicker("Value", "ideaValue",
				attrOptsIdeaValue, string(m.draft.ideaAttrs.Value))
			m.activeOverlay = formOverlayAttr
		case 3:
			validatedCurrent := ""
			if m.draft.ideaAttrs.Validated {
				validatedCurrent = "true"
			}
			m.attrPicker = newFormAttrPicker("Validated", "ideaValidated",
				attrOptsIdeaValidated, validatedCurrent)
			m.activeOverlay = formOverlayAttr
		}

	case models.TaskTypeTask:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Effort", "taskEffort",
				attrOptsEffort, string(m.draft.taskAttrs.Effort))
			m.activeOverlay = formOverlayAttr
		case 1:
			m.attrPicker = newFormAttrPicker("Work Type", "taskWorkType",
				attrOptsWorkType, string(m.draft.taskAttrs.WorkType))
			m.activeOverlay = formOverlayAttr
		}

	case models.TaskTypeStory:
		switch localIdx {
		case 0:
			pointsCurrent := ""
			if m.draft.storyAttrs.StoryPoints > 0 {
				pointsCurrent = strconv.Itoa(m.draft.storyAttrs.StoryPoints)
			}
			m.attrPicker = newFormAttrPicker("Story Points", "storyPoints",
				attrOptsStoryPoints, pointsCurrent)
			m.activeOverlay = formOverlayAttr
		case 1:
			current := m.draft.storyAttrs.Persona
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "storyPersona", CurrentValue: current}
			}
		}

	case models.TaskTypeEpic:
		switch localIdx {
		case 0:
			m.attrPicker = newFormAttrPicker("Scope", "epicScope",
				attrOptsEpicScope, string(m.draft.epicAttrs.Scope))
			m.activeOverlay = formOverlayAttr
		case 1:
			current := m.draft.epicAttrs.Milestone
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "epicMilestone", CurrentValue: current}
			}
		case 2:
			current := m.draft.epicAttrs.SuccessMetrics
			return func() tea.Msg {
				return ShowTextEditorMsg{FieldName: "epicSuccessMetrics", CurrentValue: current}
			}
		}
	}

	return nil
}
