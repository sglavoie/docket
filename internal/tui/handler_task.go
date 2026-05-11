package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate to specialized handlers by category

	// Dependency messages (already delegated)
	if handled, cmd := m.handleDependencyMessages(msg); handled {
		return handled, cmd
	}

	// CRUD operations
	if handled, cmd := m.handleTaskCRUDMessages(msg); handled {
		return handled, cmd
	}

	// Archive operations
	if handled, cmd := m.handleTaskArchiveMessages(msg); handled {
		return handled, cmd
	}

	// Date-related messages (due dates, snooze)
	if handled, cmd := m.handleTaskDateMessages(msg); handled {
		return handled, cmd
	}

	// Undo/redo operations
	if handled, cmd := m.handleTaskUndoMessages(msg); handled {
		return handled, cmd
	}

	// Parent selector messages
	if handled, cmd := m.handleTaskParentMessages(msg); handled {
		return handled, cmd
	}

	// View-opening messages (detail, comments, checklist, activity)
	if handled, cmd := m.handleTaskViewMessages(msg); handled {
		return handled, cmd
	}

	// Checklist item messages
	if handled, cmd := m.handleTaskChecklistMessages(msg); handled {
		return handled, cmd
	}

	// Comment messages
	if handled, cmd := m.handleTaskCommentMessages(msg); handled {
		return handled, cmd
	}

	// Decision messages
	if handled, cmd := m.handleTaskDecisionMessages(msg); handled {
		return handled, cmd
	}

	// Link-related messages (linked tasks, cross-project links)
	if handled, cmd := m.handleTaskLinkMessages(msg); handled {
		return handled, cmd
	}

	// Status and property change messages
	if handled, cmd := m.handleTaskStatusPropertyMessages(msg); handled {
		return handled, cmd
	}

	// Command palette task actions
	if handled, cmd := m.handleTaskPaletteMessages(msg); handled {
		return handled, cmd
	}

	// Content editing messages
	if handled, cmd := m.handleTaskContentMessages(msg); handled {
		return handled, cmd
	}

	// Move operations (already in separate file)
	if handled, cmd := m.handleTaskMoveMessages(msg); handled {
		return handled, cmd
	}

	return false, nil
}

// handleTaskArchiveMessages handles task archive messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskArchiveMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ArchiveCompletedTasksMsg:
		return m.handleArchiveCompletedTasks()

	case ArchiveAllCompletedTasksMsg:
		return m.handleArchiveAllCompletedTasks()

	case TasksArchivedMsg:
		m.pruneDeletedTaskRefs()
		statusMsg := fmt.Sprintf("Archived %d task(s) to completed.md", msg.Count)
		if msg.Projects > 0 {
			statusMsg = fmt.Sprintf("Archived %d task(s) across %d project(s)", msg.Count, msg.Projects)
		}
		return true, m.handleWithRefresh(false, false, statusMsg)
	}
	return false, nil
}

// handleTaskStatusPropertyMessages handles task status and property change messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskStatusPropertyMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case TaskStatusChangedMsg:
		if msg.Task != nil && msg.Task.Status == models.StatusDone {
			m.sessionStats.tasksCompleted++
		}
		return true, m.handleWithRefresh(false, false, "")

	case TaskPropertyChangedMsg:
		m.handleWithRefresh(false, false, "")
		cmd := m.views.Update(m.currentView(), msg)
		m.syncInboxProcessor()
		if msg.NeedsSave && m.currentProject != nil {
			cmd = tea.Batch(cmd, SaveProjectAsync(m.currentProject, ""))
		}
		return true, cmd
	}
	return false, nil
}

// handleTaskPaletteMessages handles command palette task action messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskPaletteMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CommandPaletteTaskActionMsg:
		return m.handleCommandPaletteTaskAction(msg)

	case CommandPaletteBulkActionMsg:
		return m.handleCommandPaletteBulkAction(msg)

	case InlineTaskCreateMsg:
		return m.handleInlineTaskCreate(msg)
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

	case DescriptionEditorDoneMsg:
		return m.handleDescriptionEditorDone(msg)
	}
	return false, nil
}

// handleTaskMoveMessages handles task move messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskMoveMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowProjectSelectorForMoveMsg:
		if msg.Task != nil && msg.SourceProject != nil {
			projects := m.config.GetActiveProjects()
			m.showView(ViewProjectSelector, NewProjectSelectorModel(projects, msg.Task, msg.SourceProject))
		}
		return true, nil

	case MoveTaskToProjectMsg:
		return m.handleMoveTaskToProject(msg)

	case TaskMovedToProjectMsg:
		return m.handleTaskMovedToProject(msg)
	}
	return false, nil
}
