package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var sectionAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new section to the current project",
	Long: `Add a new section to organize tasks.

Section names must not contain spaces (use hyphens or camelCase).

Examples:
  tt section add InProgress
  tt section add Review
  tt sec add Done`,
	RunE: runSectionAdd,
}

func init() {
	sectionCmd.AddCommand(sectionAddCmd)
}

func runSectionAdd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("section name")
	}

	sectionName := args[0]

	if err := models.ValidateSectionName(sectionName); err != nil {
		return err
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
