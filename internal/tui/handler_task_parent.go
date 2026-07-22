package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskParentMessages handles task parent selector messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskParentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenParentSelectorMsg:
		return m.handleOpenParentSelector(msg)

	case ParentSelectedMsg:
		return m.handleParentSelected(msg)
	}
	return false, nil
}

// handleOpenParentSelector handles the OpenParentSelectorMsg.
func (m *AppModel) handleOpenParentSelector(msg OpenParentSelectorMsg) (bool, tea.Cmd) {
	selector := NewParentSelectorModel(msg.ValidParents, msg.CurrentID, msg.AllowNone, msg.Source)
	selector.reparentTask = msg.ReparentTask
	_, cmd := m.showViewInit(ViewParentSelector, selector)
	return true, cmd
}

// handleParentSelected handles the ParentSelectedMsg.
func (m *AppModel) handleParentSelected(msg ParentSelectedMsg) (bool, tea.Cmd) {
	// Pop the parent selector view
	m.popView()
	// Forward the message directly to the underlying view (note/task creator)
	return true, m.delegateToCurrentView(msg)
}
