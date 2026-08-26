package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg processes only the inline controls owned by the Outliner. Root
// operations are dispatched by OperationRegistry before the model sees them.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// An open suggestion pane claims Tab, Esc, and the four navigation keys before
	// the search cascade below sees them, which is what makes the first Esc close
	// the pane and the next one clear or restore the search.
	if m.IsSearchMode() && m.handleSearchAutocompleteKey(key) {
		return m, nil
	}

	// Handle search mode input first
	if m.IsSearchMode() && key == "esc" {
		if m.IsRangeSelectionActive() {
			m.FreezeRangeSelection()
			m.updateVisualSelectionStatus()
			return m, nil
		}
		m.ClearSearch()
		return m, nil
	}
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		rebuildOutlinerPreservingCursor(m)
	}); handled {
		// Recompute unconditionally rather than on value change alone: a bare caret
		// move leaves the query identical while putting a different token under it.
		m.refreshSearchAutocomplete()
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
