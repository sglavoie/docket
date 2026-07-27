package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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

	case TaskDeleteRequestedMsg:
		return true, m.handleTaskDeleteRequested(msg)

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

	// Check if there are enabled templates (unless explicitly skipped)
	if !msg.SkipTemplatePicker {
		enabledTemplates := m.runtime.Automation().GetEnabledTaskTemplates()
		if len(enabledTemplates) > 0 {
			// Store the parent task and show template picker
			m.pendingCreateTaskParent = msg.ParentTask
			m.pendingCreateTaskProject = project
			_, cmd := m.showViewInit(ViewTaskTemplatePicker, NewTemplatePickerModel(enabledTemplates, project))
			return true, cmd
		}
	}
	// No templates or skipped, go directly to the compact task composer.
	return true, m.openTaskComposer(m.createComposerPreset(project, msg.ParentTask))
}

// handleEditTask handles the EditTaskMsg.
func (m *AppModel) handleEditTask(msg EditTaskMsg) (bool, tea.Cmd) {
	project := msg.Project
	if project == nil {
		project = m.currentProject
	}
	if project != nil && msg.Task != nil {
		preset := TaskComposerPreset{Kind: ComposerEdit, Project: project, Task: msg.Task}
		return true, m.openTaskComposer(preset)
	}
	return true, nil
}

// handleTaskCreated handles the TaskCreatedMsg.
func (m *AppModel) handleTaskCreated(msg TaskCreatedMsg) (bool, tea.Cmd) {
	// Capture whether this creation originated from the Quick Capture view and the
	// pre-capture task count before handleWithRefresh pops the view / reloads.
	fromQuickCapture := m.taskComposer != nil && m.taskComposer.preset.Kind == ComposerGlobal
	preCaptureCount := 0
	if m.currentProject != nil {
		preCaptureCount = len(m.currentProject.Tasks)
	}

	// Coordinator path: the capture committed one atomic ChangeSet through the
	// application layer, so reload the committed project from disk and re-resolve
	// the task by ID. The shared navigation/status/open-after logic below then runs
	// off the reloaded pointers exactly as it did for the legacy in-place save.
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

	if msg.CreatedCount > 0 {
		m.sessionStats.tasksCreated += msg.CreatedCount
	} else if msg.Task != nil {
		m.sessionStats.tasksCreated++
	}
	if msg.Task != nil {
		m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.TaskCaptured = true })
	}
	refreshCmd := m.handleWithRefresh(true, true, "")

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
		if bucket := taskStateGroupLabel(msg.Task); bucket != "" {
			m.setSuccessStatus(fmt.Sprintf("Created in %s", bucket))
		}
	}
	if msg.CreatedCount > 1 {
		m.setSuccessStatus(fmt.Sprintf("Created %d tasks", msg.CreatedCount))
	}

	// Override with onboarding next-step hint when this was the first task captured
	if fromQuickCapture && msg.Task != nil {
		if hint := captureNextHint(preCaptureCount); hint != "" {
			m.setSuccessStatus(formatSuccessWithNext("Task captured", hint))
		}
	}

	// Delegate to the current view so it can reload its state
	cmd := m.views.Update(m.currentView(), msg)

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
				return true, tea.Batch(refreshCmd, cmd, statusTimeout(2*time.Second), func() tea.Msg {
					return openMsg
				})
			}
		}
	}

	return true, tea.Batch(refreshCmd, cmd, statusTimeout(2*time.Second))
}

// handleTaskUpdated handles the TaskUpdatedMsg.
func (m *AppModel) handleTaskUpdated(msg TaskUpdatedMsg) (bool, tea.Cmd) {
	// Coordinator path: the edit committed one atomic ChangeSet through the
	// application layer, so reload the committed project from disk before the
	// shared refresh runs.
	if msg.Result != nil {
		m.reloadProjectByID(msg.Result.ProjectID)
	}

	// Common pattern: pop view, reload, refresh
	refreshCmd := m.handleWithRefresh(true, true, "")

	// Re-resolve the live task pointer AFTER handleWithRefresh, which reloads the
	// current project and replaces every task pointer. Resolving earlier would
	// hand delegated views an orphaned pointer.
	if msg.Result != nil {
		project := m.currentProject
		if project == nil || project.ID != msg.Result.ProjectID {
			project = m.projectByID(msg.Result.ProjectID)
		}
		if project != nil {
			msg.Task = project.GetTaskByID(msg.Result.TaskID)
		}
	}

	// Delegate to the current view so it can reload its state
	cmd := m.views.Update(m.currentView(), msg)
	return true, tea.Batch(refreshCmd, cmd)
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
		return performUndoForProjectsCmd(m.persister, project, activeUndoProjectsFromRuntime(m.runtime), m.repairTaskProjectRefsAfterRestore)
	}
	return true, tea.Batch(refreshCmd, m.setUndoableStatus(toast, undoCb))
}
