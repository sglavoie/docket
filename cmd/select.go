package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
)

var selectCmd = &cobra.Command{
	Use:   "select [project]",
	Short: "Select a project as current",
	Long: `Select a project to work with. If no argument is provided,
opens an interactive selector.

Examples:
  tt select              # Open interactive selector
  tt select my-project   # Select by name/slug directly`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSelect,
}

func init() {
	rootCmd.AddCommand(selectCmd)
}

func runSelect(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	var project *models.Project

	if len(args) > 0 {
		// Direct selection by name/slug
		project = cfg.GetProject(args[0])
		if project == nil {
			return fmt.Errorf("project '%s' not found", args[0])
		}
	} else {
		// Interactive selection
		projects := cfg.GetActiveProjects()
		if len(projects) == 0 {
			return fmt.Errorf("no projects available. Create one with: tt project new <name>")
		}

		// Load projects to get task counts
		for _, p := range projects {
			if err := storage.LoadProject(p); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to load project %s: %v\n", p.Name, err)
			}
		}

		items := ProjectsToSelectorItems(projects, true)
		selected, err := tui.RunSelector("Select project:", items)
		if err != nil {
			return err
		}

		project, err = GetProjectFromSelector(selected)
		if err != nil {
			return err
		}
	}

	cfg.CurrentProject = project.ID

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Selected project: %s\n", project.Name)
	return nil
}
