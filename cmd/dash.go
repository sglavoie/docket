package cmd

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
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
	dashCmd.Flags().Bool("json", false, "output result as JSON")
}

func runDash(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	if jsonRequested(cmd) {
		out, err := buildDashboardJSON(cfg, configManager)
		if err != nil {
			return err
		}
		return printJSON(out)
	}

	return displayDashboard(cfg, configManager)
}

// dashboardProjectJSON summarizes one project for the dashboard JSON envelope.
type dashboardProjectJSON struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Context   string `json:"context,omitempty"`
	Tasks     int    `json:"tasks"`
	Active    int    `json:"active"`
	Completed int    `json:"completed"`
}

// dashboardJSON is the machine-readable dashboard summary.
type dashboardJSON struct {
	Projects       []dashboardProjectJSON `json:"projects"`
	CurrentProject string                 `json:"current_project,omitempty"`
}

func buildDashboardJSON(cfg *config.Config, mgr *config.Manager) (dashboardJSON, error) {
	out := dashboardJSON{Projects: []dashboardProjectJSON{}}
	active, err := activeProjects(mgr)
	if err != nil {
		return dashboardJSON{}, err
	}
	for _, p := range active {
		if err := storage.LoadProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load project %q: %v\n", p.Name, err)
			continue
		}
		out.Projects = append(out.Projects, dashboardProjectJSON{
			Name:      p.Name,
			Slug:      p.Slug,
			Context:   p.Context,
			Tasks:     p.CountTasks(),
			Active:    p.CountActiveTasks(),
			Completed: p.CountCompletedTasks(),
		})
	}
	if current := cfg.GetCurrentProject(); current != nil {
		out.CurrentProject = current.Slug
	}
	return out, nil
}

func displayDashboard(cfg *config.Config, mgr *config.Manager) error {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("248"))

	fmt.Println(titleStyle.Render("tt - Task Tracker"))
	fmt.Println()

	// Active projects summary
	active, err := activeProjects(mgr)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		fmt.Println(subtitleStyle.Render(NoItemsMsg("projects", "tt project new <name>")))
		return nil
	}

	fmt.Println(titleStyle.Render("PROJECTS"))
	displayProjectsSummary(active)

	// Current project details
	current := cfg.GetCurrentProject()
	if current != nil {
		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("CURRENT: %s", current.Name)))

		if err := storage.LoadProject(current); err == nil {
			displayRecentTasks(current)
			displayTaskStats(current)
		}
	} else if len(active) > 0 {
		fmt.Println()
		fmt.Println(subtitleStyle.Render("No project selected. Select one with: tt select"))
	}
	return nil
}

func displayProjectsSummary(projects []*models.Project) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Name", "Context", "Tasks", "Active", "Done"})

	for _, p := range projects {
		if err := storage.LoadProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load project %q: %v\n", p.Name, err)
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
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Render(NoItemsMsg("tasks", "tt task add <title>")))
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
			task.State().Icon(),
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
	iconCounts := make(map[string]int)

	for _, task := range project.Tasks {
		typeCounts[task.Type]++
		iconCounts[task.State().Icon()]++
	}

	// Display type breakdown
	var typeStats []string
	for _, tt := range models.AllTaskTypes() {
		if count := typeCounts[tt]; count > 0 {
			typeStats = append(typeStats, fmt.Sprintf("%s:%d", tt.Icon(), count))
		}
	}

	// Display state breakdown. Counts are bucketed per icon because several
	// states share a glyph (inbox/ready/next all render the todo icon).
	var statusStats []string
	seenIcons := make(map[string]bool)
	for _, s := range models.AllWorkflowStates() {
		icon := s.Icon()
		if seenIcons[icon] {
			continue
		}
		seenIcons[icon] = true
		if count := iconCounts[icon]; count > 0 {
			statusStats = append(statusStats, fmt.Sprintf("%s:%d", icon, count))
		}
	}

	statStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	fmt.Println(statStyle.Render(fmt.Sprintf("Types: %s", strings.Join(typeStats, " "))))
	fmt.Println(statStyle.Render(fmt.Sprintf("Status: %s", strings.Join(statusStats, " "))))
}
