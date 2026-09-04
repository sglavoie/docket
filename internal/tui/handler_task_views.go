package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// handleTaskViewMessages handles task-related view opening messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskdetail.ShowTaskCommentsMsg:
		return m.handleShowTaskComments(msg)

	}
	return false, nil
}

// handleShowTaskComments handles the ShowTaskCommentsMsg.
func (m *AppModel) handleShowTaskComments(msg taskdetail.ShowTaskCommentsMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}
	view := taskdetail.NewTaskCommentsModelWithEditor(
		msg.Task,
		msg.Project,
		"",
		m.secondaryFeatures.Subview,
	)
	view.SetOperationDispatch(m.dispatchOperationKey)
	return m.showViewInit(presentation.ViewTaskComments, view)
}
