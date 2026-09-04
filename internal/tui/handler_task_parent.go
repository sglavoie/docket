package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// handleTaskParentMessages handles task parent selector messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskParentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenParentSelectorMsg:
		return m.handleOpenParentSelector(msg)

	case listsurface.ParentSelectedMsg:
		return m.handleParentSelected(msg)
	}
	return false, nil
}

// handleOpenParentSelector handles the OpenParentSelectorMsg.
func (m *AppModel) handleOpenParentSelector(msg OpenParentSelectorMsg) (bool, tea.Cmd) {
	selector := listsurface.NewParentSelectorModel(msg.ValidParents, msg.CurrentID, msg.AllowNone, msg.Source)
	selector.ReparentTask = msg.ReparentTask
	_, cmd := m.showViewInit(presentation.ViewParentSelector, selector)
	return true, cmd
}

// handleParentSelected handles the ParentSelectedMsg.
func (m *AppModel) handleParentSelected(msg listsurface.ParentSelectedMsg) (bool, tea.Cmd) {
	// Pop the parent selector view
	m.popView()
	// Forward the message directly to the underlying view (note/task creator)
	return true, m.delegateToCurrentView(msg)
}
