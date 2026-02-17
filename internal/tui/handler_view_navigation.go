package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

// handleNavigateToView dispatches a NavigateToViewMsg to the appropriate Show message.
func (m *AppModel) handleNavigateToView(msg NavigateToViewMsg) (bool, tea.Cmd) {
	project := m.config.GetCurrentProject()

	switch msg.ViewType {
	case ViewKanban:
		if project != nil {
			_ = storage.LoadProject(project)
			return m.handleViewMessages(ShowKanbanMsg{Project: project})
		}
	case ViewDailyReview:
		return m.handleViewMessages(ShowDailyReviewMsg{})
	case ViewWeeklyReview:
		return m.handleViewMessages(ShowWeeklyReviewMsg{})
	case ViewFocusBoard:
		return m.handleViewMessages(ShowFocusBoardMsg{})
	case ViewFocusView:
		return m.handleViewMessages(ShowFocusViewMsg{})
	case ViewAllTasks:
		return m.handleViewMessages(ShowAllTasksMsg{})
	case ViewDueDateOverview:
		return m.handleViewMessages(ShowDueDateOverviewMsg{})
	case ViewMetricsDashboard:
		return m.handleViewMessages(ShowMetricsDashboardMsg{})
	case ViewPriorityView:
		if project != nil {
			_ = storage.LoadProject(project)
			return m.handleViewMessages(ShowPriorityViewMsg{Project: project})
		}
	case ViewBlockers:
		if project != nil {
			_ = storage.LoadProject(project)
			return m.handleViewMessages(ShowBlockersMsg{Project: project})
		}
	case ViewGlobalInboxProcessor:
		return m.handleViewMessages(ShowGlobalInboxProcessorMsg{})
	case ViewGlobalTagManager:
		return m.handleViewMessages(ShowGlobalTagManagerMsg{})
	case ViewGlobalNotes:
		return m.handleViewMessages(ShowGlobalNotesMsg{})
	case ViewGlobalBlockers:
		return m.handleViewMessages(ShowGlobalBlockersMsg{})
	case ViewNotes:
		if project != nil {
			return m.handleViewMessages(ShowNotesMsg{Project: project})
		}
	case ViewSettings:
		return m.handleViewMessages(SettingsMsg{})
	case ViewUndoHistory:
		if project != nil {
			_ = storage.LoadProject(project)
			return m.handleViewMessages(ShowUndoHistoryMsg{Project: project})
		}
	case ViewOutliner:
		if project != nil {
			_ = storage.LoadProject(project)
			return m.handleViewMessages(ShowOutlinerMsg{Project: project})
		}
	}
	return true, nil
}
