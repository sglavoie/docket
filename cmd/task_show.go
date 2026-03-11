package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
	"github.com/sglavoie/tt/internal/util"
)

var taskShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show task details",
	Long: `Display detailed information about a task.

If no ID is specified, opens an interactive selector.
Uses glow for rendering if available and enabled in config.

Examples:
  tt task show 5
  tt task show
  tt task show 3 --no-glow
  tt t show`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskShow,
}

var noGlow bool

func init() {
	taskCmd.AddCommand(taskShowCmd)
	taskShowCmd.Flags().BoolVar(&noGlow, "no-glow", false, "disable glow rendering")
}

func runTaskShow(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task

	if len(args) > 0 {
		var findErr error
		task, findErr = FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
	} else {
		// Interactive selection
		activeTasks := filterTasks(project.Tasks)
		if len(activeTasks) == 0 {
			return fmt.Errorf("no tasks available. Add one with: tt task add <title>")
		}

		items := TasksToSelectorItems(activeTasks)
		selected, err := tui.RunSelector("Select task:", items)
		if err != nil {
			return err
		}

		task, err = GetTaskFromSelector(selected)
		if err != nil {
			return err
		}
	}

	// Use glow if available and enabled
	useGlow := cfg.GlowEnabled && Deps.Renderer.Available() && !noGlow
	if useGlow {
		return Deps.Renderer.RenderTaskMarkdown(task)
	}

	displayTaskDetail(task)
	return nil
}

func displayTaskDetail(task *models.Task) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle := lipgloss.NewStyle()

	// Header
	fmt.Println(titleStyle.Render(fmt.Sprintf("%s #%d: %s",
		task.Type.HeadingPrefix(), task.ShortID, task.Title)))
	fmt.Println()

	// Metadata
	fmt.Printf("%s %s %s\n",
		labelStyle.Render("Status:"),
		task.Status.Icon(),
		valueStyle.Render(string(task.Status)))

	fmt.Printf("%s %d\n",
		labelStyle.Render("Priority:"),
		task.Priority)

	fmt.Printf("%s %s\n",
		labelStyle.Render("Type:"),
		valueStyle.Render(task.Type.HeadingPrefix()))

	if len(task.Tags) > 0 {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Tags:"),
			valueStyle.Render(strings.Join(task.Tags, ", ")))
	}

	fmt.Printf("%s %s\n",
		labelStyle.Render("Created:"),
		valueStyle.Render(task.CreatedAt.Format(util.DateTimeFormat)))

	fmt.Printf("%s %s\n",
		labelStyle.Render("Updated:"),
		valueStyle.Render(storage.FormatRelativeTime(task.UpdatedAt)))

	if task.CompletedAt != nil {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Completed:"),
			valueStyle.Render(task.CompletedAt.Format(util.DateTimeFormat)))
	}

	if task.DueDate != nil {
		fmt.Printf("%s %s\n",
			labelStyle.Render("Due:"),
			valueStyle.Render(task.DueDate.Format(util.DateFormat)))
	}

	// Description
	if task.Description != "" {
		fmt.Println()
		fmt.Println(labelStyle.Render("Description:"))
		fmt.Println(task.Description)
	}

	// Children summary
	if task.HasProgressChildren() {
		fmt.Println()
		total := 0
		completed := 0
		for _, child := range task.Children {
			total++
			if child.IsComplete() {
				completed++
			}
		}
		fmt.Printf("%s %d/%d completed (%.0f%%)\n",
			labelStyle.Render("Subtasks:"),
			completed, total,
			task.GetProgress())
	}
}
