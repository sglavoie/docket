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

// handleLinkedTaskNavigation reveals a linked task by entering (or updating) the
// target project's scope on the single authoritative cross Outliner. It never
// constructs a separate AppModel.outliner root, so a subsequent palette action
// resolves the same project-qualified task the user sees, and Esc restores the
// previous Outliner scope.
func (m *AppModel) handleLinkedTaskNavigation(task *models.Task, project *models.Project) (bool, tea.Cmd) {
	cmd := m.enterProjectScopeOutliner(project, task, OutlinerOpenOptions{})
	m.setSuccessStatus(fmt.Sprintf("Jumped to #%d in %s", task.ShortID, project.Name))
	return true, tea.Batch(cmd, defaultStatusTimeout())
}
