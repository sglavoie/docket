package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var sectionRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a section in the current project",
	Long: `Rename a section and update all tasks with that section.

The Next section cannot be renamed.

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

	if oldName == models.SectionNext {
		return fmt.Errorf("cannot rename the Next section")
	}

	if newName == "" {
		return fmt.Errorf("new section name cannot be empty")
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
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
