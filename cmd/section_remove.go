package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var sectionRemoveForce bool

var sectionRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a section from the current project",
	Long: `Remove a section from the project.

Tasks in the removed section will have their section cleared.
The Next section cannot be removed.

Examples:
  tt section remove "In Progress"
  tt section rm Review
  tt sec delete Done`,
	Args: cobra.ExactArgs(1),
	RunE: runSectionRemove,
}

func init() {
	sectionCmd.AddCommand(sectionRemoveCmd)
	sectionRemoveCmd.Flags().BoolVarP(&sectionRemoveForce, "force", "f", false, "Skip confirmation prompt")
}

func runSectionRemove(cmd *cobra.Command, args []string) error {
	sectionName := args[0]

	if sectionName == models.SectionNext {
		return fmt.Errorf("cannot remove the Next section")
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if !project.HasSection(sectionName) {
		return fmt.Errorf("section '%s' does not exist", sectionName)
	}

	taskCount := len(project.GetTasksBySection(sectionName))

	prompt := fmt.Sprintf("Remove section '%s'?", sectionName)
	if taskCount > 0 {
		prompt = fmt.Sprintf("Remove section '%s'? %d tasks will become unsectioned.", sectionName, taskCount)
	}
	if !ConfirmYN(prompt+" [y/N]: ", sectionRemoveForce) {
		fmt.Println("Canceled.")
		return nil
	}

	if !project.RemoveSection(sectionName) {
		return fmt.Errorf("failed to remove section '%s'", sectionName)
	}

	if err := SaveProject(project, nil); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	if taskCount > 0 {
		fmt.Printf("Removed section '%s' (%d tasks moved to unsectioned)\n", sectionName, taskCount)
	} else {
		fmt.Printf("Removed section: %s\n", sectionName)
	}
	return nil
}
