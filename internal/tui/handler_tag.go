package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ShowTagSelectorMsg:
		selector := NewTagSelectorModel(msg.Task, msg.Project, msg.CurrentTags)
		m.showView(ViewTagSelector, selector)
		return true, nil

	case TagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		// Check if we came from project tree, outliner, or cross-project outliner - apply tags directly
		if m.currentView() == ViewCrossProjectOutliner && msg.Task != nil {
			return true, m.applyTagsToTask(msg.Task, msg.SelectedTags)
		}
		// Otherwise forward to the current view.
		cmd := m.views.Update(m.currentView(), msg)
		return true, cmd

	}
	return false, nil
}
