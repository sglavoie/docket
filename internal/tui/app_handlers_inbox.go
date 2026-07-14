package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// handleInboxMessages handles inbox processor related messages.
func (m *AppModel) handleInboxMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowInboxProcessorMsg:
		processor := NewInboxProcessorModel(msg.Project, m.configManager, m.runtime)
		if len(processor.inboxTasks) == 0 {
			m.setInfoStatus("No inbox items to process")
			return true, defaultStatusTimeout()
		}
		m.showView(ViewInboxProcessor, processor)
		return true, nil

	case CloseInboxProcessorMsg:
		popCmd := m.popView()
		m.views.Remove(ViewInboxProcessor)
		// Reload from disk to ensure we have the latest state
		// (handles race condition if save Cmd is still in flight)
		if m.currentProject != nil {
			_ = m.persister.Load(m.currentProject)
		}
		m.refreshProjectTree()
		if msg.ProcessedCount > 0 {
			m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.InboxTriaged = true })
			m.setSuccessStatus(fmt.Sprintf("Inbox clear! %d triaged, %d skipped. Next: ;w Today · ;q Backlog",
				msg.ProcessedCount, msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		} else if msg.SkippedCount > 0 {
			m.setSuccessStatus(fmt.Sprintf("Inbox paused: %d skipped. Next: ;q Backlog",
				msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		}
		return true, popCmd

	case ShowGlobalInboxProcessorMsg:
		processor := NewGlobalInboxProcessorModel(m.configManager, m.runtime)
		if len(processor.inboxTasks) == 0 {
			m.setInfoStatus("No inbox items across all projects")
			return true, defaultStatusTimeout()
		}
		m.globalInboxProcessor = processor
		m.showView(ViewGlobalInboxProcessor, processor)
		return true, nil

	case CloseGlobalInboxProcessorMsg:
		popCmd := m.popToAnchorView()
		// Refresh dashboard to reflect any changes
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		if msg.ProcessedCount > 0 {
			m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.InboxTriaged = true })
			m.setSuccessStatus(fmt.Sprintf("Global inbox clear! %d triaged, %d skipped. Next: ;w Today · ;q Backlog",
				msg.ProcessedCount, msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		} else if msg.SkippedCount > 0 {
			m.setSuccessStatus(fmt.Sprintf("Global inbox paused: %d skipped. Next: ;q Backlog",
				msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		}
		return true, popCmd

	case GlobalInboxProjectSavedMsg:
		// Forward to the global inbox processor
		if m.globalInboxProcessor != nil {
			_, cmd := m.globalInboxProcessor.Update(msg)
			return true, cmd
		}
		return true, nil

	case TogglePlanTodayFromPaletteMsg:
		// Palette emits this after closing itself so the underlying inbox view
		// is at the top of the stack when toggleDailyPlan reads getTaskRef.
		if cmd := m.toggleDailyPlan(); cmd != nil {
			return true, m.batchWithFocusRefresh(cmd)
		}
		return true, nil
	}
	return false, nil
}
