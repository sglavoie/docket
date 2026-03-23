package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleInboxMessages handles inbox processor related messages.
func (m *AppModel) handleInboxMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowInboxProcessorMsg:
		processor := NewInboxProcessorModel(msg.Project, m.config, m.configManager)
		if len(processor.inboxTasks) == 0 {
			m.setInfoStatus("No inbox items to process")
			return true, defaultStatusTimeout()
		}
		m.showView(ViewInboxProcessor, processor)
		return true, nil

	case CloseInboxProcessorMsg:
		m.popView()
		m.views.Remove(ViewInboxProcessor)
		// Reload from disk to ensure we have the latest state
		// (handles race condition if save Cmd is still in flight)
		if m.currentProject != nil {
			_ = m.persister.Load(m.currentProject)
		}
		m.refreshProjectTree()
		if msg.ProcessedCount > 0 || msg.SkippedCount > 0 {
			m.setSuccessStatus(fmt.Sprintf("Inbox processing complete: %d processed, %d skipped",
				msg.ProcessedCount, msg.SkippedCount))
		}
		return true, nil

	case ShowGlobalInboxProcessorMsg:
		processor := NewGlobalInboxProcessorModel(m.config, m.configManager)
		if len(processor.inboxTasks) == 0 {
			m.setInfoStatus("No inbox items across all projects")
			return true, defaultStatusTimeout()
		}
		m.globalInboxProcessor = processor
		m.showView(ViewGlobalInboxProcessor, processor)
		return true, nil

	case CloseGlobalInboxProcessorMsg:
		m.popToAnchorView()
		// Refresh dashboard to reflect any changes
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		if msg.ProcessedCount > 0 || msg.SkippedCount > 0 {
			m.setSuccessStatus(fmt.Sprintf("Global inbox processing complete: %d processed, %d skipped",
				msg.ProcessedCount, msg.SkippedCount))
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
