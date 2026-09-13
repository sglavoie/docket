package taskform

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// formOverlayKind tracks which inline overlay is active in TaskFormModel.
type formOverlayKind int

const (
	formOverlayNone           formOverlayKind = iota
	formOverlayPriority                       // formPriorityPicker
	formOverlayState                          // formStatePicker
	formOverlayFlags                          // formFlagsPicker
	formOverlayDue                            // formDueDatePicker
	formOverlayCheckIn                        // shared CheckInEditorModel
	formOverlayChecklistEdit                  // inline textinput for editing a checklist item
	formOverlaySubtaskEdit                    // inline overlay for editing a subtask draft
	formOverlayAttachmentEdit                 // inline textinput for editing an attachment draft
	formOverlayCommentEdit                    // inline textinput for editing a comment draft
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
	case formOverlayCheckIn:
		if m.checkInEditor != nil {
			return m.checkInEditor.Render(m.width)
		}
	case formOverlayChecklistEdit:
		return m.renderChecklistEditOverlay()
	case formOverlaySubtaskEdit:
		return m.renderSubtaskEditOverlay()
	case formOverlayAttachmentEdit:
		return m.renderAttachmentEditOverlay()
	case formOverlayCommentEdit:
		return m.renderCommentEditOverlay()
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
	case formOverlayCheckIn:
		if m.checkInEditor != nil {
			cmd, dismiss, result = m.checkInEditor.HandleKey(msg)
		}
	case formOverlayChecklistEdit:
		cmd, dismiss, result = m.handleChecklistEditKey(msg)
	case formOverlaySubtaskEdit:
		cmd, dismiss, result = m.handleSubtaskEditKey(msg)
	case formOverlayAttachmentEdit:
		cmd, dismiss, result = m.handleAttachmentEditKey(msg)
	case formOverlayCommentEdit:
		cmd, dismiss, result = m.handleCommentEditKey(msg)
	default:
		return nil, false
	}

	if dismiss {
		// When canceling a brand-new item, remove it from the draft.
		if result == nil {
			m.discardCanceledNewItem()
		}
		m.childrenAddingNew = false
		m.extrasAddingNew = false
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

// discardCanceledNewItem removes the blank draft row an item overlay appended
// when that overlay is dismissed without a result, so an abandoned add leaves
// nothing behind. Only the four item overlays append a row; the pickers edit a
// scalar and have nothing to undo.
func (m *TaskFormModel) discardCanceledNewItem() {
	switch m.activeOverlay { //nolint:exhaustive // only item overlays have a draft row to remove
	case formOverlayChecklistEdit:
		if m.childrenAddingNew && m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.checklist) {
			m.draft.checklist = append(
				m.draft.checklist[:m.childrenEditIdx],
				m.draft.checklist[m.childrenEditIdx+1:]...,
			)
		}
	case formOverlaySubtaskEdit:
		if m.childrenAddingNew && m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.subtasks) {
			m.draft.subtasks = append(
				m.draft.subtasks[:m.childrenEditIdx],
				m.draft.subtasks[m.childrenEditIdx+1:]...,
			)
		}
	case formOverlayAttachmentEdit:
		if m.extrasAddingNew && m.extrasEditIdx >= 0 && m.extrasEditIdx < len(m.draft.attachments) {
			m.draft.attachments = append(
				m.draft.attachments[:m.extrasEditIdx],
				m.draft.attachments[m.extrasEditIdx+1:]...,
			)
		}
	case formOverlayCommentEdit:
		if m.extrasAddingNew && m.extrasEditIdx >= 0 && m.extrasEditIdx < len(m.draft.comments) {
			m.draft.comments = append(
				m.draft.comments[:m.extrasEditIdx],
				m.draft.comments[m.extrasEditIdx+1:]...,
			)
		}
	}
}

// closeInlineOverlay resets the inline overlay state.
func (m *TaskFormModel) closeInlineOverlay() {
	m.activeOverlay = formOverlayNone
	m.priorityPicker = nil
	m.statePicker = nil
	m.flagsPicker = nil
	m.duePicker = nil
	m.checkInEditor = nil
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
		m.draft.review = msg.Review
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
	case operation.OperationValueSelectedMsg:
		value, ok := msg.Value.(operation.OperationCheckInValue)
		if !ok {
			return
		}
		if value.Clear {
			m.draft.checkIn = models.CheckInSchedule{}
		} else {
			m.draft.checkIn = value.Schedule.Clone()
		}
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

	case TaskFormAttachmentEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.attachments) {
			if strings.TrimSpace(msg.Value) == "" {
				// Drop the draft when confirmed with a blank value
				// (consistent with checklist and subtask handling above).
				m.draft.attachments = append(
					m.draft.attachments[:msg.Idx],
					m.draft.attachments[msg.Idx+1:]...,
				)
			} else {
				m.draft.attachments[msg.Idx].value = msg.Value
			}
			m.hasChanges = true
			m.err = nil
			m.clampExtrasFocusIndex()
		}
		m.extrasAddingNew = false

	case TaskFormCommentEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.comments) {
			if strings.TrimSpace(msg.Text) == "" {
				// Drop the draft when confirmed with blank text
				// (consistent with every other item overlay above).
				m.draft.comments = append(
					m.draft.comments[:msg.Idx],
					m.draft.comments[msg.Idx+1:]...,
				)
			} else {
				m.draft.comments[msg.Idx].text = msg.Text
			}
			m.hasChanges = true
			m.err = nil
			m.clampExtrasFocusIndex()
		}
		m.extrasAddingNew = false
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
