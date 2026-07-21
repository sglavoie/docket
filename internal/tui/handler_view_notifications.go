package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// handleNotificationMessages handles alert center and notification center messages.
func (m *AppModel) handleNotificationMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowNotificationCenterMsg:
		m.reevaluateAlerts()
		return true, m.activateManageHub(ManageParams{
			Module: ManageNotifications, Scope: AppEntityScope(), NotificationMode: NotificationAlerts,
		})

	case AlertCenterNavigateMsg:
		m.popView()
		switch msg.Category {
		case alerts.CategoryDailyReview:
			return true, m.activateReviewHub(ReviewDaily, nil)
		case alerts.CategoryWeeklyReview:
			return true, m.activateReviewHub(ReviewWeekly, nil)
		default:
		}

		if msg.ProjectID == "" {
			return true, nil
		}

		project := m.projectByID(msg.ProjectID)
		if project == nil {
			return true, nil
		}

		if msg.TaskID != "" {
			task := project.GetTaskByID(msg.TaskID)
			if task == nil {
				_ = m.persister.Load(project)
				task = project.GetTaskByID(msg.TaskID)
			}
			if task != nil {
				return true, m.navigateToTaskInProject(task, project)
			}
		}

		return true, m.navigateToTaskInProject(nil, project)

	case AlertCenterShowDetailMsg:
		if msg.ProjectID == "" || msg.TaskID == "" {
			return true, nil
		}
		project := m.projectByID(msg.ProjectID)
		if project == nil {
			return true, nil
		}
		task := project.GetTaskByID(msg.TaskID)
		if task == nil {
			_ = m.persister.Load(project)
			task = project.GetTaskByID(msg.TaskID)
		}
		if task != nil {
			return true, func() tea.Msg {
				return ShowTaskDetailMsg{Task: task, Project: project}
			}
		}
		return true, nil

	case DismissAlertMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.DismissAlert(msg.Key, msg.Category)
		}); err != nil {
			logging.Warn("failed to save alert dismissal: %v", err)
		}
		m.reevaluateAlerts()
		m.refreshNotificationCenter()
		m.setSuccessStatus("Alert dismissed")
		return true, defaultStatusTimeout()

	case UndismissAlertMsg:
		if err := m.commitPlanningMutation(func(p *config.PlanningState) {
			p.UndismissAlert(msg.Key)
		}); err != nil {
			logging.Warn("failed to save alert restore: %v", err)
		}
		m.reevaluateAlerts()
		m.refreshNotificationCenter()
		m.setSuccessStatus("Alert restored")
		return true, defaultStatusTimeout()

	}

	return false, nil
}
