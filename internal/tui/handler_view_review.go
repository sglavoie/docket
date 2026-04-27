package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
)

// handleViewReviewMessages handles overview, review, focus, and activity views.
func (m *AppModel) handleViewReviewMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowDueDateOverviewMsg:
		ddo := NewDueDateOverviewModel(m.config, m.configManager)
		ddo.queueIDs = m.focusQueueIDs
		ddo.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewDueDateOverview, ddo)
		return true, nil

	case ShowMetricsDashboardMsg:
		if msg.Project != nil {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.config, msg.Project)
		} else {
			m.metricsDashboard = NewMetricsDashboardModel(m.config)
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
		return true, nil

	case MetricsDashboardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowDailyReviewMsg:
		dr := NewDailyReviewModel(m.config, m.configManager)
		dr.queueIDs = m.focusQueueIDs
		dr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewDailyReview, dr)
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
		wr := NewWeeklyReviewModel(m.config, m.configManager)
		wr.queueIDs = m.focusQueueIDs
		wr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewWeeklyReview, wr)
		return true, nil

	case WeeklyReviewSelectProjectMsg:
		m.popView()
		if msg.Project != nil {
			return true, m.navigateToTaskInProject(nil, msg.Project)
		}
		return true, nil

	case WeeklyReviewSelectTaskMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case DailyReviewSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowAllTasksMsg:
		atv := NewAllTasksViewModel(m.config, m.configManager)
		atv.queueIDs = m.focusQueueIDs
		atv.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewAllTasks, atv)
		return true, nil

	case AllTasksSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowFocusBoardMsg:
		m.reevaluateAlerts()
		fb := NewFocusBoardModel(m.config, m.configManager, &m.dailyPlan, m.focusQueueIDs, m.dailyPlanIDs, m.dailyPlanCarriedIDs)
		if m.dailyPlanCarried > 0 && !m.dailyPlanCarriedShown {
			fb.setStatus(fmt.Sprintf("%d item(s) carried over from yesterday", m.dailyPlanCarried))
			m.dailyPlanCarriedShown = true
		}
		m.showView(ViewFocusBoard, fb)
		return true, nil

	case ShowPipelineHubMsg:
		hub := NewPipelineHubModel(m.config, m.configManager, m.focusQueueIDs, m.dailyPlanIDs, m.contextualProject())
		m.showView(ViewPipelineHub, hub)
		return true, nil

	case FocusBoardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case FocusBoardRefreshMsg:
		if v := m.views.Get(ViewFocusBoard); v != nil {
			v.Update(msg)
		}
		return true, nil

	case ShowFocusViewMsg:
		m.showView(ViewFocusView, NewFocusViewModel(m.config, m.configManager, &m.focusQueue, m.focusQueueIDs, m.dailyPlanIDs, m.focusDismissedIDs))
		return true, nil

	case FocusViewSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowRecentActivityMsg:
		ra := NewRecentActivityModel(m.config)
		ra.queueIDs = m.focusQueueIDs
		ra.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewRecentActivity, ra)
		return true, nil

	case ShowProjectRecentActivityMsg:
		if msg.Project != nil {
			pra := NewProjectRecentActivityModel(m.config, msg.Project)
			pra.queueIDs = m.focusQueueIDs
			pra.dailyPlanIDs = m.dailyPlanIDs
			m.showView(ViewProjectRecentActivity, pra)
		}
		return true, nil

	case ShowRecentActivityPreviewMsg:
		preview := NewRecentActivityPreviewModel(msg.Task, msg.Project)
		m.showView(ViewRecentActivityPreview, preview)
		return true, nil

	case RecentActivitySelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowProjectFocusViewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectFocusView, NewProjectFocusViewModel(msg.Project, m.config))
		}
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

	case ShowKanbanMsg:
		if msg.Project != nil {
			kanban := NewKanbanModel(msg.Project, m.config, m.configManager, NewProjectService())
			m.restoreKanbanState(kanban)
			m.showView(ViewKanban, kanban)
		}
		return true, nil

	case ShowCrossProjectKanbanMsg:
		kanban := NewCrossProjectKanbanModel(m.config, m.configManager, NewProjectService())
		m.restoreKanbanState(kanban)
		m.showView(ViewCrossProjectKanban, kanban)
		return true, nil

	case KanbanSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)
	}

	return false, nil
}
