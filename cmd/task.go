package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/tt/internal/config"
	"github.com/sglavoie/dev-helpers/go/tt/internal/models"
	"github.com/sglavoie/dev-helpers/go/tt/internal/storage"
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

// getCurrentProject loads and returns the current project.
func getCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return nil, nil, nil, fmt.Errorf("no current project selected. Select one with: tt select")
	}

	if err := storage.LoadProject(project); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}

	return project, cfg, configManager, nil
}

// saveProject saves the project's tasks, creating a backup first.
func saveProject(project *models.Project) error {
	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return storage.SaveProject(project)
}
