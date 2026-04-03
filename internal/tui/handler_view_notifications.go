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
			m.showView(ViewDailyReview, NewDailyReviewModel(m.config, m.configManager))
			return true, nil
		case alerts.CategoryWeeklyReview:
			m.showView(ViewWeeklyReview, NewWeeklyReviewModel(m.config, m.configManager))
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

	case CloseNotificationCenterMsg:
		m.popView()
		return true, nil
	}

	return false, nil
}
