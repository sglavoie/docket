package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	// Delegate dependency messages to dedicated handler
	if handled, cmd := m.handleDependencyMessages(msg); handled {
		return handled, cmd
	}

	switch msg := msg.(type) {
	case CreateTaskMsg:
		if m.currentProject != nil {
			// Check if there are enabled templates (unless explicitly skipped)
			if !msg.SkipTemplatePicker {
				enabledTemplates := m.config.GetEnabledTaskTemplates()
				if len(enabledTemplates) > 0 {
					// Store the parent task and show template picker
					m.pendingCreateTaskParent = msg.ParentTask
					picker := NewTemplatePickerModel(m.config, m.currentProject)
					m.showView(ViewTaskTemplatePicker, picker)
					return true, picker.Init()
				}
			}
			// No templates or skipped, go directly to task creator
			m.showView(ViewTaskCreator, NewTaskCreatorModel(m.currentProject, m.config, m.configManager, msg.ParentTask))
		}
		return true, nil

	case EditTaskMsg:
		if m.currentProject != nil && msg.Task != nil {
			m.showView(ViewTaskEditor, NewTaskEditorModel(m.currentProject, msg.Task, m.config, m.configManager))
		}
		return true, nil

	case TaskCreatedMsg:
		// Common pattern: pop view, reload, refresh
		m.handleWithRefresh(true, true, "")
		// Set status message showing section
		if msg.Task != nil && msg.Task.Section != "" {
			m.statusMessage = fmt.Sprintf("Created in %s", msg.Task.Section)
		}
		// Delegate to current view (e.g., inbox processor) so it can reload its state
		cmd := m.views.Update(m.currentView(), msg)
		m.syncInboxProcessor()
		return true, tea.Batch(cmd, statusTimeout(2*time.Second))

	case TaskUpdatedMsg:
		// Common pattern: pop view, reload, refresh
		m.handleWithRefresh(true, true, "")
		// Delegate to current view (e.g., inbox processor) so it can reload its state
		cmd := m.views.Update(m.currentView(), msg)
		m.syncInboxProcessor()
		return true, cmd

	case TaskDeletedMsg:
		return true, m.handleWithRefresh(false, false, "")

	case ArchiveCompletedTasksMsg:
		return m.handleArchiveCompletedTasks()

	case ArchiveAllCompletedTasksMsg:
		return m.handleArchiveAllCompletedTasks()

	case TasksArchivedMsg:
		statusMsg := fmt.Sprintf("Archived %d task(s) to completed.md", msg.Count)
		if msg.Projects > 0 {
			statusMsg = fmt.Sprintf("Archived %d task(s) across %d project(s)", msg.Count, msg.Projects)
		}
		return true, m.handleWithRefresh(false, false, statusMsg)

	case TaskStatusChangedMsg:
		return true, m.handleWithRefresh(false, false, "")

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

	case SetSnoozeDateMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewSnoozeInput, NewSnoozeInputModel(msg.Task, msg.Project))
		}
		return true, nil

	case SnoozeDateChangedMsg:
		m.popView()
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
					if item.Project.MutateTask(item.Task.ID, func(t *models.Task) {
						if newDate != nil {
							dateCopy := *newDate
							t.DueDate = &dateCopy
						} else {
							t.DueDate = nil
						}
					}) {
						projectMap[item.Project.ID] = item.Project
						count++
					}
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
			return true, defaultStatusTimeout()
		}

		m.pendingBulkTasks = nil
		m.refreshProjectTree()
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
		return true, m.handleWithRefresh(true, false, fmt.Sprintf("Undid: %s", msg.Action))

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
		return true, m.handleWithRefresh(false, false, "")

	case ShowTaskCommentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewTaskComments, NewTaskCommentsModelWithEditor(msg.Task, msg.Project, m.config.Editor))
		}
		return true, nil

	case CloseTaskCommentsMsg:
		m.popView()
		return true, nil

	case ShowTaskChecklistMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewTaskChecklist, NewTaskChecklistModelWithEditor(msg.Task, msg.Project, m.config.Editor))
		}
		return true, nil

	case CloseTaskChecklistMsg:
		m.popView()
		return true, nil

	case ChecklistItemAddedMsg:
		m.statusMessage = "Checklist item added"
		return true, defaultStatusTimeout()

	case ChecklistItemDeletedMsg:
		m.statusMessage = "Checklist item deleted"
		return true, defaultStatusTimeout()

	case ChecklistItemEditedMsg:
		m.statusMessage = "Checklist item updated"
		return true, defaultStatusTimeout()

	case ChecklistItemToggledMsg:
		return true, nil

	case OpenTaskActivityMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewTaskActivity, NewTaskActivityModel(msg.Task, msg.Project))
		}
		return true, nil

	case CloseTaskActivityMsg:
		m.popView()
		return true, nil

	case CommentAddedMsg:
		m.statusMessage = "Comment added"
		return true, defaultStatusTimeout()

	case CommentDeletedMsg:
		m.statusMessage = "Comment deleted"
		return true, defaultStatusTimeout()

	case CommentEditedMsg:
		m.statusMessage = "Comment updated"
		return true, defaultStatusTimeout()

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

	case CreateLinkedTaskMsg:
		if msg.SourceTask != nil && msg.Project != nil {
			m.showView(ViewLinkedTaskCreator, NewLinkedTaskCreatorModel(msg.SourceTask, msg.Project, msg.LinkType))
		}
		return true, nil

	case LinkedTaskCreatedMsg:
		m.popView()
		m.refreshProjectTree()

		// Determine link type name for status message
		var linkName string
		switch msg.LinkType {
		case LinkTypeBlocker:
			linkName = "blocker"
		case LinkTypeDependent:
			linkName = "dependent"
		case LinkTypeSubtask:
			linkName = "subtask"
		case LinkTypeFollowUp:
			linkName = "follow-up"
		}
		m.statusMessage = fmt.Sprintf("Created %s #%d", linkName, msg.NewTask.ShortID)

		// Jump to new task if configured
		if m.config.GetLinkedTaskJumpToNew() && m.projectTree != nil && msg.NewTask != nil {
			m.projectTree.JumpToTask(msg.NewTask)
		}

		return true, defaultStatusTimeout()

	case ShowLinkedTaskSelectorMsg:
		if msg.Task != nil && msg.Project != nil {
			selector := NewLinkedTaskSelectorModelWithConfig(msg.Task, msg.Project, m.config)
			// If no linked tasks, do nothing
			if !selector.HasLinkedTasks() {
				m.statusMessage = "No linked tasks"
				return true, defaultStatusTimeout()
			}
			// If single linked task with same project, jump directly (unless unlinking)
			if !msg.ForUnlink {
				if singleTask := selector.GetSingleLinkedTask(); singleTask != nil {
					return m.handleLinkedTaskNavigation(singleTask, msg.Project)
				}
			}
			// Multiple linked tasks, cross-project, or unlinking: show selector
			m.showView(ViewLinkedTaskSelector, selector)
		}
		return true, nil

	case CloseLinkedTaskSelectorMsg:
		m.popView()
		return true, nil

	case LinkedTaskSelectedMsg:
		m.popView()
		// Check if this is a cross-project reference
		if msg.TaskRef != "" {
			return m.handleCrossProjectNavigation(msg.TaskRef)
		}
		return m.handleLinkedTaskNavigation(msg.Task, msg.Project)

	case ShowCrossProjectLinkerMsg:
		if msg.SourceTask != nil && msg.SourceProject != nil {
			linker := NewCrossProjectLinkerModel(msg.SourceTask, msg.SourceProject, m.config)
			m.showView(ViewCrossProjectLinker, linker)
		}
		return true, nil

	case CloseCrossProjectLinkerMsg:
		m.popView()
		return true, nil

	case CrossProjectLinkCreatedMsg:
		m.popView()
		m.refreshProjectTree()
		// Determine link type name for status message
		var linkName string
		switch msg.LinkType {
		case LinkTypeBlocker:
			linkName = "blocker"
		case LinkTypeDependent:
			linkName = "dependency"
		case LinkTypeFollowUp:
			linkName = "related"
		}
		m.statusMessage = fmt.Sprintf("Linked #%d → %s #%d (%s)", msg.SourceTask.ShortID, msg.TargetProject.Name, msg.TargetTask.ShortID, linkName)
		return true, statusTimeout(3 * time.Second)

	case LinkRemovedMsg:
		// Save task ID before refresh (project reload creates new task objects)
		sourceTaskID := msg.SourceTask.ID
		m.refreshProjectTree()
		// Adjust cursors after link removal
		if m.projectTree != nil && m.currentProject != nil {
			// Find the task again after refresh (refresh reloads project)
			task := m.currentProject.GetTaskByID(sourceTaskID)
			if task != nil {
				// Restore main tree cursor to the source task
				m.projectTree.JumpToTask(task)

				sections := m.projectTree.getAvailableSections(task)
				sidebar := m.projectTree.sidebar

				if len(sections) == 0 {
					// No links remaining, unfocus sidebar
					sidebar.SetFocused(false)
				} else {
					// Check if current section still has items
					currentSection := sidebar.GetDependencySection()
					sectionHasItems := false
					for _, s := range sections {
						if s == currentSection {
							sectionHasItems = true
							break
						}
					}
					if !sectionHasItems {
						// Switch to first available section
						sidebar.SetDependencySection(sections[0])
					}
					// Clamp cursor to valid range
					count := m.projectTree.getCurrentSidebarLinkedItemCount(task)
					if sidebar.GetDependencyCursor() >= count {
						if count > 0 {
							sidebar.SetDependencyCursor(count - 1)
						} else {
							sidebar.SetDependencyCursor(0)
						}
					}
				}
			}
		}
		// Determine section name for status message
		var sectionName string
		switch msg.Section {
		case 0:
			sectionName = "blocker"
		case 1:
			sectionName = "dependent"
		case 2:
			sectionName = "related"
		}
		m.statusMessage = fmt.Sprintf("Removed %s link", sectionName)
		return true, defaultStatusTimeout()
	}
	return false, nil
}

// handleCrossProjectNavigation navigates to a task in another project using a qualified ref.
func (m *AppModel) handleCrossProjectNavigation(taskRef string) (bool, tea.Cmd) {
	ref := models.ParseTaskRef(taskRef)
	if ref.IsEmpty() {
		return true, errorCmd("invalid task reference")
	}

	// Get target project
	targetProject := m.config.GetProject(ref.ProjectID)
	if targetProject == nil {
		m.statusMessage = "Project not found (link may be broken)"
		return true, defaultStatusTimeout()
	}

	// Load the project
	if err := m.persister.Load(targetProject); err != nil {
		return true, errorCmd("failed to load project: %w", err)
	}

	// Find the task
	task := targetProject.GetTaskByID(ref.TaskID)
	if task == nil {
		m.statusMessage = fmt.Sprintf("Task not found in %s (link may be broken)", targetProject.Name)
		return true, defaultStatusTimeout()
	}

	return m.handleLinkedTaskNavigation(task, targetProject)
}

// handleLinkedTaskNavigation navigates to a linked task, switching projects if needed.
func (m *AppModel) handleLinkedTaskNavigation(task *models.Task, project *models.Project) (bool, tea.Cmd) {
	// If we're in the project tree for this project, just jump to the task
	if m.currentView() == ViewProjectTree && m.currentProject != nil && m.currentProject.ID == project.ID {
		if m.projectTree != nil {
			m.projectTree.JumpToTask(task)
			m.statusMessage = fmt.Sprintf("Jumped to #%d", task.ShortID)
		}
		return true, defaultStatusTimeout()
	}

	// Otherwise, switch to the project and jump
	m.switchToProject(project, true)
	m.pushView(ViewProjectTree)
	if m.projectTree != nil {
		m.projectTree.JumpToTask(task)
		m.statusMessage = fmt.Sprintf("Jumped to #%d in %s", task.ShortID, project.Name)
	}
	return true, defaultStatusTimeout()
}

// handleCommandPaletteTaskAction handles inline task actions from the command palette.
// It applies status changes (complete, start, reopen) and saves the project.
func (m *AppModel) handleCommandPaletteTaskAction(msg CommandPaletteTaskActionMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}

	current := msg.Project.GetTaskByID(msg.Task.ID)
	if current == nil {
		return true, errorCmd("task no longer exists in project")
	}

	// Apply the status change
	switch msg.Action {
	case "complete":
		current.Complete()
	case "start":
		current.Start()
	case "block":
		current.Block()
	case "cancel":
		current.Cancel()
	case "reopen":
		current.Reopen()
	default:
		return true, nil
	}

	// Async save with loading indicator
	successMsg := fmt.Sprintf("Task %s: %s", msg.Action, current.Title)
	return true, tea.Batch(
		startLoading("Saving..."),
		SaveProjectAsync(msg.Project, successMsg),
	)
}

// handleInlineTaskCreate creates a new task from the command palette inline creation.
func (m *AppModel) handleInlineTaskCreate(msg InlineTaskCreateMsg) (bool, tea.Cmd) {
	if msg.Project == nil || msg.Title == "" {
		return true, nil
	}

	// Ensure project is loaded
	if err := m.persister.Load(msg.Project); err != nil {
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
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	editor := external.ResolveEditor(m.config.Editor)
	c := external.EditorCommand(editor, tmpPath)

	task := msg.Task
	project := msg.Project
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return TaskContentEditedMsg{Task: task, Project: project, Err: err}
		}
		content, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
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
			if msg.Project != nil {
				// Re-lookup task in project to avoid stale pointer after concurrent reload
				if !msg.Project.MutateTask(msg.Task.ID, func(t *models.Task) {
					t.Title = msg.NewTitle
					t.Description = msg.NewDescription
				}) {
					return true, errorCmd("task no longer exists in project")
				}

				// Refresh task editor view first if it's open
				if m.currentView() == ViewTaskEditor {
					current := msg.Project.GetTaskByID(msg.Task.ID)
					taskEditor := NewTaskEditorModel(msg.Project, current, m.config, m.configManager)
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
		current := msg.Project.GetTaskByID(msg.Task.ID)
		if current != nil {
			taskEditor := NewTaskEditorModel(msg.Project, current, m.config, m.configManager)
			taskEditor.SetSize(m.width, m.height-2)
			m.views.Register(ViewTaskEditor, taskEditor)
		}
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
	if err := m.persister.Load(destProject); err != nil {
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
	persister := m.persister
	return func() tea.Msg {
		// Backup and save source project
		if err := persister.Backup(source); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to backup source project: %w", err)}
		}
		if err := persister.Save(source); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to save source project: %w", err)}
		}

		// Backup and save destination project
		if err := persister.Backup(dest); err != nil {
			return SaveErrorMsg{Err: fmt.Errorf("failed to backup destination project: %w", err)}
		}
		if err := persister.Save(dest); err != nil {
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

// handleArchiveCompletedTasks archives completed tasks from the current project.
func (m *AppModel) handleArchiveCompletedTasks() (bool, tea.Cmd) {
	if m.currentProject == nil {
		return true, nil
	}

	project := m.currentProject
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	completedCount := project.CountCompletedTasks()

	cmd := func() tea.Msg {
		if completedCount == 0 {
			return TasksArchivedMsg{Count: 0}
		}

		// Record undo snapshot before archiving
		if undoDepth > 0 {
			_ = m.persister.RecordUndoSnapshot(project, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth)
		}

		// Create backup before archiving
		if err := m.persister.Backup(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		// Archive the tasks
		count, err := m.persister.ArchiveTasks(project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}

		// Reload project
		_ = m.persister.Load(project)

		return TasksArchivedMsg{Count: count}
	}

	return true, cmd
}

// handleArchiveAllCompletedTasks archives completed tasks across all active projects.
func (m *AppModel) handleArchiveAllCompletedTasks() (bool, tea.Cmd) {
	projects := m.config.GetActiveProjects()

	cmd := func() tea.Msg {
		totalArchived := 0
		projectsAffected := 0

		for _, project := range projects {
			if err := m.persister.Load(project); err != nil {
				continue
			}

			if project.CountCompletedTasks() == 0 {
				continue
			}

			if err := m.persister.Backup(project); err != nil {
				continue
			}

			count, err := m.persister.ArchiveTasks(project)
			if err != nil {
				continue
			}

			totalArchived += count
			projectsAffected++
		}

		// Reload current project if it was affected
		if m.currentProject != nil {
			_ = m.persister.Load(m.currentProject)
		}

		return TasksArchivedMsg{Count: totalArchived, Projects: projectsAffected}
	}

	return true, cmd
}
