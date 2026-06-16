package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var sectionAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Section creation is no longer supported",
	Long: `Sections are fixed to the built-in GTD set.

Flexible grouping now uses tags and pinned tag tabs.

Examples:
  tt tag pin backend
  tt tag list --pinned`,
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
	if models.IsGTDSectionNameCI(sectionName) {
		return fmt.Errorf("cannot add section '%s': built-in GTD sections already exist and cannot be added", sectionName)
	}
	return fmt.Errorf("cannot add section '%s': flexible grouping now uses tags; use 'tt tag pin %s'", sectionName, sectionName)
}
