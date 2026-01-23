package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// SaveProjectAsync returns a command that saves a project asynchronously.
// It backs up the project first, then saves it.
func SaveProjectAsync(project *models.Project, successMsg string) tea.Cmd {
	return func() tea.Msg {
		if err := storage.BackupProject(project); err != nil {
			return SaveErrorMsg{Err: err}
		}
		if err := storage.SaveProject(project); err != nil {
			return SaveErrorMsg{Err: err}
		}
		storage.LoadProject(project)
		return SaveCompleteMsg{SuccessMessage: successMsg}
	}
}

// SaveMultipleProjectsAsync returns a command that saves multiple projects asynchronously.
func SaveMultipleProjectsAsync(projects []*models.Project, successMsg string) tea.Cmd {
	return func() tea.Msg {
		for _, project := range projects {
			if err := storage.BackupProject(project); err != nil {
				return SaveErrorMsg{Err: err}
			}
			if err := storage.SaveProject(project); err != nil {
				return SaveErrorMsg{Err: err}
			}
			storage.LoadProject(project)
		}
		return SaveCompleteMsg{SuccessMessage: successMsg}
	}
}

// startLoading returns a command that triggers the loading state.
func startLoading(message string) tea.Cmd {
	return func() tea.Msg {
		return SaveStartMsg{Message: message}
	}
}
