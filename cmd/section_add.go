package cmd

import (
	"encoding/json"
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
	addOutputFlags(sectionAddCmd)
}

func runSectionAdd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("section name")
	}

	sectionName := args[0]

	if err := models.ValidateSectionName(sectionName); err != nil {
		return err
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if project.HasSection(sectionName) {
		return fmt.Errorf("section '%s' already exists; use 'tt section list' to see sections", sectionName)
	}

	project.EnsureSection(sectionName)

	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Add Section", UndoContext: sectionName,
	}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, _ := json.MarshalIndent(struct {
			Section string `json:"section"`
			Project string `json:"project"`
		}{Section: sectionName, Project: project.Name}, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Added section '%s' to project '%s'\n", sectionName, project.Name)
	return nil
}
