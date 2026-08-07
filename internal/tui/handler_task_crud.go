package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
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

	case TaskDeleteRequestedMsg:
		m.handleTaskDeleteRequested(msg)
		return true, nil

	case TaskDeletedMsg:
		return m.handleTaskDeleted(msg)
	}
	return false, nil
}

// handleCreateTask handles the CreateTaskMsg.
func (m *AppModel) handleCreateTask(msg CreateTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, nil
	}

	return true, m.openTaskComposer(m.createComposerPreset(project, msg.ParentTask))
}

// handleEditTask handles the EditTaskMsg.
func (m *AppModel) handleEditTask(msg EditTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project != nil && msg.Task != nil {
		return m.handleEditorMessages(ShowTaskTextEditorMsg{
			Task: msg.Task, Project: project, Operation: app.TaskOperationEditTitle,
		})
	}
	return true, nil
}

// handleTaskCreated handles the TaskCreatedMsg.
func (m *AppModel) handleTaskCreated(msg TaskCreatedMsg) (bool, tea.Cmd) {
	// Capture whether this creation originated from the Quick Capture view and the
	// pre-capture task count before handleWithRefresh pops the view / reloads.
	// Coordinator path: the capture committed one atomic ChangeSet through the
	// application layer, so reload the committed project from disk and re-resolve
	// the task by ID. The shared navigation/status/open-after logic below then runs
	// off the reloaded pointers exactly as it did for the legacy in-place save.
	planningRefreshed := false
	if msg.PlanningChanged {
		planningRefreshed = m.refreshRuntime()
	}
	if msg.Result != nil {
		m.reloadProjectByID(msg.Result.ProjectID)
		if project := m.projectByID(msg.Result.ProjectID); project != nil {
			msg.Project = project
			// OpenTaskID overrides the navigation/open target (task-form
			// open-after-subtask targets a created child, not the parent).
			targetID := msg.Result.TaskID
			if msg.OpenTaskID != "" {
				targetID = msg.OpenTaskID
			}
			msg.Task = project.GetTaskByID(targetID)
		}
	}
	if planningRefreshed {
		m.rebuildPlanningFromRuntime()
	}

	refreshCmd := m.handleWithRefresh(true, true)

	if msg.Task != nil {
		project := msg.Project
		if project == nil && msg.Result != nil {
			project = m.projectByID(msg.Result.ProjectID)
		}
		if m.crossOutliner != nil && project != nil {
			m.crossOutliner.NavigateToTaskInProject(msg.Task, project)
		}
	}

	// Set status message showing section
	if msg.Task != nil {
		if bucket := m.createdTaskSectionLabel(msg.Task, msg.Project); bucket != "" {
			if parked := m.parkedOwnerLabel(msg.Project); parked != "" {
				m.setSuccessStatus(fmt.Sprintf("Created in %s — %s", bucket, parked))
			} else {
				m.setSuccessStatus(fmt.Sprintf("Created in %s", bucket))
			}
		}
	}
	if msg.CreatedCount > 1 {
		m.setSuccessStatus(fmt.Sprintf("Created %d tasks", msg.CreatedCount))
	}
	successMessage := m.statusMessage
	if successMessage == "" {
		successMessage = "Created task"
	}
	transactionID := ""
	if msg.Result != nil {
		transactionID = msg.Result.TransactionID
	}
	feedbackCmd := m.setRecoveryFeedback(successMessage, transactionID, false)

	// Delegate to the current view so it can reload its state
	cmd := m.views.Update(m.currentView(), msg)

	// If open-after requested, re-lookup the task (stale after reload) and open
	if msg.Task != nil && msg.OpenContent {
		project := msg.Project
		if project == nil {
			project = m.currentProject
		}
		if project != nil {
			if freshTask := project.GetTaskByID(msg.Task.ID); freshTask != nil {
				openMsg := EditTaskContentMsg{Task: freshTask, Project: project}
				return true, tea.Batch(refreshCmd, cmd, feedbackCmd, func() tea.Msg {
					return openMsg
				})
			}
		}
	}

	return true, tea.Batch(refreshCmd, cmd, feedbackCmd)
}

func (m *AppModel) createdTaskSectionLabel(task *models.Task, project *models.Project) string {
	if task == nil {
		return ""
	}
	if project != nil {
		identity := taskIdentityFor(task, project)
		if m.todayIDs[identity] {
			return models.StateGroupToday.Label()
		}
		if m.nextOrderIDs[identity] {
			return models.StateGroupNext.Label()
		}
	}
	return taskStateGroupLabel(task)
}

// handleTaskUpdated handles the TaskUpdatedMsg.
func (m *AppModel) handleTaskUpdated(msg TaskUpdatedMsg) (bool, tea.Cmd) {
	// Coordinator path: the edit committed one atomic ChangeSet through the
	// application layer, so reload the committed project from disk before the
	// shared refresh runs.
	if msg.Result != nil {
		m.reloadProjectByID(msg.Result.ProjectID)
	}
	if msg.PlanningChanged && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}

	// Common pattern: pop view, reload, refresh
	refreshCmd := m.handleWithRefresh(true, true)

	// Delegate to the current view so it can reload its state
	cmd := m.views.Update(m.currentView(), msg)
	transactionID := ""
	if msg.Result != nil {
		transactionID = msg.Result.TransactionID
	}
	feedbackCmd := m.setRecoveryFeedback("Updated task", transactionID, false)
	return true, tea.Batch(refreshCmd, cmd, feedbackCmd)
}

// handleTaskDeleted handles the TaskDeletedMsg.
func (m *AppModel) handleTaskDeleted(msg TaskDeletedMsg) (bool, tea.Cmd) {
	m.pruneDeletedTaskRefs()
	refreshCmd := m.handleWithRefresh(false, false)

	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, refreshCmd
	}

	return true, tea.Batch(refreshCmd, m.setRecoveryFeedback("Deleted task", msg.TransactionID, false))
}
