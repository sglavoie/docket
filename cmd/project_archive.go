package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/tt/internal/config"
	"github.com/sglavoie/dev-helpers/go/tt/internal/models"
	"github.com/spf13/cobra"
)

var projectArchiveCmd = &cobra.Command{
	Use:   "archive [name|id]",
	Short: "Archive a project",
	Long: `Archive a project to hide it from the active list.

Archived projects are not deleted and can be unarchived later.

Examples:
  tt project archive "Old Project"
  tt p archive my-project`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectArchive,
}

var projectUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [name|id]",
	Short: "Unarchive a project",
	Long: `Restore an archived project to the active list.

Examples:
  tt project unarchive "Old Project"
  tt p unarchive my-project`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectUnarchive,
}

func init() {
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)
}

func runProjectArchive(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project := cfg.GetProject(args[0])
	if project == nil {
		return fmt.Errorf("project '%s' not found", args[0])
	}

	if project.Archived {
		return fmt.Errorf("project '%s' is already archived", project.Name)
	}

	project.Archive()

	// Clear current project if it was the archived one
	if cfg.CurrentProject == project.ID {
		cfg.CurrentProject = ""
	}

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Archived project '%s'\n", project.Name)
	return nil
}

func runProjectUnarchive(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var project *models.Project
	// Search in all projects including archived
	for _, p := range cfg.Projects {
		if p.ID == args[0] || p.Slug == args[0] || p.Name == args[0] {
			project = p
			break
		}
	}

	if project == nil {
		return fmt.Errorf("project '%s' not found", args[0])
	}

	if !project.Archived {
		return fmt.Errorf("project '%s' is not archived", project.Name)
	}

	project.Unarchive()

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Unarchived project '%s'\n", project.Name)
	return nil
}
