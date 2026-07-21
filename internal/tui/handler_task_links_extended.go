package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleTaskLinkMessages handles task link-related messages (linked tasks, cross-project links).
// Returns true if the message was handled.
func (m *AppModel) handleTaskLinkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CreateLinkedTaskMsg:
		return m.handleCreateLinkedTask(msg)

	case LinkedTaskCreatedMsg:
		return m.handleLinkedTaskCreated(msg)

	case LinkedTaskBatchCreatedMsg:
		return m.handleLinkedTaskBatchCreated(msg)

	case ShowLinkedTaskSelectorMsg:
		return m.handleShowLinkedTaskSelector(msg)

	case LinkedTaskSelectedMsg:
		return m.handleLinkedTaskSelected(msg)

	case ShowCrossProjectLinkerMsg:
		return m.handleShowCrossProjectLinker(msg)

	case CrossProjectLinkCreatedMsg:
		return m.handleCrossProjectLinkCreated(msg)

	case LinkRemovedMsg:
		return m.handleLinkRemoved(msg)
	}
	return false, nil
}

func (m *AppModel) handleLinkedTaskBatchCreated(msg LinkedTaskBatchCreatedMsg) (bool, tea.Cmd) {
	if len(msg.Results) == 0 {
		return true, nil
	}
	projectID := msg.Results[0].New.ProjectID
	m.reloadProjectByID(projectID)
	m.popView()
	m.refreshAllViews()
	m.sessionStats.tasksCreated += len(msg.Results)
	m.setSuccessStatus(fmt.Sprintf("Created %d linked tasks", len(msg.Results)))
	return true, defaultStatusTimeout()
}

// handleCreateLinkedTask handles the CreateLinkedTaskMsg.
func (m *AppModel) handleCreateLinkedTask(msg CreateLinkedTaskMsg) (bool, tea.Cmd) {
	if msg.SourceTask != nil && msg.Project != nil {
		if m.typedNavigation {
			preset := linkedComposerPreset(msg.Project, msg.SourceTask, msg.LinkType)
			return true, m.openTaskComposer(preset, ViewLinkedTaskCreator)
		}
		m.showView(ViewLinkedTaskCreator, NewLinkedTaskCreatorModel(msg.SourceTask, msg.Project, msg.LinkType))
	}
	return true, nil
}

// handleLinkedTaskCreated handles the LinkedTaskCreatedMsg.
func (m *AppModel) handleLinkedTaskCreated(msg LinkedTaskCreatedMsg) (bool, tea.Cmd) {
	// Coordinator path: reload the committed project and re-resolve both tasks by
	// ID so the shared jump/open/status logic runs off the reloaded pointers.
	if msg.Result != nil {
		m.reloadProjectByID(msg.Result.New.ProjectID)
		if project := m.projectByID(msg.Result.New.ProjectID); project != nil {
			msg.Project = project
			msg.NewTask = project.GetTaskByID(msg.Result.New.TaskID)
		}
	}
	m.popView()
	m.refreshAllViews()

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
	if m.runtime.Settings().GetLinkedTaskJumpToNew() && m.outliner != nil && msg.NewTask != nil {
		m.outliner.NavigateToTaskInProject(msg.NewTask, m.outliner.project)
	}

	if msg.NewTask != nil && (msg.OpenContent || msg.OpenDetail) {
		project := msg.Project
		if project == nil {
			project = m.currentProject
		}
		if project != nil {
			if freshTask := project.GetTaskByID(msg.NewTask.ID); freshTask != nil {
				var openMsg tea.Msg
				if msg.OpenDetail {
					openMsg = ShowTaskDetailMsg{Task: freshTask, Project: project}
				} else {
					openMsg = EditTaskContentMsg{Task: freshTask, Project: project}
				}
				return true, tea.Batch(statusTimeout(2*time.Second), func() tea.Msg {
					return openMsg
				})
			}
		}
	}

	return true, defaultStatusTimeout()
}

// handleShowLinkedTaskSelector handles the ShowLinkedTaskSelectorMsg.
func (m *AppModel) handleShowLinkedTaskSelector(msg ShowLinkedTaskSelectorMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		selector := NewLinkedTaskSelectorModelWithConfig(msg.Task, msg.Project, m.runtime, m.secondaryFeatures.Link)
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
}

// handleLinkedTaskSelected handles the LinkedTaskSelectedMsg.
func (m *AppModel) handleLinkedTaskSelected(msg LinkedTaskSelectedMsg) (bool, tea.Cmd) {
	m.popView()
	// Check if this is a cross-project reference
	if msg.TaskRef != "" {
		return m.handleCrossProjectNavigation(msg.TaskRef)
	}
	return m.handleLinkedTaskNavigation(msg.Task, msg.Project)
}

// handleShowCrossProjectLinker handles the ShowCrossProjectLinkerMsg.
func (m *AppModel) handleShowCrossProjectLinker(msg ShowCrossProjectLinkerMsg) (bool, tea.Cmd) {
	if msg.SourceTask != nil && msg.SourceProject != nil {
		linker := NewCrossProjectLinkerModel(msg.SourceTask, msg.SourceProject, m.runtime, m.secondaryFeatures.Link)
		m.showView(ViewCrossProjectLinker, linker)
	}
	return true, nil
}

// handleCrossProjectLinkCreated handles the CrossProjectLinkCreatedMsg.
func (m *AppModel) handleCrossProjectLinkCreated(msg CrossProjectLinkCreatedMsg) (bool, tea.Cmd) {
	m.popView()
	m.refreshAllViews()
	// Determine link type name for status message
	var linkName string
	switch msg.LinkType {
	case LinkTypeBlocker:
		linkName = "blocker"
	case LinkTypeDependent:
		linkName = "dependency"
	case LinkTypeFollowUp:
		linkName = "related"
	default:
	}
	m.setSuccessStatus(fmt.Sprintf("Linked #%d → %s #%d (%s)", msg.SourceTask.ShortID, msg.TargetProject.Name, msg.TargetTask.ShortID, linkName))
	return true, statusTimeout(3 * time.Second)
}

// handleLinkRemoved handles the LinkRemovedMsg and adjusts sidebar cursors.
func (m *AppModel) handleLinkRemoved(msg LinkRemovedMsg) (bool, tea.Cmd) {
	// Save task ID before refresh (project reload creates new task objects)
	sourceTaskID := msg.SourceTask.ID
	// The removal committed through the application layer against its own fresh
	// snapshot, so reload the touched projects' in-memory copies before adjusting
	// cursors (a cross-project related unlink touches both sides).
	for _, projectID := range msg.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}
	m.refreshAllViews()
	// Adjust cursors after link removal
	if m.currentProject != nil {
		task := m.currentProject.GetTaskByID(sourceTaskID)
		if task != nil {
			if m.outliner != nil {
				m.outliner.NavigateToTaskInProject(task, m.outliner.project)
				if m.outliner.sidebar != nil {
					sidebarAdjustAfterLinkRemoval(m.currentProject, m.outliner.sidebar, task)
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
