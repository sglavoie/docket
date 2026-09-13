package taskform

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
)

// applyTextEditorResult merges a TextEditorModel result into the draft.
func (m *TaskFormModel) applyTextEditorResult(field, value string) {
	switch field {
	case "formDesc":
		m.draft.desc = value
	}
	m.hasChanges = true
	m.err = nil
}

// buildValidParents returns tasks in the project that can parent the new task.
func (m *TaskFormModel) buildValidParents() []*models.Task {
	return append([]*models.Task(nil), m.project.Tasks...)
}

// handlePaste forwards a tea.PasteMsg to the currently focused text input.
// An active Extras item overlay owns the paste: tea.PasteMsg never reaches
// handleFormOverlayKey, which only sees tea.KeyPressMsg, so the overlay inputs
// are routed here instead. The overlay stays open, exactly as a typed rune
// leaves it open.
func (m *TaskFormModel) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	markDirty := func() {
		m.hasChanges = true
		m.err = nil
	}
	switch {
	case m.activeOverlay == formOverlayCheckIn && m.checkInEditor != nil:
		return m, m.checkInEditor.HandlePaste(msg)
	case m.activeOverlay == formOverlayAttachmentEdit:
		return m, m.pasteIntoInput(msg, &m.attachmentItemInput, markDirty)
	case m.activeOverlay == formOverlayCommentEdit:
		return m, m.pasteIntoInput(msg, &m.commentItemInput, markDirty)
	case m.activeTab == formTabBasics && m.focusIndex == 0:
		var cmd tea.Cmd
		m.titleInput, cmd = m.titleInput.Update(msg)
		markDirty()
		return m, cmd
	case m.activeTab == formTabBasics && !m.collapsedBasics && m.focusIndex == formBasicsContextField:
		return m, m.pasteIntoInput(msg, &m.contextInput, markDirty)
	case m.activeTab == formTabSchedule && m.focusIndex == 0:
		var cmd tea.Cmd
		m.dueTextInput, cmd = m.dueTextInput.Update(msg)
		markDirty()
		return m, cmd
	}
	return m, nil
}

// pasteIntoInput forwards a paste to one textinput and marks the form dirty
// only when the pasted content actually changed the value, mirroring the
// before/after comparison handleTextInputKey uses for typed keys.
func (m *TaskFormModel) pasteIntoInput(
	msg tea.PasteMsg,
	input *textinput.Model,
	markDirty func(),
) tea.Cmd {
	before := input.Value()
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	if input.Value() != before {
		markDirty()
	}
	return cmd
}

// handleTextInputKey consolidates the common key handling for textinput fields
// (context, due date). It calls commit before navigation/save/esc, and forwards
// unrecognized keys to the textinput. Returns (cmd, true) if the key was consumed.
func (m *TaskFormModel) handleTextInputKey(msg tea.KeyPressMsg, commit func(), input *textinput.Model) tea.Cmd {
	switch msg.String() {
	case "tab":
		commit()
		m.focusNext()
		m.syncInputFocus()
		return nil
	case "shift+tab":
		commit()
		m.focusPrev()
		m.syncInputFocus()
		return nil
	case "shift+left":
		commit()
		m.prevTab()
		m.syncInputFocus()
		m.err = nil
		return nil
	case "shift+right":
		commit()
		m.nextTab()
		m.syncInputFocus()
		m.err = nil
		return nil
	case "enter":
		commit()
		m.focusNext()
		m.syncInputFocus()
		return nil
	case "ctrl+s":
		commit()
		return m.handleSave()
	case "esc":
		commit()
		if m.hasChanges {
			m.ConfirmDiscard = true
		} else {
			return func() tea.Msg { return status.BackMsg{} }
		}
		return nil
	default:
		before := input.Value()
		var cmd tea.Cmd
		*input, cmd = input.Update(msg)
		after := input.Value()
		if before != after {
			m.hasChanges = true
			m.err = nil
		}
		return cmd
	}
}
