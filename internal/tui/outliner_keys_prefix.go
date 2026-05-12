package tui

import (
	"github.com/sglavoie/tt/internal/models"

	tea "charm.land/bubbletea/v2"
)

// handlePrefixKey handles the second key in a prefix key sequence.
func (m *OutlinerModel) handlePrefixKey(key string) (tea.Model, tea.Cmd) {
	prefix := m.pendingPrefix
	m.pendingPrefix = "" // Always clear prefix

	if key == "esc" {
		return m, nil // Cancel prefix mode
	}

	// Handle filter sub-prefixes (Ft, Fs, Fd, Fa, Fas-Fap, FS, Fv)
	switch prefix {
	case "Ft", "Fs", "Fd", "Fa", "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap", "FS", "F.":
		if prefix == "FS" && key == "C" {
			var custom *float64
			if m.config != nil {
				custom = m.config.ScoreFilterCustom
			}
			m.scoreFilterInput.Enter(custom)
			return m, nil
		}
		if dispatchFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.config, func(p string) { m.pendingPrefix = p }) {
			m.rebuildRows()
		}
		return m, nil
	case "Fv":
		return m.handleSavedFilterPrefix(key)
	}

	// Get cursor task (may be nil if cursor is out of bounds)
	var cursorTask *models.Task
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		cursorTask = m.rows[m.cursor].task
	}

	switch prefix {
	case ",":
		return m.handleStatusPrefix(key, cursorTask)

	case ",D":
		return m.handleDeferRecurrencePrefix(key)

	case "t":
		return m.handleTaskPrefix(key, cursorTask)

	case "P":
		return m.handlePerspectivePrefix(key)

	case "s":
		return m.handleStagePrefixOutliner(key)

	case "!":
		return m.handlePriorityPrefixOutliner(key)

	case "d":
		return m.handleDueDatePrefixOutliner(key)

	case "z":
		return m.handleSnoozePrefixOutliner(key)

	case "f":
		return m.handleFoldPrefix(key, cursorTask)

	case "F":
		// Filter prefix
		return m.handleFilterPrefix(key)

	case "m":
		return m.handleSectionMovePrefixOutliner(key, cursorTask)

	case "b":
		return m.handleBlockerPrefixOutliner(key, cursorTask)

	case "c":
		return m.handleLinkedTaskPrefixOutliner(key, cursorTask)

	case "g":
		return m.handleNavigationPrefix(key, cursorTask)
	}

	// Invalid prefix sequence - silently ignore
	return m, nil
}
