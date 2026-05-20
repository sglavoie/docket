package cmd

import (
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

	if models.IsGTDSection(oldName) {
		return fmt.Errorf("cannot rename built-in section '%s'", oldName)
	}

	if err := models.ValidateSectionName(newName); err != nil {
		return err
	}

	// Reject renaming to a built-in GTD section name. Without this guard a custom
	// section can be renamed to "Inbox", "Next", etc., silently creating a
	// duplicate that conflicts with the built-in section.
	if models.IsGTDSection(newName) {
		return fmt.Errorf("cannot rename to '%s': name is reserved for the built-in section", newName)
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
		return printJSON(struct {
			OldName string `json:"old_name"`
			NewName string `json:"new_name"`
			Project string `json:"project"`
		}{OldName: oldName, NewName: newName, Project: project.Name})
	}
	fmt.Printf("Renamed section '%s' to '%s'\n", oldName, newName)
	return nil
}
