package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
)

// handleViewReviewMessages handles overview, review, focus, and activity views.
func (m *AppModel) handleViewReviewMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowDueDateOverviewMsg:
		m.showView(ViewDueDateOverview, NewDueDateOverviewModel(m.config, m.configManager))
		return true, nil

	case CloseDueDateOverviewMsg:
		m.popView()
		return true, nil

	case ShowMetricsDashboardMsg:
		if msg.Project != nil {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.config, msg.Project)
		} else {
			m.metricsDashboard = NewMetricsDashboardModel(m.config)
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
		return true, nil

	case CloseMetricsDashboardMsg:
		m.popToAnchorView()
		return true, nil

	case MetricsDashboardSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowDailyReviewMsg:
		m.showView(ViewDailyReview, NewDailyReviewModel(m.config, m.configManager))
		return true, nil

	case CloseDailyReviewMsg:
		m.popView()
		return true, nil

	case MarkDailyReviewDoneMsg:
		m.config.SetLastDailyReviewAt(time.Now())
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Daily review completed")
		return true, nil

	case MarkWeeklyReviewDoneMsg:
		m.config.SetLastWeeklyReviewAt(time.Now())
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review completed")
		return true, nil

	case ResetDailyReviewMsg:
		m.config.ClearLastDailyReviewAt()
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Daily review reset")
		return true, nil

	case ResetWeeklyReviewMsg:
		m.config.ClearLastWeeklyReviewAt()
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review reset")
		return true, nil

	case ShowWeeklyReviewMsg:
		m.showView(ViewWeeklyReview, NewWeeklyReviewModel(m.config, m.configManager))
		return true, nil

	case CloseWeeklyReviewMsg:
		m.popView()
		return true, nil

	case WeeklyReviewSelectProjectMsg:
		m.popView()
		if msg.Project != nil {
			return true, m.navigateToTaskInProject(nil, msg.Project)
		}
		return true, nil

	case WeeklyReviewSelectTaskMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case DailyReviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowAllTasksMsg:
		m.showView(ViewAllTasks, NewAllTasksViewModel(m.config, m.configManager))
		return true, nil

	case CloseAllTasksMsg:
		m.popToAnchorView()
		return true, nil

	case AllTasksSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowFocusBoardMsg:
		m.reevaluateAlerts()
		m.showView(ViewFocusBoard, NewFocusBoardModel(m.config, m.configManager))
		return true, nil

	case CloseFocusBoardMsg:
		m.popToAnchorView()
		return true, nil

	case FocusBoardSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case FocusBoardRefreshMsg:
		if v := m.views.Get(ViewFocusBoard); v != nil {
			v.Update(msg)
		}
		return true, nil

	case ShowFocusViewMsg:
		m.showView(ViewFocusView, NewFocusViewModel(m.config, m.configManager, &m.focusQueue, m.focusQueueIDs, m.focusDismissedIDs))
		return true, nil

	case FocusViewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowRecentActivityMsg:
		m.showView(ViewRecentActivity, NewRecentActivityModel(m.config))
		return true, nil

	case CloseRecentActivityMsg:
		m.popToAnchorView()
		return true, nil

	case ShowProjectRecentActivityMsg:
		if msg.Project != nil {
			m.showView(ViewProjectRecentActivity, NewProjectRecentActivityModel(m.config, msg.Project))
		}
		return true, nil

	case CloseProjectRecentActivityMsg:
		m.popView()
		return true, nil

	case ShowRecentActivityPreviewMsg:
		preview := NewRecentActivityPreviewModel(msg.Task, msg.Project)
		m.showView(ViewRecentActivityPreview, preview)
		return true, nil

	case CloseRecentActivityPreviewMsg:
		m.popView()
		return true, nil

	case RecentActivitySelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			return true, m.navigateToTaskInProject(msg.Task, msg.Project)
		}
		return true, nil

	case ShowProjectFocusViewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectFocusView, NewProjectFocusViewModel(msg.Project))
		}
		return true, nil

	case CloseProjectFocusViewMsg:
		m.popView()
		return true, nil

	case DueDateOverviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.currentProject = msg.Project
			return true, func() tea.Msg {
				return GlowViewMsg{Task: msg.Task}
			}
		}
		return true, nil
	}

	return false, nil
}
