package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg handles keyboard input in normal mode.
func (m *FocusBoardModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" && len(m.selection) > 0 {
		m.clearMultiSelect()
		return m, nil
	}
	if model, cmd, handled := m.Dispatcher.Dispatch(msg); handled {
		return model, cmd
	}
	if msg.String() == "esc" && RunEscCascade(m.escSteps()) {
		return m, nil
	}
	return m, nil
}

// GetHelpItems returns help items for this view.
func (m *FocusBoardModel) GetHelpItems() []HelpItem {
	return mergeHelpItems(
		GetKeybindingRegistry().GetGlobalBindings(),
		helpItemsFromKeyMap(m.buildKeyMap()),
	)
}

func (m *FocusBoardModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
