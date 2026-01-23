package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var focusCmd = &cobra.Command{
	Use:   "focus",
	Short: "List all active-flagged tasks across projects",
	Long: `List all tasks marked as "active" (focused) across all active projects.

Tasks are sorted by priority (highest first), then by most recently updated.

Examples:
  tt focus`,
	RunE: runFocus,
}

func init() {
	rootCmd.AddCommand(focusCmd)
}

// focusTask wraps a task with its project for display.
type focusTask struct {
	Task    *models.Task
	Project *models.Project
}

func runFocus(cmd *cobra.Command, args []string) error {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Collect active tasks from all projects
	var tasks []focusTask
	projects := cfg.GetActiveProjects()

	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if !task.Active {
				continue
			}
			if task.Status.IsComplete() {
				continue
			}
			tasks = append(tasks, focusTask{Task: task, Project: project})
		}
	}

	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
	})

	if len(tasks) == 0 {
		fmt.Println("No tasks in focus.")
		fmt.Println("Mark tasks as active with: tt task focus <id>")
		return nil
	}

	displayFocusTable(tasks)
	return nil
}

func displayFocusTable(tasks []focusTask) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Project", "ID", "Type", "Status", "Pri", "Title", "Due"})

	for _, ft := range tasks {
		task := ft.Task
		dueStr := ""
		if task.DueDate != nil {
			dueStr = formatRelativeDueCLI(task.DueDate)
		}

		t.AppendRow(table.Row{
			util.Truncate(ft.Project.Name, 15),
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			task.Priority,
			util.Truncate(task.Title, 40),
			dueStr,
		})
	}

	fmt.Println(t.Render())
	fmt.Printf("\n%d task(s) in focus\n", len(tasks))
}

func formatRelativeDueCLI(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, now.Location())

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return dueDate.Format("Mon")
	default:
		if dueDate.Year() != now.Year() {
			return dueDate.Format("Jan 2 '06")
		}
		return dueDate.Format("Jan 2")
	}
}
