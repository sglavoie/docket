package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
)

var projectOpenCmd = &cobra.Command{
	Use:   "open [name|id]",
	Short: "Open a project in VS Code",
	Long: `Open a project's directory in VS Code.

If no project is specified, opens an interactive selector.

Examples:
  tt project open my-project
  tt project open              # Interactive selector
  tt p open`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectOpen,
}

var openNewWindow bool

func init() {
	projectCmd.AddCommand(projectOpenCmd)
	projectOpenCmd.Flags().BoolVarP(&openNewWindow, "new-window", "n", false, "open in a new VS Code window")
}

func runProjectOpen(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	var project *models.Project

	if len(args) > 0 {
		project = cfg.GetProject(args[0])
		if project == nil {
			return fmt.Errorf("project '%s' not found", args[0])
		}
	} else {
		// Interactive selection
		projects := cfg.GetActiveProjects()
		if len(projects) == 0 {
			return fmt.Errorf("no projects available")
		}

		// Use fzf if available
		if cfg.FzfEnabled && Deps.Selector.Available() {
			for _, p := range projects {
				if err := storage.LoadProject(p); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to load project %s: %v\n", p.Name, err)
				}
			}
			selected, err := Deps.Selector.SelectProject(projects, "Open project:")
			if err != nil {
				return err
			}
			project = selected
		} else {
			for _, p := range projects {
				if err := storage.LoadProject(p); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to load project %s: %v\n", p.Name, err)
				}
			}

			items := ProjectsToSelectorItems(projects, false)
			selected, err := tui.RunSelector("Open project:", items)
			if err != nil {
				return err
			}
			project, err = GetProjectFromSelector(selected)
			if err != nil {
				return err
			}
		}
	}

	if !Deps.Editor.Available() {
		return fmt.Errorf("VS Code (code) is not available in PATH")
	}

	var openErr error
	if openNewWindow {
		openErr = Deps.Editor.OpenNewWindow(project.Path)
	} else {
		openErr = Deps.Editor.OpenFolder(project.Path)
	}

	if openErr != nil {
		return fmt.Errorf("failed to open VS Code: %w", openErr)
	}

	fmt.Printf("Opening %s in VS Code...\n", project.Name)
	return nil
}
