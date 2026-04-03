package tui

import tea "charm.land/bubbletea/v2"

// handleViewMessages handles view navigation and display messages.
// Returns true if the message was handled.
func (m *AppModel) handleViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate search messages to dedicated handler
	if handled, cmd := m.handleSearchMessages(msg); handled {
		return handled, cmd
	}

	// Delegate saved filter messages to dedicated handler
	if handled, cmd := m.handleSavedFilterMessages(msg); handled {
		return handled, cmd
	}

	// Delegate notes messages to dedicated handler
	if handled, cmd := m.handleNotesMessages(msg); handled {
		return handled, cmd
	}

	// Delegate context messages to dedicated handler
	if handled, cmd := m.handleContextMessages(msg); handled {
		return handled, cmd
	}

	// Delegate scoring messages to dedicated handler
	if handled, cmd := m.handleScoringMessages(msg); handled {
		return handled, cmd
	}

	// Delegate editor messages to dedicated handler
	if handled, cmd := m.handleEditorMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewCoreMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewSettingsMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewSelectionMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewReviewMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewActionMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleViewSupportMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleProjectViewMessages(msg); handled {
		return handled, cmd
	}

	if handled, cmd := m.handleNotificationMessages(msg); handled {
		return handled, cmd
	}
	return false, nil
}
