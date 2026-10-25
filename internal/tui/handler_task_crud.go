package tui

import (
	"cmp"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/taskform"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
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
	// A capture that created its projects changed the catalog too, and the
	// reload below can only find a project the runtime already knows.
	if msg.PlanningChanged || msg.ProjectsCreated {
		planningRefreshed = m.refreshRuntime()
	}
	if msg.ProjectsCreated {
		m.refreshProjectHubIfActive()
	}
	// A multi-line capture can span projects, and the planning rebuild below
	// resolves membership against the loaded tasks, so every project the commit
	// touched is reloaded before it, not just the first result's.
	reloaded := make(map[string]bool, len(msg.Results)+1)
	reload := func(projectID string) {
		if projectID != "" && !reloaded[projectID] {
			reloaded[projectID] = true
			m.reloadProjectByID(projectID)
		}
	}
	for _, result := range msg.Results {
		reload(result.ProjectID)
	}
	if msg.Result != nil {
		reload(msg.Result.ProjectID)
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

	refreshCmd := m.handleWithRefresh(!msg.KeepComposerOpen, true)

	if msg.Task != nil && !msg.KeepComposerOpen {
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
	successMessage := m.feedback.message
	if successMessage == "" {
		successMessage = "Created task"
	}
	transactionID := ""
	if msg.Result != nil {
		transactionID = msg.Result.TransactionID
	}
	// A creation has no before address, so undoing it correlates to nothing and
	// leaves the view alone; redoing it puts the cursor back on the task it mints
	// again. A multiline capture created several tasks and names none of them.
	created := coordinator.CommittedFocusTarget{}
	if msg.Result != nil && msg.CreatedCount <= 1 {
		created.After = app.TaskLocator{ProjectID: msg.Result.ProjectID, TaskID: msg.Result.TaskID}
	}
	m.undoFocus.ObserveCommit(transactionID, created)
	successMessage = cmp.Or(msg.RecoveryWarning, successMessage)
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
