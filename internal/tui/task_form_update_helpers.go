package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
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

// buildValidParents returns tasks in the project that can parent the current task.
// In edit mode, the task being edited and all its descendants are excluded to
// prevent circular parent-child relationships.
func (m *TaskFormModel) buildValidParents() []*models.Task {
	// In edit mode, build a set of the task's own ID + all descendant IDs.
	excluded := make(map[string]bool)
	if m.mode == FormModeEdit && m.task != nil {
		var collectDescendants func(t *models.Task)
		collectDescendants = func(t *models.Task) {
			excluded[t.ID] = true
			for _, child := range t.Children {
				collectDescendants(child)
			}
		}
		collectDescendants(m.task)
	}

	var parents []*models.Task
	for _, t := range m.project.Tasks {
		if excluded[t.ID] {
			continue
		}
		parents = append(parents, t)
	}
	return parents
}

// handlePaste forwards a tea.PasteMsg to the currently focused text input.
func (m *TaskFormModel) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	markDirty := func() {
		m.hasChanges = true
		m.err = nil
	}
	switch {
	case m.activeTab == formTabBasics && m.focusIndex == 0:
		var cmd tea.Cmd
		m.titleInput, cmd = m.titleInput.Update(msg)
		markDirty()
		return m, cmd
	case m.activeTab == formTabSchedule && m.focusIndex == 0:
		var cmd tea.Cmd
		m.dueTextInput, cmd = m.dueTextInput.Update(msg)
		markDirty()
		return m, cmd
	}
	return m, nil
}

// handleTextInputKey consolidates the common key handling for textinput fields
// (due date). It calls commit before navigation/save/esc, and forwards
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
			m.confirmDiscard = true
		} else {
			return func() tea.Msg { return BackMsg{} }
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
