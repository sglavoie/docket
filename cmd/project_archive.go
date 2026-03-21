package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectArchiveCmd = &cobra.Command{
	Use:   "archive [name|id]",
	Short: "Archive a project",
	Long: `Archive a project to hide it from the active list.

Archived projects are not deleted and can be unarchived later.

Examples:
  tt project archive "Old Project"
  tt p archive my-project`,
	RunE: runProjectArchive,
}

var projectUnarchiveCmd = &cobra.Command{
	Use:   "unarchive [name|id]",
	Short: "Unarchive a project",
	Long: `Restore an archived project to the active list.

Examples:
  tt project unarchive "Old Project"
  tt p unarchive my-project`,
	RunE: runProjectUnarchive,
}

func init() {
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)
}

func runProjectArchive(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	project, cfg, configManager, err := LoadConfigAndProject(args[0])
	if err != nil {
		return err
	}

	if project.Sandbox {
		return fmt.Errorf("cannot archive the sandbox project")
	}

	if project.Archived {
		return fmt.Errorf("project '%s' is already archived", project.Name)
	}

	project.Archive()

	// Clear current project if it was the archived one
	if cfg.CurrentProject == project.ID {
		cfg.CurrentProject = ""
	}

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	fmt.Printf("Archived project '%s'\n", project.Name)
	return nil
}

func runProjectUnarchive(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
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
		return tterrors.NewProjectNotFoundError(args[0])
	}

	if !project.Archived {
		return fmt.Errorf("project '%s' is not archived", project.Name)
	}

	project.Unarchive()

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	fmt.Printf("Unarchived project '%s'\n", project.Name)
	return nil
}
