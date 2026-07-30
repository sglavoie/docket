package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskViewMessages handles task-related view opening messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskInspectorMsg:
		m.showInspectorOverlay(msg.Task, msg.Project)
		return true, nil

	case ShowTaskDetailMsg:
		return m.handleShowTaskDetail(msg)

	case ShowTaskCommentsMsg:
		return m.handleShowTaskComments(msg)

	case ShowTaskChecklistMsg:
		return m.handleShowTaskChecklist(msg)

	case ShowTaskDecisionsMsg:
		return m.handleShowTaskDecisions(msg)

	case TaskDetailTabChangedMsg:
		return true, m.syncTaskDetailTab(msg.Tab)
	}
	return false, nil
}

// handleShowTaskDetail handles the ShowTaskDetailMsg.
func (m *AppModel) handleShowTaskDetail(msg ShowTaskDetailMsg) (bool, tea.Cmd) {
	return true, m.activateTaskDetail(msg.Task, msg.Project, TaskOverview)
}

// handleShowTaskComments handles the ShowTaskCommentsMsg.
func (m *AppModel) handleShowTaskComments(msg ShowTaskCommentsMsg) (bool, tea.Cmd) {
	return true, m.activateTaskDetail(msg.Task, msg.Project, TaskComments)
}

// handleShowTaskChecklist handles the ShowTaskChecklistMsg.
func (m *AppModel) handleShowTaskChecklist(msg ShowTaskChecklistMsg) (bool, tea.Cmd) {
	return true, m.activateTaskDetail(msg.Task, msg.Project, TaskChecklist)
}

// handleShowTaskDecisions handles the ShowTaskDecisionsMsg.
func (m *AppModel) handleShowTaskDecisions(msg ShowTaskDecisionsMsg) (bool, tea.Cmd) {
	return true, m.activateTaskDetail(msg.Task, msg.Project, TaskDecisions)
}
