package tui

import tea "charm.land/bubbletea/v2"

// handleScheduleEnter dispatches Enter key events for focused Schedule-tab rows.
func (m *TaskFormModel) handleScheduleEnter() {
	if m.focusIndex == 0 {
		// Due is the sole retained schedule field.
		m.commitDueInput()
		m.focusNext()
		m.syncInputFocus()
	}
}

// handleBasicsEnter dispatches Enter key events for focused Basics-tab rows.
func (m *TaskFormModel) handleBasicsEnter() tea.Cmd {
	if m.collapsedBasics {
		switch m.focusIndex {
		case 0:
			return nil
		case 1:
			m.priorityPicker = newFormPriorityPicker(m.draft.priority)
			m.activeOverlay = formOverlayPriority
			return nil
		case 2:
			m.collapsedBasics = false
			m.focusIndex = 0
			m.syncInputFocus()
			return nil
		}
		return nil
	}

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
		// Priority: open inline priority picker
		m.priorityPicker = newFormPriorityPicker(m.draft.priority)
		m.activeOverlay = formOverlayPriority
		return nil

	case 3:
		// Parent: open full parent selector.
		parents := m.buildValidParents()
		return func() tea.Msg {
			return OpenParentSelectorMsg{
				ValidParents: parents,
				CurrentID:    m.draft.parentID,
				AllowNone:    true,
				Source:       ViewTaskForm,
			}
		}

	case 4:
		m.statePicker = newFormStatePicker(m.draft.state)
		m.activeOverlay = formOverlayState
		return nil

	case 5:
		// Flags: open inline flags picker
		m.flagsPicker = newFormFlagsPicker(m.draft.pinned)
		m.activeOverlay = formOverlayFlags
		return nil

	case 6:
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

	}
	return nil
}
