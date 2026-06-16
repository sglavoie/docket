package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var sectionRemoveForce bool

var sectionRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "Section removal is no longer supported",
	Long: `Sections are fixed to the built-in GTD set.

Flexible grouping now uses tags and pinned tag tabs.

Examples:
  tt tag unpin backend
  tt tag list --pinned`,
	RunE: runSectionRemove,
}

func init() {
	sectionCmd.AddCommand(sectionRemoveCmd)
	sectionRemoveCmd.Flags().BoolVarP(&sectionRemoveForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(sectionRemoveCmd)
}

func runSectionRemove(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("section name")
	}

	sectionName := args[0]

	if models.IsGTDSectionNameCI(sectionName) {
		return fmt.Errorf("cannot remove section '%s': built-in GTD sections cannot be removed", sectionName)
	}
	return fmt.Errorf("cannot remove section '%s': flexible grouping now uses tags; use 'tt tag unpin %s' to unpin a tag tab", sectionName, sectionName)
}
