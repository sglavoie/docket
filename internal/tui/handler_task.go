package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
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

	// View-opening messages (detail, comments, checklist)
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

// handleTaskStatusPropertyMessages handles task status and property change messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskStatusPropertyMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case TaskStatusChangedMsg:
		if msg.Task != nil && msg.Task.State() == models.StateDone {
			m.sessionStats.tasksCompleted++
			m.handleWithRefresh(false, false, "")
			m.setSuccessStatus(fmt.Sprintf("Done! (%d today)", m.sessionStats.tasksCompleted))
			cmd := defaultStatusTimeout()
			if bannerCmd := m.maybeFireTodayCleared(); bannerCmd != nil {
				cmd = tea.Batch(cmd, bannerCmd)
			}
			return true, cmd
		}
		return true, m.handleWithRefresh(false, false, "")

	case TaskPropertyChangedMsg:
		m.handleWithRefresh(false, false, "")
		cmd := m.views.Update(m.currentView(), msg)
		if msg.NeedsSave && m.currentProject != nil {
			// The palette/command handlers already applied the property change to the
			// task(s) in m.currentProject in memory (toggle active/pin, set priority,
			// set due date). Commit that mutated project through the application layer
			// so the durable write flows through the transactional repository and
			// workspace ledger instead of the legacy async save wrapper.
			// The shell reloads the touched project via the BatchTransactionResultMsg.
			cmd = tea.Batch(cmd, m.secondaryFeatures.TaskItem.CommitProject(m.currentProject, projectCommitOptsFromTaskPropertyMsg(msg), ""))
		}
		return true, cmd
	}
	return false, nil
}

// projectCommitOptsFromTaskPropertyMsg builds the project-scoped commit options
// for an in-place task property change.
func projectCommitOptsFromTaskPropertyMsg(msg TaskPropertyChangedMsg) app.ProjectCommitOptions {
	return app.ProjectCommitOptions{
		UndoAction:  msg.UndoAction,
		UndoContext: msg.UndoContext,
	}
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
			projects := m.activeProjects()
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

// maybeFireTodayCleared evaluates whether this status mutation cleared the
// Today plan. The banner is session-only; streak and cleared-date bookkeeping
// are not part of the lean planning domain.
func (m *AppModel) maybeFireTodayCleared() tea.Cmd {
	if m.runtime == nil {
		return nil
	}
	loops := m.openLoops
	if loops.TodayTotal == 0 || loops.TodayDone != loops.TodayTotal {
		return nil
	}
	m.todayClearedBanner = "Today cleared."
	return todayClearedBannerTimeout()
}
