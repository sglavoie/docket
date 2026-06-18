package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

var projectService = NewProjectService()

// SaveProjectAsync returns a command that saves a project asynchronously.
// It backs up the project first, then saves it.
func SaveProjectAsync(project *models.Project, successMsg string) tea.Cmd {
	return projectService.SaveWithBackup(project, successMsg)
}

// SaveProjectAsyncWithOptions returns a command that saves a project
// asynchronously with optional undo metadata.
func SaveProjectAsyncWithOptions(project *models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd {
	return projectService.SaveWithBackupOptions(project, successMsg, opts)
}

// SaveMultipleProjectsAsync returns a command that saves multiple projects asynchronously.
func SaveMultipleProjectsAsync(projects []*models.Project, successMsg string) tea.Cmd {
	return projectService.SaveMultiple(projects, successMsg)
}

// SaveMultipleProjectsAsyncWithOptions returns a command that saves multiple
// projects asynchronously with optional undo metadata.
func SaveMultipleProjectsAsyncWithOptions(projects []*models.Project, successMsg string, opts ProjectSaveOptions) tea.Cmd {
	return projectService.SaveMultipleOptions(projects, successMsg, opts)
}

// startLoading returns a command that triggers the loading state.
func startLoading(message string) tea.Cmd {
	return func() tea.Msg {
		return SaveStartMsg{Message: message}
	}
}
