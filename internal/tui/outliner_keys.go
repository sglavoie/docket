package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleKeyMsg processes only the inline controls owned by the Outliner. Root
// operations are dispatched by OperationRegistry before the model sees them.
func (m *OutlinerModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// The expanded query readout owns input outright while it is open: its peel
	// digits are a gated keymap the footer is advertising, and letting a root key
	// through underneath would run an act the on-screen footer does not name.
	if m.queryReadoutExpanded {
		return m.handleQueryReadoutKey(key)
	}

	// A lens query edit owns the input outright: it carries no suggestion pane and
	// no projection, so it must be read before the ordinary search cascade below
	// mistakes it for one.
	if m.editingLensQuery() {
		return m.handleLensQueryKey(msg)
	}

	// An open suggestion pane claims Tab, Esc, and the four navigation keys before
	// the search cascade below sees them, which is what makes the first Esc close
	// the pane and the next one clear or restore the search.
	if m.IsSearchMode() && m.handleSearchAutocompleteKey(key) {
		return m, nil
	}

	// Up and Down walk the session's submitted queries, below the pane's claim
	// above so an open pane keeps them for its own cursor. The walk is confined to
	// the input: no root key is bound for it, and the shell routes every key here
	// while search mode owns the input.
	if m.recallSearchHistory(key) {
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
	// Submitting is what retains a query, so an abandoned one never joins the
	// ring. This runs before HandleSearchMode consumes the key, which is the only
	// place the submitted text is still readable as input rather than as a query.
	if m.IsSearchMode() && key == "enter" {
		m.recordSearchHistoryEntry(m.SearchInputValue())
	}
	if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		// Typing ends the walk, so the next Up starts from the newest query again
		// rather than from wherever the abandoned walk had reached.
		m.searchHistory.resetRecall()
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
