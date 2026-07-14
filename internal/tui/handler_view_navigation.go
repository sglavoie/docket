package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// handleNavigateToView dispatches a NavigateToViewMsg to the appropriate Show message.
func (m *AppModel) handleNavigateToView(msg NavigateToViewMsg) (bool, tea.Cmd) {
	project := m.currentProject

	switch msg.ViewType {
	case ViewDailyReview:
		return m.handleViewMessages(ShowDailyReviewMsg{})
	case ViewWeeklyReview:
		return m.handleViewMessages(ShowWeeklyReviewMsg{})
	case ViewFocusBoard:
		return m.handleViewMessages(ShowFocusBoardMsg{})
	case ViewCrossProjectOutliner:
		return m.handleViewMessages(ShowGlobalOutlinerMsg{})
	case ViewFocusView:
		return m.handleViewMessages(ShowFocusViewMsg{})
	case ViewAllTasks:
		return m.handleViewMessages(ShowAllTasksMsg{})
	case ViewDueDateOverview:
		return m.handleViewMessages(ShowDueDateOverviewMsg{})
	case ViewMetricsDashboard:
		return m.handleViewMessages(ShowMetricsDashboardMsg{})
	case ViewRoadmap:
		return m.handleViewMessages(ShowRoadmapMsg{})
	case ViewPriorityView:
		if project != nil {
			if err := storage.LoadProject(project); err != nil {
				logging.Warn("failed to load project %s: %v", project.Name, err)
			}
			return m.handleViewMessages(ShowPriorityViewMsg{Project: project})
		}
	case ViewBlockers:
		if project != nil {
			if err := storage.LoadProject(project); err != nil {
				logging.Warn("failed to load project %s: %v", project.Name, err)
			}
			return m.handleViewMessages(ShowBlockersMsg{Project: project})
		}
	case ViewPipelineHub:
		return m.handleViewMessages(ShowPipelineHubMsg{})
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
	case ViewOutliner:
		if project != nil {
			if err := storage.LoadProject(project); err != nil {
				logging.Warn("failed to load project %s: %v", project.Name, err)
			}
			return m.handleViewMessages(ShowOutlinerMsg{Project: project})
		}
	default:
	}
	return true, nil
}
