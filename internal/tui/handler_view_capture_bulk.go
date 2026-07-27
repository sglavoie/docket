package tui

import tea "charm.land/bubbletea/v2"

// handleCaptureAndBulkMessages handles the Task Composer.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskComposerMsg:
		return true, m.openTaskComposer(msg.Preset)

	}

	return false, nil
}
