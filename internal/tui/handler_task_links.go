package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleCrossProjectNavigation navigates to a task in another project using a qualified ref.
func (m *AppModel) handleCrossProjectNavigation(taskRef string) (bool, tea.Cmd) {
	ref := models.ParseTaskRef(taskRef)
	if ref.IsEmpty() {
		return true, errorCmd("invalid task reference")
	}

	// Get target project
	targetProject := m.projectByID(ref.ProjectID)
	if targetProject == nil {
		m.setInfoStatus("Project not found (link may be broken)")
		return true, defaultStatusTimeout()
	}

	// Load the project
	if err := m.persister.Load(targetProject); err != nil {
		return true, errorCmd("failed to load project: %w", err)
	}

	// Find the task
	task := targetProject.GetTaskByID(ref.TaskID)
	if task == nil {
		m.setInfoStatus(fmt.Sprintf("Task not found in %s (link may be broken)", targetProject.Name))
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
			m.setSuccessStatus(fmt.Sprintf("Jumped to #%d", task.ShortID))
		}
		return true, defaultStatusTimeout()
	}

	// Otherwise, switch to the project and jump
	m.switchToProject(project, true)
	m.pushView(ViewProjectTree)
	if m.projectTree != nil {
		m.projectTree.JumpToTask(task)
		m.setSuccessStatus(fmt.Sprintf("Jumped to #%d in %s", task.ShortID, project.Name))
	}
	return true, defaultStatusTimeout()
}
