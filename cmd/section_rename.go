package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var sectionRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a section in the current project",
	Long: `Rename a section and update all tasks with that section.

The Next section cannot be renamed. New names must not contain spaces.

Examples:
  tt section rename InProgress Working
  tt section rename Review Done`,
	RunE: runSectionRename,
}

func init() {
	sectionCmd.AddCommand(sectionRenameCmd)
	addOutputFlags(sectionRenameCmd)
}

func runSectionRename(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("old name and new name")
	}

	oldName := args[0]
	newName := args[1]

	if err := models.ValidateSectionName(newName); err != nil {
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

	if err := project.RenameSection(oldName, newName); err != nil {
		return err
	}

	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Rename Section", UndoContext: fmt.Sprintf("%s → %s", oldName, newName),
	}); err != nil {
		return tterrors.NewSaveProjectError(project.Name, err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, err := json.MarshalIndent(struct {
			OldName string `json:"old_name"`
			NewName string `json:"new_name"`
			Project string `json:"project"`
		}{OldName: oldName, NewName: newName, Project: project.Name}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Renamed section '%s' to '%s'\n", oldName, newName)
	return nil
}
