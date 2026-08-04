package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskCommentMessages handles task comment messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskCommentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CommentAddedMsg:
		return m.handleCommentAdded(msg)

	case CommentDeletedMsg:
		return m.handleCommentDeleted(msg)

	case CommentEditedMsg:
		return m.handleCommentEdited(msg)

	}
	return false, nil
}

// refreshAfterSubviewCommit re-syncs the task views after a subview panel
// mutation committed through the application layer. The commit loaded a fresh
// snapshot into the shared project pointers, so views caching task pointers
// (tree nodes, outliner rows) must re-resolve to observe the committed objects
// — the raw-save path they replaced mutated the shared objects in place and
// never needed this.
func (m *AppModel) refreshAfterSubviewCommit() tea.Cmd {
	return tea.Batch(m.refreshTaskViewsAfterSave()...)
}

// handleCommentAdded handles the CommentAddedMsg.
func (m *AppModel) handleCommentAdded(msg CommentAddedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Comment added")
	// Delegate to current view so it can refresh stale task data
	// (outliner rows contain clones from tree projections)
	viewCmd := m.delegateToCurrentView(msg)
	return true, tea.Batch(defaultStatusTimeout(), viewCmd, m.refreshAfterSubviewCommit())
}

// handleCommentDeleted handles the CommentDeletedMsg.
func (m *AppModel) handleCommentDeleted(msg CommentDeletedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Comment deleted")
	return true, tea.Batch(defaultStatusTimeout(), m.refreshAfterSubviewCommit())
}

// handleCommentEdited handles the CommentEditedMsg.
func (m *AppModel) handleCommentEdited(msg CommentEditedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Comment updated")
	return true, tea.Batch(defaultStatusTimeout(), m.refreshAfterSubviewCommit())
}
