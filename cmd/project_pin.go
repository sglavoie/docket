package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var projectPinCmd = &cobra.Command{
	Use:   "pin <slug-or-id>",
	Short: "Toggle a project's pinned status",
	Long: `Toggle a project's pinned status to keep it at the top of lists.

Pinned projects are highlighted in green and appear first in the project list.

Examples:
  tt project pin my-project
  tt p pin my-project`,
	RunE: runProjectPin,
}

func init() {
	projectCmd.AddCommand(projectPinCmd)
	addOutputFlags(projectPinCmd)
}

func runProjectPin(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	project, cfg, configManager, err := LoadConfigAndProject(args[0])
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project.TogglePinned()

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if project.Pinned {
		return out.OutputProject(project, func() { fmt.Printf("Pinned project '%s'\n", project.Name) })
	}
	return out.OutputProject(project, func() { fmt.Printf("Unpinned project '%s'\n", project.Name) })
}
