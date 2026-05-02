package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskCommentMessages handles task comment messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskCommentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenQuickNoteMsg:
		return m.handleOpenQuickNote(msg)

	case CommentAddedMsg:
		return m.handleCommentAdded(msg)

	case CommentDeletedMsg:
		return m.handleCommentDeleted(msg)

	case CommentEditedMsg:
		return m.handleCommentEdited(msg)

	case CommentResolvedMsg:
		return m.handleCommentResolved(msg)
	}
	return false, nil
}

// handleOpenQuickNote handles the OpenQuickNoteMsg.
func (m *AppModel) handleOpenQuickNote(msg OpenQuickNoteMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.quickNoteOverlay = newQuickNoteOverlay(msg.Task, msg.Project)
		return true, m.quickNoteOverlay.initCmd()
	}
	return true, nil
}

// handleCommentAdded handles the CommentAddedMsg.
func (m *AppModel) handleCommentAdded(msg CommentAddedMsg) (bool, tea.Cmd) {
	m.quickNoteOverlay = nil
	m.setSuccessStatus("Comment added")
	// Delegate to current view so it can refresh stale task data
	// (outliner rows contain clones from filterTaskTree)
	viewCmd := m.delegateToCurrentView(msg)
	return true, tea.Batch(defaultStatusTimeout(), viewCmd)
}

// handleCommentDeleted handles the CommentDeletedMsg.
func (m *AppModel) handleCommentDeleted(msg CommentDeletedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Comment deleted")
	return true, defaultStatusTimeout()
}

// handleCommentEdited handles the CommentEditedMsg.
func (m *AppModel) handleCommentEdited(msg CommentEditedMsg) (bool, tea.Cmd) {
	m.setSuccessStatus("Comment updated")
	return true, defaultStatusTimeout()
}

// handleCommentResolved handles the CommentResolvedMsg.
func (m *AppModel) handleCommentResolved(msg CommentResolvedMsg) (bool, tea.Cmd) {
	if msg.Task != nil && len(msg.Task.Comments) > 0 {
		// Find the toggled comment from the comments view cursor
		if view := m.views.Get(ViewTaskComments); view != nil {
			if commentsView, ok := view.(*TaskCommentsModel); ok {
				idx := commentsView.cursor
				if idx >= 0 && idx < len(msg.Task.Comments) {
					if msg.Task.Comments[idx].Resolved {
						m.setSuccessStatus("Comment resolved")
					} else {
						m.setSuccessStatus("Comment unresolved")
					}
					return true, defaultStatusTimeout()
				}
			}
		}
	}
	m.setSuccessStatus("Comment resolved")
	return true, defaultStatusTimeout()
}
