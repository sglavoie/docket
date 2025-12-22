package cmd

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:     "task",
	Aliases: []string{"t"},
	Short:   "Manage tasks",
	Long:    `Commands for creating, listing, and managing tasks within the current project.`,
	RunE:    runTaskList, // Default to listing tasks
}

func init() {
	rootCmd.AddCommand(taskCmd)
}

// getCurrentProject is an alias for LoadCurrentProject for internal package use.
func getCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	return LoadCurrentProject()
}

// saveProject is an alias for SaveProjectWithBackup for internal package use.
func saveProject(project *models.Project) error {
	return SaveProjectWithBackup(project)
}

// findTask is an alias for FindTaskByIdentifier for internal package use.
func findTask(project *models.Project, identifier string) *models.Task {
	return FindTaskByIdentifier(project, identifier)
}

// parseTaskTypeFlags is an alias for ParseTaskTypeFlags for internal package use.
func parseTaskTypeFlags(epic, story, bug, idea bool, typeFlag string) (models.TaskType, error) {
	return ParseTaskTypeFlags(epic, story, bug, idea, typeFlag)
}
