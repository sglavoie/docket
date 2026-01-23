package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Show dashboard summary",
	Long: `Display a quick overview of projects and tasks.

The dashboard shows:
  - All active projects with task counts
  - Current project details
  - Recent tasks
  - Task statistics by type and status

Examples:
  tt dash`,
	RunE: runDash,
}

func init() {
	rootCmd.AddCommand(dashCmd)
}

func runDash(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	displayDashboard(cfg)
	return nil
}

func displayDashboard(cfg *config.Config) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	fmt.Println(titleStyle.Render("tt - Task Tracker"))
	fmt.Println()

	// Active projects summary
	activeProjects := cfg.GetActiveProjects()
	if len(activeProjects) == 0 {
		fmt.Println(subtitleStyle.Render("No projects yet. Create one with: tt project new <name>"))
		return
	}

	fmt.Println(titleStyle.Render("PROJECTS"))
	displayProjectsSummary(activeProjects)

	// Current project details
	current := cfg.GetCurrentProject()
	if current != nil {
		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("CURRENT: %s", current.Name)))

		if err := storage.LoadProject(current); err == nil {
			displayRecentTasks(current)
			displayTaskStats(current)
		}
	} else if len(activeProjects) > 0 {
		fmt.Println()
		fmt.Println(subtitleStyle.Render("No project selected. Select one with: tt select"))
	}
}

func displayProjectsSummary(projects []*models.Project) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Name", "Context", "Tasks", "Active", "Done"})

	for _, p := range projects {
		if err := storage.LoadProject(p); err != nil {
			continue
		}

		t.AppendRow(table.Row{
			p.Name,
			p.Context,
			p.CountTasks(),
			p.CountActiveTasks(),
			p.CountCompletedTasks(),
		})
	}

	fmt.Println(t.Render())
}

func displayRecentTasks(project *models.Project) {
	recentTasks := storage.GetRecentTasks(project.Tasks, 5)
	if len(recentTasks) == 0 {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("No tasks yet."))
		return
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Render("Recent Tasks"))

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Type", "Status", "Title", "Updated"})

	for _, task := range recentTasks {
		t.AppendRow(table.Row{
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			util.Truncate(task.Title, 40),
			storage.FormatRelativeTime(task.UpdatedAt),
		})
	}

	fmt.Println(t.Render())
}

func displayTaskStats(project *models.Project) {
	if len(project.Tasks) == 0 {
		return
	}

	fmt.Println()

	// Count by type
	typeCounts := make(map[models.TaskType]int)
	statusCounts := make(map[models.TaskStatus]int)

	for _, task := range project.Tasks {
		typeCounts[task.Type]++
		statusCounts[task.Status]++
	}

	// Display type breakdown
	var typeStats []string
	for _, tt := range models.AllTaskTypes() {
		if count := typeCounts[tt]; count > 0 {
			typeStats = append(typeStats, fmt.Sprintf("%s:%d", tt.Icon(), count))
		}
	}

	// Display status breakdown
	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			statusStats = append(statusStats, fmt.Sprintf("%s:%d", s.Icon(), count))
		}
	}

	statStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	fmt.Println(statStyle.Render(fmt.Sprintf("Types: %s", joinStrings(typeStats, " "))))
	fmt.Println(statStyle.Render(fmt.Sprintf("Status: %s", joinStrings(statusStats, " "))))
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
