package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var sectionRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Section rename is no longer supported",
	Long: `Sections are fixed to the built-in GTD set.

Flexible grouping now uses tags and pinned tag tabs.

Examples:
  tt tag pin backend
  tt tag unpin backend`,
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

	if models.IsGTDSectionNameCI(oldName) {
		return fmt.Errorf("cannot rename section '%s': built-in GTD sections cannot be renamed", oldName)
	}

	if oldName == newName {
		return fmt.Errorf("new section name is identical to old name: '%s'", oldName)
	}

	if err := models.ValidateSectionName(newName); err != nil {
		return err
	}

	if models.IsGTDSectionNameCI(newName) {
		return fmt.Errorf("cannot rename to '%s': built-in GTD section names are reserved", newName)
	}
	return fmt.Errorf("cannot rename section '%s': flexible grouping now uses tags", oldName)
}
