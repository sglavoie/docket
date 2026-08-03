package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// formOverlayKind tracks which inline overlay is active in TaskFormModel.
type formOverlayKind int

const (
	formOverlayNone          formOverlayKind = iota
	formOverlayPriority                      // formPriorityPicker
	formOverlayState                         // formStatePicker
	formOverlayFlags                         // formFlagsPicker
	formOverlayDue                           // formDueDatePicker
	formOverlayChecklistEdit                 // inline textinput for editing a checklist item
	formOverlaySubtaskEdit                   // inline overlay for editing a subtask draft
)

// ── Overlay rendering dispatch ────────────────────────────────────────────────

// renderFormOverlay renders the active inline overlay.
// Returns empty string if no overlay is active.
func (m *TaskFormModel) renderFormOverlay() string {
	switch m.activeOverlay {
	case formOverlayNone:
		return ""
	case formOverlayPriority:
		if m.priorityPicker != nil {
			return m.priorityPicker.render(m.width, m.draft.priority)
		}
	case formOverlayState:
		if m.statePicker != nil {
			return m.statePicker.render(m.width, m.draft.state)
		}
	case formOverlayFlags:
		if m.flagsPicker != nil {
			return m.flagsPicker.render(m.width)
		}
	case formOverlayDue:
		if m.duePicker != nil {
			return m.duePicker.render(m.width, m.draft.dueDate)
		}
	case formOverlayChecklistEdit:
		return m.renderChecklistEditOverlay()
	case formOverlaySubtaskEdit:
		return m.renderSubtaskEditOverlay()
	}
	return ""
}

// handleFormOverlayKey routes key events to the active inline overlay.
// Returns consumed=true when the key was handled by the overlay.
func (m *TaskFormModel) handleFormOverlayKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	var (
		cmd     tea.Cmd
		dismiss bool
		result  tea.Msg
	)
	switch m.activeOverlay {
	case formOverlayPriority:
		if m.priorityPicker != nil {
			dismiss, result = m.priorityPicker.handleKey(msg)
		}
	case formOverlayState:
		if m.statePicker != nil {
			dismiss, result = m.statePicker.handleKey(msg)
		}
	case formOverlayFlags:
		if m.flagsPicker != nil {
			dismiss, result = m.flagsPicker.handleKey(msg)
		}
	case formOverlayDue:
		if m.duePicker != nil {
			dismiss, result = m.duePicker.handleKey(msg)
		}
	case formOverlayChecklistEdit:
		cmd, dismiss, result = m.handleChecklistEditKey(msg)
	case formOverlaySubtaskEdit:
		cmd, dismiss, result = m.handleSubtaskEditKey(msg)
	default:
		return nil, false
	}

	if dismiss {
		// When canceling a brand-new item, remove it from the draft.
		if result == nil && m.childrenAddingNew {
			switch m.activeOverlay { //nolint:exhaustive // only checklist/subtask overlays have items to remove
			case formOverlayChecklistEdit:
				if m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.checklist) {
					m.draft.checklist = append(
						m.draft.checklist[:m.childrenEditIdx],
						m.draft.checklist[m.childrenEditIdx+1:]...,
					)
				}
			case formOverlaySubtaskEdit:
				if m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.subtasks) {
					m.draft.subtasks = append(
						m.draft.subtasks[:m.childrenEditIdx],
						m.draft.subtasks[m.childrenEditIdx+1:]...,
					)
				}
			}
		}
		m.childrenAddingNew = false
		m.closeInlineOverlay()
	}
	if result != nil {
		m.applyFormOverlayResult(result)
		if msg, ok := result.(TaskFormSubtaskEditedMsg); ok && msg.OpenContent {
			m.subtaskOpenIdx = msg.Idx
			m.subtaskOpenContent = msg.OpenContent
			return tea.Batch(cmd, m.handleSave()), true
		}
	}
	return cmd, true
}

// closeInlineOverlay resets the inline overlay state.
func (m *TaskFormModel) closeInlineOverlay() {
	m.activeOverlay = formOverlayNone
	m.priorityPicker = nil
	m.statePicker = nil
	m.flagsPicker = nil
	m.duePicker = nil
}

// applyFormOverlayResult merges an overlay result into the draft and marks dirty.
func (m *TaskFormModel) applyFormOverlayResult(msg tea.Msg) {
	switch msg := msg.(type) {
	case TaskFormPriorityPickedMsg:
		m.draft.priority = msg.Priority
		m.hasChanges = true
		m.err = nil
	case TaskFormStatePickedMsg:
		m.draft.state = msg.State
		m.hasChanges = true
		m.err = nil
	case TaskFormFlagsPickedMsg:
		m.draft.pinned = msg.Pinned
		m.hasChanges = true
		m.err = nil
	case TaskFormDueDatePickedMsg:
		m.draft.dueDate = msg.Date
		if msg.Date == nil {
			m.draft.dueInput = ""
		} else {
			m.draft.dueInput = msg.Date.Format("2006-01-02")
		}
		// Sync the textinput so it shows the updated value if still on the Due row.
		m.dueTextInput.SetValue(m.draft.dueInput)
		m.hasChanges = true
		m.err = nil

	case TaskFormChecklistItemEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.checklist) {
			if strings.TrimSpace(msg.Text) == "" {
				// Remove the item when confirmed with blank text.
				m.draft.checklist = append(
					m.draft.checklist[:msg.Idx],
					m.draft.checklist[msg.Idx+1:]...,
				)
			} else {
				m.draft.checklist[msg.Idx].Text = msg.Text
			}
			m.hasChanges = true
			m.err = nil
		}
		m.childrenAddingNew = false

	case TaskFormSubtaskEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.subtasks) {
			if strings.TrimSpace(msg.Title) == "" {
				// Remove the subtask when confirmed with blank title
				// (consistent with checklist item handling above).
				m.draft.subtasks = append(
					m.draft.subtasks[:msg.Idx],
					m.draft.subtasks[msg.Idx+1:]...,
				)
			} else {
				m.draft.subtasks[msg.Idx].title = msg.Title
			}
			m.hasChanges = true
			m.err = nil
		}
		m.childrenAddingNew = false
	}
}

// formFindTask returns the task with the given ID from a flat list, or nil.
func formFindTask(tasks []*models.Task, id string) *models.Task {
	for _, t := range tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}
