package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/logging"
)

// handleNotificationMessages handles alert center and notification center messages.
func (m *AppModel) handleNotificationMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowNotificationCenterMsg:
		m.reevaluateAlerts()
		m.showView(ViewNotificationCenter, NewNotificationCenterModel(m.proactiveAlerts, m.rawAlerts, m.dismissedSet))
		return true, nil

	case AlertCenterNavigateMsg:
		m.popView()
		switch msg.Category {
		case alerts.CategoryDailyReview:
			dr := NewDailyReviewModel(m.config, m.configManager)
			dr.queueIDs = m.focusQueueIDs
			dr.dailyPlanIDs = m.dailyPlanIDs
			m.showView(ViewDailyReview, dr)
			return true, nil
		case alerts.CategoryWeeklyReview:
			wr := NewWeeklyReviewModel(m.config, m.configManager)
			wr.queueIDs = m.focusQueueIDs
			wr.dailyPlanIDs = m.dailyPlanIDs
			m.showView(ViewWeeklyReview, wr)
			return true, nil
		default:
		}

		if msg.ProjectID == "" {
			return true, nil
		}

		project := m.config.GetProject(msg.ProjectID)
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

	case DismissAlertMsg:
		m.config.DismissAlert(msg.Key, msg.Category)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.refreshNotificationCenter()
		m.setSuccessStatus("Alert dismissed")
		return true, defaultStatusTimeout()

	case UndismissAlertMsg:
		m.config.UndismissAlert(msg.Key)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		m.reevaluateAlerts()
		m.refreshNotificationCenter()
		m.setSuccessStatus("Alert restored")
		return true, defaultStatusTimeout()

	}

	return false, nil
}
