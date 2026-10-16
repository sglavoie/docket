package taskform

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// handleScheduleEnter dispatches Enter key events for focused Schedule-tab rows.
func (m *TaskFormModel) handleScheduleEnter() tea.Cmd {
	switch m.focusIndex {
	case 3:
		return m.openRelationEditor(formRelationDefer)
	case 0:
		m.commitDueInput()
		m.focusNext()
		m.syncInputFocus()
		return nil
	case 1, 2:
		m.checkInEditor = NewCheckInEditorModel(
			operation.OperationExecutionRequest{},
			"",
			m.draft.checkIn,
			false,
		)
		if m.focusIndex == 2 {
			m.checkInEditor.focus = checkInFieldRepeat
			m.checkInEditor.syncFocus()
		}
		m.activeOverlay = formOverlayCheckIn
		return m.checkInEditor.InitCmd()
	}
	return nil
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
		draftTitle := strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
		return func() tea.Msg {
			return chrome.ShowTextEditorMsg{
				FieldName:    chrome.TextEditorFieldFormDescription,
				Subtitle:     draftTitle,
				CurrentValue: currentDesc,
			}
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
				Source:       presentation.ViewTaskComposer,
			}
		}

	case 4:
		m.statePicker = newFormStatePicker(m.draft.state)
		m.activeOverlay = formOverlayState
		return nil

	case 5:
		// Flags: open inline flags picker
		m.flagsPicker = newFormFlagsPicker(m.draft.pinned, m.draft.review)
		m.activeOverlay = formOverlayFlags
		return nil

	case 6:
		// Tags: open tag selector
		tags := append([]string(nil), m.draft.tags...)
		project := m.project
		return func() tea.Msg {
			return listsurface.ShowTagSelectorMsg{
				Task:        nil,
				Project:     project,
				CurrentTags: tags,
			}
		}

	}
	return nil
}
