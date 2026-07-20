package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// handleViewReviewMessages handles overview, review, focus, and activity views.
func (m *AppModel) handleViewReviewMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowMetricsDashboardMsg:
		if msg.Project != nil {
			m.metricsDashboard = NewMetricsDashboardModelForProject(m.configManager, msg.Project)
		} else {
			m.metricsDashboard = NewMetricsDashboardModel(m.configManager)
		}
		m.showView(ViewMetricsDashboard, m.metricsDashboard)
		return true, nil

	case MetricsDashboardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowDailyReviewMsg:
		dr := NewDailyReviewModel(m.runtime, m.configManager)
		dr.queueIDs = m.focusQueueIDs
		dr.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewDailyReview, dr)
		return true, nil

	case MarkDailyReviewDoneMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SetLastDailyReviewAt(time.Now())
		}); err != nil {
			logging.Warn("failed to save daily review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Review complete! Next: start working (;w)")
		return true, nil

	case MarkWeeklyReviewDoneMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SetLastWeeklyReviewAt(time.Now())
		}); err != nil {
			logging.Warn("failed to save weekly review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review completed")
		return true, nil

	case ResetDailyReviewMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.ClearLastDailyReviewAt()
		}); err != nil {
			logging.Warn("failed to reset daily review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Daily review reset")
		return true, nil

	case ResetWeeklyReviewMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.ClearLastWeeklyReviewAt()
		}); err != nil {
			logging.Warn("failed to reset weekly review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Weekly review reset")
		return true, nil

	case ShowWeeklyReviewMsg:
		wr := NewWeeklyReviewModel(m.runtime, m.configManager)
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
		var opts []AllTasksOption
		if msg.SetGroup {
			opts = append(opts, WithInitialGrouping(msg.Group))
		}
		atv := NewAllTasksViewModel(m.runtime, m.configManager, opts...)
		atv.bulk = m.secondaryFeatures.Bulk
		atv.queueIDs = m.focusQueueIDs
		atv.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewAllTasks, atv)
		return true, nil

	case AllTasksSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowFocusBoardMsg:
		m.reevaluateAlerts()
		fb := NewFocusBoardModel(m.runtime, m.configManager, &m.dailyPlan, &m.focusQueue, m.focusQueueIDs, m.dailyPlanIDs, m.dailyPlanCarriedIDs)
		if msg.SetTab {
			fb.tabs.Active = msg.Tab
			fb.initialTabSet = true
			fb.applyFilter()
			fb.clampCursor()
		}
		if m.dailyPlanCarried > 0 && !m.dailyPlanCarriedShown {
			fb.setStatus(fmt.Sprintf("%d item(s) carried over from yesterday", m.dailyPlanCarried))
			m.dailyPlanCarriedShown = true
		}
		m.showView(ViewFocusBoard, fb)
		return true, nil

	case ShowPipelineHubMsg:
		hub := NewPipelineHubModel(m.runtime, m.focusQueueIDs, m.dailyPlanIDs, m.contextualProject())
		m.showView(ViewPipelineHub, hub)
		return true, nil

	case PipelineHubCaptureMsg:
		m.popView()
		if msg.Purpose == PickerPurposeQuickCapture {
			return true, func() tea.Msg { return ShowQuickCaptureMsg{} }
		}
		return true, func() tea.Msg {
			return ShowGlobalProjectPickerMsg{Purpose: msg.Purpose}
		}

	case FocusBoardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case FocusBoardRefreshMsg:
		if v := m.views.Get(ViewFocusBoard); v != nil {
			v.Update(msg)
		}
		return true, nil

	case ShowRecentActivityMsg:
		ra := NewRecentActivityModel(m.runtime, m.configManager)
		ra.queueIDs = m.focusQueueIDs
		ra.dailyPlanIDs = m.dailyPlanIDs
		m.showView(ViewRecentActivity, ra)
		return true, nil

	case ShowProjectRecentActivityMsg:
		if msg.Project != nil {
			pra := NewProjectRecentActivityModel(m.runtime, m.configManager, msg.Project)
			pra.queueIDs = m.focusQueueIDs
			pra.dailyPlanIDs = m.dailyPlanIDs
			m.showView(ViewProjectRecentActivity, pra)
		}
		return true, nil

	case ShowRecentActivityPreviewMsg:
		preview := NewRecentActivityPreviewModel(msg.Task, msg.ProjectScope)
		m.showView(ViewRecentActivityPreview, preview)
		return true, nil

	case RecentActivitySelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowProjectFocusViewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectFocusView, NewProjectFocusViewModel(msg.Project, m.runtime))
		}
		return true, nil

	case ShowRoadmapMsg:
		if msg.Project != nil {
			m.roadmap = NewRoadmapModelForProject(m.runtime, msg.Project)
		} else {
			m.roadmap = NewRoadmapModel(m.runtime)
		}
		m.showView(ViewRoadmap, m.roadmap)
		return true, nil

	case RoadmapSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowKanbanMsg:
		if msg.Project != nil {
			kanban := NewKanbanModel(msg.Project, m.configManager, m.runtime, NewProjectService())
			m.restoreKanbanState(kanban)
			m.showView(ViewKanban, kanban)
		}
		return true, nil

	case ShowCrossProjectKanbanMsg:
		kanban := NewCrossProjectKanbanModel(m.configManager, m.runtime, NewProjectService())
		m.restoreKanbanState(kanban)
		m.showView(ViewCrossProjectKanban, kanban)
		return true, nil

	case KanbanSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)
	}

	return false, nil
}
