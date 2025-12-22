package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/spf13/cobra"
)

var projectActiveCmd = &cobra.Command{
	Use:   "active [name|id]",
	Short: "Toggle a project's active status",
	Long: `Toggle a project's active status to mark it as being actively worked on.

Active projects are highlighted in green in the project list.

Examples:
  tt project active "My Project"
  tt p active my-project`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectActive,
}

func init() {
	projectCmd.AddCommand(projectActiveCmd)
}

func runProjectActive(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project := cfg.GetProject(args[0])
	if project == nil {
		return fmt.Errorf("project '%s' not found", args[0])
	}

	project.ToggleActive()

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if project.Active {
		fmt.Printf("Marked project '%s' as active\n", project.Name)
	} else {
		fmt.Printf("Marked project '%s' as inactive\n", project.Name)
	}
	return nil
}
