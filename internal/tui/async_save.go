package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

var projectService = NewProjectService()

// SaveProjectAsyncWithOptions returns a command that saves a project
// asynchronously with optional undo metadata.
func SaveProjectAsyncWithOptions(project *models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd {
	return projectService.SaveWithBackupOptions(project, successMsg, opts)
}

// SaveMultipleProjectsAsync returns a command that saves multiple projects asynchronously.
func SaveMultipleProjectsAsync(projects []*models.Project, successMsg string) tea.Cmd {
	return projectService.SaveMultiple(projects, successMsg)
}

// startLoading returns a command that triggers the loading state.
func startLoading(message string) tea.Cmd {
	return func() tea.Msg {
		return SaveStartMsg{Message: message}
	}
}
