package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

var projectShowCmd = &cobra.Command{
	Use:   "show [name|id]",
	Short: "Show project details",
	Long: `Display detailed information about a project.

If no project is specified, shows the current project.

Examples:
  tt project show
  tt project show "My Project"
  tt p show my-project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectShow,
}

func init() {
	projectCmd.AddCommand(projectShowCmd)
}

func runProjectShow(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	var project *models.Project

	if len(args) > 0 {
		project = cfg.GetProject(args[0])
		if project == nil {
			return fmt.Errorf("project '%s': %w", args[0], tterrors.ErrProjectNotFound)
		}
	} else {
		project = cfg.GetCurrentProject()
		if project == nil {
			return fmt.Errorf("%w. Specify a project or select one with: tt select", tterrors.ErrNoCurrentProject)
		}
	}

	// Load tasks
	if err := storage.LoadProject(project); err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Display project info
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	fmt.Println(titleStyle.Render(project.Name))
	if project.Description != "" {
		fmt.Println(project.Description)
	}
	fmt.Println()

	fmt.Printf("%s %s\n", labelStyle.Render("ID:"), project.ID)
	fmt.Printf("%s %s\n", labelStyle.Render("Slug:"), project.Slug)
	fmt.Printf("%s %s\n", labelStyle.Render("Context:"), project.Context)
	fmt.Printf("%s %s\n", labelStyle.Render("Path:"), project.Path)
	fmt.Printf("%s %s\n", labelStyle.Render("Created:"), project.CreatedAt.Format(util.DateFormat))

	if project.Archived {
		fmt.Printf("%s %s\n", labelStyle.Render("Archived:"), project.ArchivedAt.Format(util.DateFormat))
	}

	fmt.Println()

	// Task statistics
	fmt.Println(titleStyle.Render("Tasks"))

	if len(project.Tasks) == 0 {
		fmt.Println("No tasks yet.")
		return nil
	}

	// By type
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Type", "Total", "Active", "Done"})

	for _, taskType := range models.AllTaskTypes() {
		tasks := project.GetTasksByType(taskType)
		if len(tasks) == 0 {
			continue
		}

		active := 0
		done := 0
		for _, task := range tasks {
			if task.Status.IsActive() {
				active++
			} else if task.IsComplete() {
				done++
			}
		}

		t.AppendRow(table.Row{
			fmt.Sprintf("%s %s", taskType.Icon(), taskType.HeadingPrefix()),
			len(tasks),
			active,
			done,
		})
	}

	fmt.Println(t.Render())

	return nil
}
