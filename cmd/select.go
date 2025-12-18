package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/tt/internal/config"
	"github.com/sglavoie/dev-helpers/go/tt/internal/models"
	"github.com/sglavoie/dev-helpers/go/tt/internal/storage"
	"github.com/sglavoie/dev-helpers/go/tt/internal/tui"
	"github.com/spf13/cobra"
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
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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

		// Build selector items
		items := make([]tui.SelectorItem, len(projects))
		for i, p := range projects {
			if err := storage.LoadProject(p); err == nil {
				items[i] = tui.SelectorItem{
					ID:   p.ID,
					Data: p,
					Columns: []string{
						p.Name,
						p.Context,
						fmt.Sprintf("%d tasks", p.CountTasks()),
						fmt.Sprintf("%d active", p.CountActiveTasks()),
					},
				}
			}
		}

		selected, err := tui.RunSelector("Select project:", items)
		if err != nil {
			return err
		}

		project = selected.Data.(*models.Project)
	}

	cfg.CurrentProject = project.ID

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Selected project: %s\n", project.Name)
	return nil
}
