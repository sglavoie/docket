package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg processes only the inline controls owned by the Outliner. Root
// operations are dispatched by OperationRegistry before the model sees them.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// The inline mode picker owns all input while open.
	if m.modePickerActive() {
		return m.handleModePickerKey(msg)
	}

	// Handle score filter custom value input
	if m.scoreFilterInput.IsActive() {
		cmd := m.scoreFilterInput.ProcessKey(msg, &m.filter.ScoreMin, func() { m.rebuildRows() })
		return m, cmd
	}

	// Handle search mode input first
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.rebuildRows()
		DetectSearchAutocomplete(m.SearchAC(), m.SearchInputValue(), m.searchTagCache())
	}); handled {
		return model, cmd
	}

	// Handle quick edit mode input first
	if m.quickEdit.IsActive() {
		return m.handleQuickEditMode(msg)
	}

	// Handle confirm dialog before anything else
	if m.confirm.IsActive() {
		return m.handleConfirmDialog(key)
	}

	return m, nil
}
