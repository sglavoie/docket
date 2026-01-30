package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var sectionAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new section to the current project",
	Long: `Add a new section to organize tasks.

Examples:
  tt section add "In Progress"
  tt section add Review
  tt sec add Done`,
	Args: cobra.ExactArgs(1),
	RunE: runSectionAdd,
}

func init() {
	sectionCmd.AddCommand(sectionAddCmd)
}

func runSectionAdd(cmd *cobra.Command, args []string) error {
	sectionName := args[0]
	if sectionName == "" {
		return fmt.Errorf("section name cannot be empty")
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if project.HasSection(sectionName) {
		return fmt.Errorf("section '%s' already exists", sectionName)
	}

	project.EnsureSection(sectionName)

	if err := SaveProject(project, nil); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	fmt.Printf("Added section: %s\n", sectionName)
	return nil
}
