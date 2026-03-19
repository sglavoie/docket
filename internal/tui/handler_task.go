package tui

import (
	"fmt"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
		project := msg.Project
		if project == nil {
			project = m.currentProject
		}
		if project != nil && msg.Task != nil {
			m.showView(ViewTaskEditor, NewTaskEditorModel(project, msg.Task, m.config, m.configManager))
		}
		return true, nil

	case TaskCreatedMsg:
		// Common pattern: pop view, reload, refresh
		m.handleWithRefresh(true, true, "")
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

		// If open-after requested, re-lookup the task (stale after reload) and open editor
		if msg.OpenContent && msg.Task != nil {
			project := msg.Project
			if project == nil {
				project = m.currentProject
			}
			if project != nil {
				if freshTask := project.GetTaskByID(msg.Task.ID); freshTask != nil {
					return true, tea.Batch(cmd, statusTimeout(2*time.Second), func() tea.Msg {
						return EditTaskContentMsg{Task: freshTask, Project: project}
					})
				}
			}
		}

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

	case TaskPropertyChangedMsg:
		m.handleWithRefresh(false, false, "")
		cmd := m.views.Update(m.currentView(), msg)
		m.syncInboxProcessor()
		return true, cmd

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

	case DescriptionEditorDoneMsg:
		return m.handleDescriptionEditorDone(msg)

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
		return m.handleDueDateChanged(msg)

	case ShowUndoHistoryMsg:
		if msg.Project != nil {
			m.showView(ViewUndoHistory, NewUndoHistoryModel(msg.Project))
		}
		return true, nil

	case CloseUndoHistoryMsg:
		m.popView()
		return true, nil

	case UndoPerformedMsg:
		// Pop the undo history view if it's on top; otherwise stay in the current view
		popView := m.currentView() == ViewUndoHistory
		undoMsg := fmt.Sprintf("Undid: %s", msg.Action)
		return true, m.handleWithRefresh(popView, false, undoMsg)

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

	case ShowTaskDetailMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskDetail, NewTaskDetailModel(msg.Task, msg.Project, m.config, m.configManager))
		}
		return true, nil

	case CloseTaskDetailMsg:
		m.popView()
		return true, nil

	case ShowTaskCommentsMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskComments, NewTaskCommentsModelWithEditor(msg.Task, msg.Project, m.config.Editor))
		}
		return true, nil

	case CloseTaskCommentsMsg:
		m.popView()
		return true, nil

	case ShowTaskChecklistMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskChecklist, NewTaskChecklistModelWithEditor(msg.Task, msg.Project, m.config.Editor))
		}
		return true, nil

	case CloseTaskChecklistMsg:
		m.popView()
		return true, nil

	case ChecklistItemAddedMsg:
		m.setSuccessStatus("Checklist item added")
		return true, defaultStatusTimeout()

	case ChecklistItemDeletedMsg:
		m.setSuccessStatus("Checklist item deleted")
		return true, defaultStatusTimeout()

	case ChecklistItemEditedMsg:
		m.setSuccessStatus("Checklist item updated")
		return true, defaultStatusTimeout()

	case ChecklistItemToggledMsg:
		return true, nil

	case OpenTaskActivityMsg:
		if msg.Task != nil && msg.Project != nil {
			m.lastNavigatedTaskID = msg.Task.ID
			m.recordTaskVisit(msg.Task, msg.Project)
			m.showView(ViewTaskActivity, NewTaskActivityModel(msg.Task, msg.Project))
		}
		return true, nil

	case CloseTaskActivityMsg:
		m.popView()
		return true, nil

	case CommentAddedMsg:
		m.setSuccessStatus("Comment added")
		return true, defaultStatusTimeout()

	case CommentDeletedMsg:
		m.setSuccessStatus("Comment deleted")
		return true, defaultStatusTimeout()

	case CommentEditedMsg:
		m.setSuccessStatus("Comment updated")
		return true, defaultStatusTimeout()

	case CommentResolvedMsg:
		if msg.Task != nil && len(msg.Task.Comments) > 0 {
			// Find the toggled comment from the comments view cursor
			if view := m.views.Get(ViewTaskComments); view != nil {
				if commentsView, ok := view.(*TaskCommentsModel); ok {
					idx := commentsView.cursor
					if idx >= 0 && idx < len(msg.Task.Comments) {
						if msg.Task.Comments[idx].Resolved {
							m.setSuccessStatus("Comment resolved")
						} else {
							m.setSuccessStatus("Comment unresolved")
						}
						return true, defaultStatusTimeout()
					}
				}
			}
		}
		m.setSuccessStatus("Comment resolved")
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
		return m.handleTaskMovedToProject(msg)

	case CreateLinkedTaskMsg:
		if msg.SourceTask != nil && msg.Project != nil {
			m.showView(ViewLinkedTaskCreator, NewLinkedTaskCreatorModel(msg.SourceTask, msg.Project, msg.LinkType))
		}
		return true, nil

	case LinkedTaskCreatedMsg:
		m.popView()
		m.refreshProjectTree()
		if m.outliner != nil {
			m.outliner.rebuildRows()
		}

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
		m.setSuccessStatus(fmt.Sprintf("Created %s #%d", linkName, msg.NewTask.ShortID))

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
				m.setInfoStatus("No linked tasks")
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
		if m.outliner != nil {
			m.outliner.rebuildRows()
		}
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
		m.setSuccessStatus(fmt.Sprintf("Linked #%d → %s #%d (%s)", msg.SourceTask.ShortID, msg.TargetProject.Name, msg.TargetTask.ShortID, linkName))
		return true, statusTimeout(3 * time.Second)

	case LinkRemovedMsg:
		return m.handleLinkRemoved(msg)
	}
	return false, nil
}

// handleDueDateChanged handles the DueDateChangedMsg, including bulk operations.
func (m *AppModel) handleDueDateChanged(msg DueDateChangedMsg) (bool, tea.Cmd) {
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
		m.setSuccessStatus(successMsg)
		m.refreshProjectTree()
		if m.outliner != nil {
			m.outliner.rebuildRows()
		}
		return true, defaultStatusTimeout()
	}

	m.pendingBulkTasks = nil
	m.refreshProjectTree()
	if m.outliner != nil {
		m.outliner.rebuildRows()
	}
	return true, nil
}

// handleLinkRemoved handles the LinkRemovedMsg and adjusts sidebar cursors.
func (m *AppModel) handleLinkRemoved(msg LinkRemovedMsg) (bool, tea.Cmd) {
	// Save task ID before refresh (project reload creates new task objects)
	sourceTaskID := msg.SourceTask.ID
	m.refreshProjectTree()
	if m.outliner != nil {
		m.outliner.rebuildRows()
	}
	// Adjust cursors after link removal
	if m.projectTree != nil && m.currentProject != nil {
		// Find the task again after refresh (refresh reloads project)
		task := m.currentProject.GetTaskByID(sourceTaskID)
		if task != nil {
			// Restore main tree cursor to the source task
			m.projectTree.JumpToTask(task)

			sections := sidebarGetAvailableSections(m.currentProject, task)
			sidebar := m.projectTree.sidebar

			if len(sections) == 0 {
				// No links remaining, unfocus sidebar
				sidebar.SetFocused(false)
			} else {
				// Check if current section still has items
				currentSection := sidebar.GetDependencySection()
				sectionHasItems := slices.Contains(sections, currentSection)
				if !sectionHasItems {
					// Switch to first available section
					sidebar.SetDependencySection(sections[0])
				}
				// Clamp cursor to valid range
				count := sidebarGetLinkedItemCount(m.currentProject, sidebar, task)
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
	// Adjust outliner sidebar cursor after link removal
	if m.outliner != nil && m.outliner.sidebar != nil && m.currentProject != nil {
		task := m.currentProject.GetTaskByID(sourceTaskID)
		if task != nil {
			sections := sidebarGetAvailableSections(m.currentProject, task)
			sidebar := m.outliner.sidebar

			if len(sections) == 0 {
				sidebar.SetFocused(false)
			} else {
				currentSection := sidebar.GetDependencySection()
				sectionHasItems := slices.Contains(sections, currentSection)
				if !sectionHasItems {
					sidebar.SetDependencySection(sections[0])
				}
				count := sidebarGetLinkedItemCount(m.currentProject, sidebar, task)
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
	m.setSuccessStatus(fmt.Sprintf("Removed %s link", sectionName))
	return true, defaultStatusTimeout()
}
