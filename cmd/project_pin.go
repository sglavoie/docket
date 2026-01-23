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
	Args: cobra.ExactArgs(1),
	RunE: runProjectPin,
}

func init() {
	projectCmd.AddCommand(projectPinCmd)
}

func runProjectPin(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	project := cfg.GetProject(args[0])
	if project == nil {
		return tterrors.NewProjectNotFoundError(args[0])
	}

	project.TogglePinned()

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if project.Pinned {
		fmt.Printf("Pinned project '%s'\n", project.Name)
	} else {
		fmt.Printf("Unpinned project '%s'\n", project.Name)
	}
	return nil
}
