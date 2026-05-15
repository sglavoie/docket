package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

// handleTaskCRUDMessages handles task CRUD messages (create, edit, delete).
// Returns true if the message was handled.
func (m *AppModel) handleTaskCRUDMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CreateTaskMsg:
		return m.handleCreateTask(msg)

	case EditTaskMsg:
		return m.handleEditTask(msg)

	case TaskCreatedMsg:
		return m.handleTaskCreated(msg)

	case TaskUpdatedMsg:
		return m.handleTaskUpdated(msg)

	case TaskDeletedMsg:
		return m.handleTaskDeleted(msg)
	}
	return false, nil
}

// handleCreateTask handles the CreateTaskMsg.
func (m *AppModel) handleCreateTask(msg CreateTaskMsg) (bool, tea.Cmd) {
	if m.currentProject == nil {
		return true, nil
	}

	// Check if there are enabled templates (unless explicitly skipped)
	if !msg.SkipTemplatePicker {
		enabledTemplates := m.config.GetEnabledTaskTemplates()
		if len(enabledTemplates) > 0 {
			// Store the parent task and show template picker
			m.pendingCreateTaskParent = msg.ParentTask
			_, cmd := m.showViewInit(ViewTaskTemplatePicker, NewTemplatePickerModel(m.config, m.currentProject))
			return true, cmd
		}
	}
	// No templates or skipped, go directly to task form
	m.showView(ViewTaskForm, NewTaskFormModelCreate(m.currentProject, m.config, m.configManager, msg.ParentTask))
	return true, nil
}

// handleEditTask handles the EditTaskMsg.
func (m *AppModel) handleEditTask(msg EditTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project != nil && msg.Task != nil {
		m.showView(ViewTaskForm, NewTaskFormModelEdit(project, msg.Task, m.config, m.configManager))
	}
	return true, nil
}

// handleTaskCreated handles the TaskCreatedMsg.
func (m *AppModel) handleTaskCreated(msg TaskCreatedMsg) (bool, tea.Cmd) {
	if msg.Task != nil {
		m.sessionStats.tasksCreated++
	}
	// When created from the floating overlay, close it without popping the view stack.
	fromOverlay := m.quickCaptureOverlay != nil
	m.quickCaptureOverlay = nil
	m.handleWithRefresh(!fromOverlay, true, "")

	// Navigate cursor to the newly created task
	if msg.Task != nil && m.currentProject != nil {
		if fresh := m.currentProject.GetTaskByID(msg.Task.ID); fresh != nil {
			if m.projectTree != nil {
				m.projectTree.NavigateToTask(fresh)
			}
			if m.outliner != nil {
				m.outliner.NavigateToSidebarTask(fresh)
			}
		}
	}

	// Set status message showing section
	if msg.Task != nil && msg.Task.Section != "" {
		if msg.Task.CustomSection != "" {
			m.setSuccessStatus(fmt.Sprintf("Created in %s (%s)", msg.Task.Section, msg.Task.CustomSection))
		} else {
			m.setSuccessStatus(fmt.Sprintf("Created in %s", msg.Task.Section))
		}
	}

	// Delegate to current view (e.g., inbox processor) so it can reload its state
	cmd := m.views.Update(m.currentView(), msg)
	m.syncInboxProcessor()

	// If open-after requested, re-lookup the task (stale after reload) and open
	if msg.Task != nil && (msg.OpenContent || msg.OpenDetail) {
		project := msg.Project
		if project == nil {
			project = m.currentProject
		}
		if project != nil {
			if freshTask := project.GetTaskByID(msg.Task.ID); freshTask != nil {
				var openMsg tea.Msg
				if msg.OpenDetail {
					openMsg = ShowTaskDetailMsg{Task: freshTask, Project: project}
				} else {
					openMsg = EditTaskContentMsg{Task: freshTask, Project: project}
				}
				return true, tea.Batch(cmd, statusTimeout(2*time.Second), func() tea.Msg {
					return openMsg
				})
			}
		}
	}

	return true, tea.Batch(cmd, statusTimeout(2*time.Second))
}

// handleTaskUpdated handles the TaskUpdatedMsg.
func (m *AppModel) handleTaskUpdated(msg TaskUpdatedMsg) (bool, tea.Cmd) {
	// Common pattern: pop view, reload, refresh
	m.handleWithRefresh(true, true, "")
	// Delegate to current view (e.g., inbox processor) so it can reload its state
	cmd := m.views.Update(m.currentView(), msg)
	m.syncInboxProcessor()
	return true, cmd
}

// handleTaskDeleted handles the TaskDeletedMsg.
func (m *AppModel) handleTaskDeleted(msg TaskDeletedMsg) (bool, tea.Cmd) {
	m.pruneDeletedTaskRefs()
	refreshCmd := m.handleWithRefresh(false, false, "")

	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, refreshCmd
	}

	// Pull the latest undo snapshot so the toast labels exactly what
	// `u` will undo. Falling back to the in-message task title would
	// drift if the project has no undo entry recorded.
	entries, err := storage.LoadUndoHistory(project)
	if err != nil || len(entries) == 0 {
		return true, refreshCmd
	}
	latest := entries[0]

	toast := formatUndoToast(latest.Action, latest.Context)
	undoCb := func() tea.Cmd {
		return performUndoCmd(m.persister, project)
	}
	return true, tea.Batch(refreshCmd, m.setUndoableStatus(toast, undoCb))
}
