package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// handleTaskCRUDMessages handles task CRUD messages (create, edit, delete).
// Returns true if the message was handled.
func (m *AppModel) handleTaskCRUDMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case taskform.CreateTaskMsg:
		return m.handleCreateTask(msg)

	case coordinator.TaskCreatedMsg:
		return m.handleTaskCreated(msg)

	case TaskDeleteRequestedMsg:
		m.handleTaskDeleteRequested(msg)
		return true, nil

	}
	return false, nil
}

// handleCreateTask handles the CreateTaskMsg.
func (m *AppModel) handleCreateTask(msg taskform.CreateTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project == nil {
		return true, nil
	}

	return true, m.openTaskComposer(m.createComposerPreset(project, msg.ParentTask))
}

// handleTaskCreated handles the coordinator.TaskCreatedMsg.
func (m *AppModel) handleTaskCreated(msg coordinator.TaskCreatedMsg) (bool, tea.Cmd) {
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
	cmd := m.viewNav.views.Update(m.currentView(), msg)

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
		identity := taskref.For(task, project)
		if m.todayIDs[identity] {
			return models.StateGroupToday.Label()
		}
		if m.nextOrderIDs[identity] {
			return models.StateGroupNext.Label()
		}
	}
	return theme.TaskStateGroupLabel(task)
}
