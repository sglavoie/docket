package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// handleViewReviewMessages handles overview, review, focus, and activity views.
func (m *AppModel) handleViewReviewMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowReviewHubMsg:
		return true, m.activateReviewHub(msg.Module, msg.Project)

	case ShowMetricsDashboardMsg:
		return true, m.activateReviewHub(ReviewMetrics, msg.Project)

	case MetricsDashboardSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	case ShowDailyReviewMsg:
		// Daily Review is an Outliner computed mode: select it in the global
		// Outliner rather than launching a standalone review screen.
		return true, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeDailyReview}}
		}

	case MarkDailyReviewDoneMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.SetLastDailyReviewAt(time.Now())
		}); err != nil {
			logging.Warn("failed to save daily review completion: %v", err)
		}
		m.reevaluateAlerts()
		m.setSuccessStatus("Review complete! Next: open the Today mode")
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
		return true, m.activateReviewHub(ReviewWeekly, nil)

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

	case ShowRecentActivityMsg:
		// Activity is an Outliner computed mode over all projects.
		return true, func() tea.Msg {
			return ShowGlobalOutlinerMsg{Options: OutlinerOpenOptions{SetMode: true, Mode: ModeActivity}}
		}

	case ShowProjectRecentActivityMsg:
		if msg.Project == nil {
			return true, nil
		}
		return true, m.openScopedOutlinerActivity(msg.Project)

	case ShowRoadmapMsg:
		return true, m.activateReviewHub(ReviewRoadmap, msg.Project)

	case RoadmapSelectMsg:
		return m.popAndNavigateToTask(msg.Task, msg.Project)

	}

	return false, nil
}

// openScopedOutlinerActivity opens the global Outliner scoped to a single
// project and selects the Activity computed mode, replacing the retired
// standalone project recent-activity screen.
func (m *AppModel) openScopedOutlinerActivity(project *models.Project) tea.Cmd {
	if m.outliner != nil {
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
	}
	presentationCmd := m.prepareCrossProjectOutliner()
	m.crossOutliner.pushProjectScope(project.ID)
	m.crossOutliner.ApplyOpenOptions(OutlinerOpenOptions{SetMode: true, Mode: ModeActivity})
	if m.typedNavigation {
		route, err := NewRoute(TaskWorkspace, TaskWorkspaceParams{Scope: AppEntityScope()})
		if err != nil {
			return func() tea.Msg { return ErrorMsg{Err: err} }
		}
		return tea.Batch(m.activateTaskWorkspaceRoute(route), presentationCmd)
	}
	m.showView(ViewCrossProjectOutliner, m.crossOutliner)
	return presentationCmd
}
