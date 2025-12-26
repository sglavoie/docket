package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleInboxMessages handles inbox processor related messages.
func (m *AppModel) handleInboxMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowInboxProcessorMsg:
		processor := NewInboxProcessorModel(msg.Project, m.config, m.configManager)
		m.showView(ViewInboxProcessor, processor)
		return true, nil

	case CloseInboxProcessorMsg:
		m.popView()
		m.refreshProjectTree()
		if msg.ProcessedCount > 0 || msg.SkippedCount > 0 {
			m.statusMessage = fmt.Sprintf("Inbox processing complete: %d processed, %d skipped",
				msg.ProcessedCount, msg.SkippedCount)
		}
		return true, nil
	}
	return false, nil
}
