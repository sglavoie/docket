package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/spf13/cobra"
)

var sectionRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a section in the current project",
	Long: `Rename a section and update all tasks with that section.

The Backlog section cannot be renamed.

Examples:
  tt section rename "In Progress" "Working"
  tt section rename Review Done`,
	Args: cobra.ExactArgs(2),
	RunE: runSectionRename,
}

func init() {
	sectionCmd.AddCommand(sectionRenameCmd)
}

func runSectionRename(cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	if oldName == models.SectionBacklog {
		return fmt.Errorf("cannot rename the Backlog section")
	}

	if newName == "" {
		return fmt.Errorf("new section name cannot be empty")
	}

	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return fmt.Errorf("no project selected. Use 'tt select' to choose a project")
	}

	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	if !project.HasSection(oldName) {
		return fmt.Errorf("section '%s' does not exist", oldName)
	}

	if project.HasSection(newName) {
		return fmt.Errorf("section '%s' already exists", newName)
	}

	if !project.RenameSection(oldName, newName) {
		return fmt.Errorf("failed to rename section '%s'", oldName)
	}

	if err := storage.SaveProject(project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	fmt.Printf("Renamed section '%s' to '%s'\n", oldName, newName)
	return nil
}
