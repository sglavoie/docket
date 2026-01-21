package tui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CreateTaskMsg:
		if m.currentProject != nil {
			m.showView(ViewTaskCreator, NewTaskCreatorModel(m.currentProject, m.config, m.configManager, msg.ParentTask))
		}
		return true, nil

	case CreateNoteMsg:
		if m.currentProject != nil {
			m.showView(ViewNoteCreator, NewNoteCreatorModel(m.currentProject, m.config, m.configManager, msg.ParentTask))
		}
		return true, nil

	case EditTaskMsg:
		if m.currentProject != nil && msg.Task != nil {
			m.showView(ViewTaskEditor, NewTaskEditorModel(m.currentProject, msg.Task, m.config, m.configManager))
		}
		return true, nil

	case TaskCreatedMsg, TaskUpdatedMsg:
		m.popView()
		// Reload current project to get fresh task references after save
		if m.currentProject != nil {
			storage.LoadProject(m.currentProject)
		}
		m.refreshProjectTree()
		// Delegate to current view (e.g., inbox processor) so it can reload its state
		cmd := m.views.Update(m.currentView(), msg)
		// Keep typed pointer in sync if it's the inbox processor
		if m.currentView() == ViewInboxProcessor {
			if v := m.views.Get(ViewInboxProcessor); v != nil {
				m.inboxProcessor = v.(*InboxProcessorModel)
			}
		}
		return true, cmd

	case TaskDeletedMsg:
		m.refreshProjectTree()
		return true, nil

	case TaskStatusChangedMsg:
		m.refreshProjectTree()
		return true, nil

	case CommandPaletteTaskActionMsg:
		return m.handleCommandPaletteTaskAction(msg)

	case CommandPaletteBulkActionMsg:
		return m.handleCommandPaletteBulkAction(msg)

	case InlineTaskCreateMsg:
		return m.handleInlineTaskCreate(msg)

	case EditTaskContentMsg:
		return m.handleEditTaskContent(msg)

	case TaskContentEditedMsg:
		return m.handleTaskContentEdited(msg)

	case SetDueDateMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDateInput, NewDateInputModel(msg.Task, msg.Project))
		}
		return true, nil

	case DueDateChangedMsg:
		m.popView()

		// Check if this is a bulk operation from palette
		if len(m.pendingBulkTasks) > 1 && msg.Task != nil {
			// The first task was already saved by DateInputModel
			// Apply the same date to remaining tasks
			newDate := msg.Task.DueDate
			count := 1 // First task already updated
			projectMap := make(map[string]*models.Project)

			for i, item := range m.pendingBulkTasks {
				if i == 0 {
					// Skip the first task (already saved by DateInputModel)
					projectMap[item.Project.ID] = item.Project
					continue
				}
				if item.Task != nil && item.Project != nil {
					if newDate != nil {
						dateCopy := *newDate
						item.Task.DueDate = &dateCopy
					} else {
						item.Task.DueDate = nil
					}
					item.Task.UpdatedAt = time.Now()
					projectMap[item.Project.ID] = item.Project
					count++
				}
			}

			// Build list of projects to save (except first which was already saved)
			var projectsToSave []*models.Project
			for _, project := range projectMap {
				if project.ID != m.pendingBulkTasks[0].Project.ID {
					projectsToSave = append(projectsToSave, project)
				}
			}

			var successMsg string
			if newDate != nil {
				successMsg = fmt.Sprintf("Set due date to %s for %d tasks", newDate.Format("2006-01-02"), count)
			} else {
				successMsg = fmt.Sprintf("Cleared due date for %d tasks", count)
			}
			m.pendingBulkTasks = nil

			if len(projectsToSave) > 0 {
				return true, tea.Batch(
					startLoading("Saving..."),
					SaveMultipleProjectsAsync(projectsToSave, successMsg),
				)
			}
			m.statusMessage = successMsg
			m.refreshProjectTree()
			return true, statusTimeout(2 * time.Second)
		}

		m.pendingBulkTasks = nil
		m.refreshProjectTree()
		return true, nil

	case ManageDependenciesMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencySelector, NewDependencySelectorModel(msg.Task, msg.Project))
		}
		return true, nil

	case DependencySelectorDoneMsg:
		m.popView()
		if msg.HasChanges && msg.Task != nil && msg.Project != nil {
			msg.Task.DependsOn = msg.NewDepsOn
			msg.Task.UpdatedAt = time.Now()
			return true, tea.Batch(
				startLoading("Saving..."),
				SaveProjectAsync(msg.Project, "Dependencies updated"),
			)
		}
		return true, nil

	case ShowDependencyViewerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencyViewer, NewDependencyViewerModel(msg.Task, msg.Project))
		}
		return true, nil

	case CloseDependencyViewerMsg:
		m.popView()
		return true, nil

	case DependencyViewerSelectMsg:
		m.popView()
		if msg.Task != nil && m.projectTree != nil {
			// If the task is completed, switch to Completed view so it's visible
			if msg.Task.IsComplete() && m.projectTree.viewMode != ViewModeCompleted {
				m.projectTree.viewMode = ViewModeCompleted
				m.projectTree.updateVisibility()
			}
			m.projectTree.NavigateToTask(msg.Task)
		}
		return true, nil

	case DependencyViewerViewDepsMsg:
		// Replace current view with new dependency viewer for selected task
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencyViewer, NewDependencyViewerModel(msg.Task, msg.Project))
		}
		return true, nil

	case ShowUndoHistoryMsg:
		if msg.Project != nil {
			m.showView(ViewUndoHistory, NewUndoHistoryModel(msg.Project))
		}
		return true, nil

	case CloseUndoHistoryMsg:
		m.popView()
		return true, nil

	case UndoPerformedMsg:
		// Pop the undo history view, refresh project tree, show status
		m.popView()
		m.refreshProjectTree()
		m.statusMessage = fmt.Sprintf("Undid: %s", msg.Action)
		return true, nil

	case OpenParentSelectorMsg:
		selector := NewParentSelectorModel(msg.ValidParents, msg.CurrentID, msg.AllowNone, msg.Source)
		m.showView(ViewParentSelector, selector)
		return true, selector.Init()

	case CloseParentSelectorMsg:
		m.popView()
		return true, nil

	case ParentSelectedMsg:
		// Pop the parent selector view
		m.popView()
		// Forward the message directly to the underlying view (note/task creator)
		return true, m.delegateToCurrentView(msg)

	case UndoCompletedMsg:
		m.refreshProjectTree()
		return true, nil

	case ShowTaskCommentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewTaskComments, NewTaskCommentsModelWithEditor(msg.Task, msg.Project, m.config.Editor))
		}
		return true, nil

	case CloseTaskCommentsMsg:
		m.popView()
		return true, nil

	case CommentAddedMsg:
		m.statusMessage = "Comment added"
		return true, statusTimeout(2 * time.Second)

	case CommentDeletedMsg:
		m.statusMessage = "Comment deleted"
		return true, statusTimeout(2 * time.Second)

	case CommentEditedMsg:
		m.statusMessage = "Comment updated"
		return true, statusTimeout(2 * time.Second)

	case ShowProjectSelectorForMoveMsg:
		if msg.Task != nil && msg.SourceProject != nil {
			projects := m.config.GetActiveProjects()
			m.showView(ViewProjectSelector, NewProjectSelectorModel(projects, msg.Task, msg.SourceProject))
		}
		return true, nil

	case CloseProjectSelectorMsg:
		m.popView()
		return true, nil

	case MoveTaskToProjectMsg:
		return m.handleMoveTaskToProject(msg)

	case TaskMovedToProjectMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.popView() // Close project selector
		m.refreshProjectTree()
		var statusMsg string
		if msg.MovedCount > 1 {
			statusMsg = fmt.Sprintf("Moved #%d and %d descendants to %s", msg.OldShortID, msg.MovedCount-1, msg.DestProject.Name)
		} else {
			statusMsg = fmt.Sprintf("Moved #%d to %s", msg.OldShortID, msg.DestProject.Name)
		}
		if msg.ClearedDeps > 0 {
			statusMsg += fmt.Sprintf(" (%d deps cleared)", msg.ClearedDeps)
		}
		m.statusMessage = statusMsg
		return true, statusTimeout(3 * time.Second)
	}
	return false, nil
}

// handleCommandPaletteTaskAction handles inline task actions from the command palette.
// It applies status changes (complete, start, reopen) and saves the project.
func (m *AppModel) handleCommandPaletteTaskAction(msg CommandPaletteTaskActionMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}

	// Apply the status change
	switch msg.Action {
	case "complete":
		msg.Task.Complete()
	case "start":
		msg.Task.Start()
	case "block":
		msg.Task.Block()
	case "cancel":
		msg.Task.Cancel()
	case "reopen":
		msg.Task.Reopen()
	default:
		return true, nil
	}

	// Async save with loading indicator
	successMsg := fmt.Sprintf("Task %s: %s", msg.Action, msg.Task.Title)
	return true, tea.Batch(
		startLoading("Saving..."),
		SaveProjectAsync(msg.Project, successMsg),
	)
}

// handleCommandPaletteBulkAction handles bulk actions on multiple selected tasks.
func (m *AppModel) handleCommandPaletteBulkAction(msg CommandPaletteBulkActionMsg) (bool, tea.Cmd) {
	if len(msg.Tasks) == 0 {
		return true, nil
	}

	// Group tasks by project for efficient saving
	projectTasks := make(map[string][]struct {
		Task    *models.Task
		Project *models.Project
	})
	for _, item := range msg.Tasks {
		if item.Task != nil && item.Project != nil {
			projectTasks[item.Project.ID] = append(projectTasks[item.Project.ID], item)
		}
	}

	// Apply action to all tasks
	count := 0
	var projects []*models.Project
	for _, items := range projectTasks {
		for _, item := range items {
			switch msg.Action {
			case "complete":
				item.Task.Complete()
				count++
			case "start":
				item.Task.Start()
				count++
			}
		}
		if len(items) > 0 {
			projects = append(projects, items[0].Project)
		}
	}

	// Build success message
	actionPast := msg.Action + "d"
	if msg.Action == "complete" {
		actionPast = "completed"
	} else if msg.Action == "start" {
		actionPast = "started"
	}
	successMsg := fmt.Sprintf("%d tasks %s", count, actionPast)

	// Async save with loading indicator
	return true, tea.Batch(
		startLoading("Saving..."),
		SaveMultipleProjectsAsync(projects, successMsg),
	)
}

// handleInlineTaskCreate creates a new task from the command palette inline creation.
func (m *AppModel) handleInlineTaskCreate(msg InlineTaskCreateMsg) (bool, tea.Cmd) {
	if msg.Project == nil || msg.Title == "" {
		return true, nil
	}

	// Ensure project is loaded
	if err := storage.LoadProject(msg.Project); err != nil {
		return true, errorCmd("failed to load project: %w", err)
	}

	// Ensure Inbox section exists
	if !msg.Project.HasSection(models.SectionInbox) {
		msg.Project.EnsureSection(models.SectionInbox)
		msg.Project.SetSectionColor(models.SectionInbox, "red")
	}

	// Create the task
	task := &models.Task{
		ID:        uuid.New().String(),
		ShortID:   msg.Project.GetNextShortID(),
		Type:      models.TaskTypeTask,
		Title:     msg.Title,
		Status:    models.StatusTodo,
		Priority:  msg.Priority,
		Section:   models.SectionInbox,
		DueDate:   msg.DueDate,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add tags
	for _, tag := range msg.Tags {
		task.AddTag(tag)
	}

	// Add to project at the beginning so it's prominently visible
	msg.Project.Tasks = append([]*models.Task{task}, msg.Project.Tasks...)
	msg.Project.BuildTaskTree()

	// Async save with loading indicator
	successMsg := fmt.Sprintf("Created: %s", msg.Title)
	return true, tea.Batch(
		startLoading("Saving..."),
		SaveProjectAsync(msg.Project, successMsg),
	)
}

// handleEditTaskContent launches external editor for task title and description.
func (m *AppModel) handleEditTaskContent(msg EditTaskContentMsg) (bool, tea.Cmd) {
	if msg.Task == nil {
		return true, nil
	}

	tmpFile, err := os.CreateTemp("", "tt-edit-*.md")
	if err != nil {
		return true, errorCmd("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	content := fmt.Sprintf("# %s\n\n<!-- Edit the title above and description below. -->\n\n%s", msg.Task.Title, msg.Task.Description)
	tmpFile.WriteString(content)
	tmpFile.Close()

	editor := m.config.Editor
	if editor == "" {
		editor = "code"
	}

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	task := msg.Task
	project := msg.Project
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmpPath)
			return TaskContentEditedMsg{Task: task, Project: project, Err: err}
		}
		content, readErr := os.ReadFile(tmpPath)
		os.Remove(tmpPath)
		if readErr != nil {
			return TaskContentEditedMsg{Task: task, Project: project, Err: readErr}
		}
		newTitle, newDesc := parseTaskContent(string(content), task)
		return TaskContentEditedMsg{
			Task:           task,
			Project:        project,
			NewTitle:       newTitle,
			NewDescription: newDesc,
		}
	})
}

// handleTaskContentEdited handles result from external editor.
func (m *AppModel) handleTaskContentEdited(msg TaskContentEditedMsg) (bool, tea.Cmd) {
	if msg.Err != nil {
		return true, errorCmd("editor failed: %w", msg.Err)
	}

	if msg.Task != nil {
		titleChanged := msg.NewTitle != msg.Task.Title
		descChanged := msg.NewDescription != msg.Task.Description
		if titleChanged || descChanged {
			msg.Task.Title = msg.NewTitle
			msg.Task.Description = msg.NewDescription
			msg.Task.UpdatedAt = time.Now()

			if msg.Project != nil {
				// Refresh task editor view first if it's open
				if m.currentView() == ViewTaskEditor {
					taskEditor := NewTaskEditorModel(msg.Project, msg.Task, m.config, m.configManager)
					taskEditor.SetSize(m.width, m.height-2)
					m.views.Register(ViewTaskEditor, taskEditor)
				}
				return true, tea.Batch(
					startLoading("Saving..."),
					SaveProjectAsync(msg.Project, "Task updated"),
				)
			}
		}
	}

	if m.currentView() == ViewTaskEditor && msg.Task != nil {
		taskEditor := NewTaskEditorModel(msg.Project, msg.Task, m.config, m.configManager)
		taskEditor.SetSize(m.width, m.height-2)
		m.views.Register(ViewTaskEditor, taskEditor)
	}
	return true, nil
}

// handleMoveTaskToProject handles moving a task to another project.
func (m *AppModel) handleMoveTaskToProject(msg MoveTaskToProjectMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.SourceProject == nil || msg.DestProjectID == "" {
		return true, nil
	}

	// Get destination project
	destProject := m.config.GetProject(msg.DestProjectID)
	if destProject == nil {
		return true, errorCmd("destination project not found")
	}

	// Load destination project
	if err := storage.LoadProject(destProject); err != nil {
		return true, errorCmd("failed to load destination project: %w", err)
	}

	// Store old short ID before move
	oldShortID := msg.Task.ShortID

	// Perform the move
	result, err := msg.SourceProject.MoveTaskToProject(msg.Task, destProject)
	if err != nil {
		return true, errorCmd("failed to move task: %w", err)
	}
	if result == nil {
		return true, nil
	}

	// Save both projects
	return true, tea.Batch(
		startLoading("Moving task..."),
		m.saveBothProjectsAsync(msg.SourceProject, destProject, result, oldShortID),
	)
}

// saveBothProjectsAsync saves both source and destination projects after a cross-project move.
func (m *AppModel) saveBothProjectsAsync(source, dest *models.Project, result *models.MoveTaskToProjectResult, oldShortID int) tea.Cmd {
	return func() tea.Msg {
		// Backup and save source project
		if err := storage.BackupProject(source); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to backup source project: %w", err)}
		}
		if err := storage.SaveProject(source); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to save source project: %w", err)}
		}

		// Backup and save destination project
		if err := storage.BackupProject(dest); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to backup destination project: %w", err)}
		}
		if err := storage.SaveProject(dest); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to save destination project: %w", err)}
		}

		// Return success message with move details
		var movedTask *models.Task
		if len(result.MovedTasks) > 0 {
			movedTask = result.MovedTasks[0]
		}

		return TaskMovedToProjectMsg{
			Task:          movedTask,
			SourceProject: source,
			DestProject:   dest,
			MovedCount:    len(result.MovedTasks),
			ClearedDeps:   len(result.ClearedDeps),
			OldShortID:    oldShortID,
		}
	}
}
