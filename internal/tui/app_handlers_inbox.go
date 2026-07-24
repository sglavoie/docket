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
		if msg.Project == nil {
			return true, m.activateManageProjectPicker(ManageInbox)
		}
		return true, m.activateManageHub(ManageParams{
			Module: ManageInbox,
			Scope:  ProjectEntityScope(msg.Project.ID),
		})

	case CloseInboxProcessorMsg:
		popCmd := m.popView()
		m.views.Remove(ViewInboxProcessor)
		// Reload from disk to ensure we have the latest state
		// (handles race condition if save Cmd is still in flight)
		if m.currentProject != nil {
			_ = m.persister.Load(m.currentProject)
		}
		m.refreshProjectView()
		if msg.ProcessedCount > 0 {
			m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.InboxTriaged = true })
			m.setSuccessStatus(fmt.Sprintf("Inbox clear! %d triaged, %d skipped. Next: plan Today or Next",
				msg.ProcessedCount, msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		} else if msg.SkippedCount > 0 {
			m.setSuccessStatus(fmt.Sprintf("Inbox paused: %d skipped. Next: shortlist Next",
				msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		}
		return true, popCmd

	case ShowGlobalInboxProcessorMsg:
		return true, m.activateManageHub(ManageParams{Module: ManageInbox, Scope: AppEntityScope()})

	case CloseGlobalInboxProcessorMsg:
		popCmd := m.popToAnchorView()
		// Refresh dashboard to reflect any changes
		if msg.ProcessedCount > 0 {
			m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.InboxTriaged = true })
			m.setSuccessStatus(fmt.Sprintf("Global inbox clear! %d triaged, %d skipped. Next: plan Today or Next",
				msg.ProcessedCount, msg.SkippedCount))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		} else if msg.SkippedCount > 0 {
			m.setSuccessStatus(fmt.Sprintf("Global inbox paused: %d skipped. Next: shortlist Next",
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
			return true, cmd
		}
		return true, nil
	}
	return false, nil
}
