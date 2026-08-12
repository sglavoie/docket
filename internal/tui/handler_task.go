package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate to specialized handlers by category

	// CRUD operations
	if handled, cmd := m.handleTaskCRUDMessages(msg); handled {
		return handled, cmd
	}

	// Parent selector messages
	if handled, cmd := m.handleTaskParentMessages(msg); handled {
		return handled, cmd
	}

	// View-opening messages (detail, comments)
	if handled, cmd := m.handleTaskViewMessages(msg); handled {
		return handled, cmd
	}

	// Comment messages
	if handled, cmd := m.handleTaskCommentMessages(msg); handled {
		return handled, cmd
	}

	// Command palette task actions
	// Content editing messages
	if handled, cmd := m.handleTaskContentMessages(msg); handled {
		return handled, cmd
	}

	return false, nil
}

// handleTaskContentMessages handles task content editing messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskContentMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case EditTaskContentMsg:
		return m.handleEditTaskContent(msg)

	case TaskContentEditedMsg:
		return m.handleTaskContentEdited(msg)

	case OpenOutlineEditorMsg:
		return true, m.handleOpenOutlineEditor(msg)

	case OutlineEditedMsg:
		return true, m.handleOutlineEdited(msg)

	case TaskLineEditedMsg:
		return true, m.handleTaskLineEdited(msg)

	case TaskLineEditFailedMsg:
		return true, m.handleTaskLineEditFailed(msg)

	}
	return false, nil
}
