package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

// handleCaptureAndBulkMessages handles the Task Composer.
func (m *AppModel) handleCaptureAndBulkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.ShowTaskComposerMsg:
		return true, m.openTaskComposer(msg.Preset)

	}

	return false, nil
}
