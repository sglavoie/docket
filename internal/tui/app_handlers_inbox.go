package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleInboxMessages handles inbox processor related messages.
func (m *AppModel) handleInboxMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowInboxProcessorMsg:
		processor := NewInboxProcessorModel(msg.Project, m.config, m.configManager)
		if len(processor.inboxTasks) == 0 {
			m.statusMessage = "No inbox items to process"
			return true, statusTimeout(2 * time.Second)
		}
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

	case ShowGlobalInboxProcessorMsg:
		processor := NewGlobalInboxProcessorModel(m.config, m.configManager)
		if len(processor.inboxTasks) == 0 {
			m.statusMessage = "No inbox items across all projects"
			return true, statusTimeout(2 * time.Second)
		}
		m.globalInboxProcessor = processor
		m.showView(ViewGlobalInboxProcessor, processor)
		return true, nil

	case CloseGlobalInboxProcessorMsg:
		m.popView()
		// Refresh dashboard to reflect any changes
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		if msg.ProcessedCount > 0 || msg.SkippedCount > 0 {
			m.statusMessage = fmt.Sprintf("Global inbox processing complete: %d processed, %d skipped",
				msg.ProcessedCount, msg.SkippedCount)
		}
		return true, nil

	case GlobalInboxProjectSavedMsg:
		// Forward to the global inbox processor
		if m.globalInboxProcessor != nil {
			_, cmd := m.globalInboxProcessor.Update(msg)
			return true, cmd
		}
		return true, nil
	}
	return false, nil
}
