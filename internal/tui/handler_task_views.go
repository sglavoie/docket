package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskViewMessages handles task-related view opening messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTaskCommentsMsg:
		return m.handleShowTaskComments(msg)

	case ShowTaskChecklistMsg:
		return m.handleShowTaskChecklist(msg)

	}
	return false, nil
}

// handleShowTaskComments handles the ShowTaskCommentsMsg.
func (m *AppModel) handleShowTaskComments(msg ShowTaskCommentsMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}
	view := NewTaskCommentsModelWithEditor(
		msg.Task,
		msg.Project,
		"",
		m.secondaryFeatures.Subview,
	)
	return m.showViewInit(ViewTaskComments, view)
}

// handleShowTaskChecklist handles the ShowTaskChecklistMsg.
func (m *AppModel) handleShowTaskChecklist(msg ShowTaskChecklistMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}
	view := NewTaskChecklistModelWithEditor(
		msg.Task,
		msg.Project,
		"",
		m.secondaryFeatures.Subview,
	)
	return m.showViewInit(ViewTaskChecklist, view)
}
