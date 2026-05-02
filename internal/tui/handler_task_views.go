package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskViewMessages handles task-related view opening messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskDetailMsg:
		return m.handleShowTaskDetail(msg)

	case ShowTaskCommentsMsg:
		return m.handleShowTaskComments(msg)

	case ShowTaskChecklistMsg:
		return m.handleShowTaskChecklist(msg)

	case OpenTaskActivityMsg:
		return m.handleOpenTaskActivity(msg)
	}
	return false, nil
}

// handleShowTaskDetail handles the ShowTaskDetailMsg.
func (m *AppModel) handleShowTaskDetail(msg ShowTaskDetailMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.lastNavigatedTaskID = msg.Task.ID
		m.recordTaskVisit(msg.Task, msg.Project)
		m.showView(ViewTaskDetail, NewTaskDetailModel(msg.Task, msg.Project, m.config, m.configManager))
	}
	return true, nil
}

// handleShowTaskComments handles the ShowTaskCommentsMsg.
func (m *AppModel) handleShowTaskComments(msg ShowTaskCommentsMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.lastNavigatedTaskID = msg.Task.ID
		m.recordTaskVisit(msg.Task, msg.Project)
		m.showView(ViewTaskComments, NewTaskCommentsModelWithEditor(msg.Task, msg.Project, m.config.Editor))
	}
	return true, nil
}

// handleShowTaskChecklist handles the ShowTaskChecklistMsg.
func (m *AppModel) handleShowTaskChecklist(msg ShowTaskChecklistMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.lastNavigatedTaskID = msg.Task.ID
		m.recordTaskVisit(msg.Task, msg.Project)
		m.showView(ViewTaskChecklist, NewTaskChecklistModelWithEditor(msg.Task, msg.Project, m.config.Editor))
	}
	return true, nil
}

// handleOpenTaskActivity handles the OpenTaskActivityMsg.
func (m *AppModel) handleOpenTaskActivity(msg OpenTaskActivityMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.lastNavigatedTaskID = msg.Task.ID
		m.recordTaskVisit(msg.Task, msg.Project)
		m.showView(ViewTaskActivity, NewTaskActivityModel(msg.Task, msg.Project))
	}
	return true, nil
}
