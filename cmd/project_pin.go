package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var projectPinCmd = &cobra.Command{
	Use:   "pin [name|id]",
	Short: "Toggle a project's pinned status",
	Long: `Toggle a project's pinned status to keep it at the top of lists.

Pinned projects are highlighted in green and appear first in the project list.

Examples:
  tt project pin "My Project"
  tt p pin my-project`,
	RunE: runProjectPin,
}

func init() {
	projectCmd.AddCommand(projectPinCmd)
}

func runProjectPin(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	project, cfg, configManager, err := LoadConfigAndProject(args[0])
	if err != nil {
		return err
	}

	project.TogglePinned()

	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}

	if project.Pinned {
		fmt.Printf("Pinned project '%s'\n", project.Name)
	} else {
		fmt.Printf("Unpinned project '%s'\n", project.Name)
	}
	return nil
}
