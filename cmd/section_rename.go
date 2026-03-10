package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var sectionRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a section in the current project",
	Long: `Rename a section and update all tasks with that section.

The Next section cannot be renamed. New names must not contain spaces.

Examples:
  tt section rename InProgress Working
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

	if err := models.ValidateSectionName(newName); err != nil {
		return err
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if err := project.RenameSection(oldName, newName); err != nil {
		return err
	}

	if err := SaveProject(project, nil); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	fmt.Printf("Renamed section '%s' to '%s'\n", oldName, newName)
	return nil
}
