package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var focusCmd = &cobra.Command{
	Use:   "focus",
	Short: "Show actionable tasks across projects",
	Long: `Show all actionable tasks across active projects, sorted by priority.

By default, lists all non-completed, non-snoozed, non-blocked tasks.
Use --active to show only active-flagged tasks (legacy behavior).

Examples:
  tt focus            # all actionable tasks
  tt focus --active   # only active-flagged tasks`,
	RunE: runFocus,
}

var focusActiveOnly bool
var focusSuggestLimit int

var focusSuggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Show ranked task suggestions across projects",
	Long: `Show tasks ranked by the "What's Next" suggestion engine.

Tasks are scored based on priority, due dates, status, activity recency,
unblocking potential, and composite scoring profiles.

Examples:
  tt focus suggest        # top 10 suggestions
  tt focus suggest -n 5   # top 5 suggestions`,
	RunE: runFocusSuggest,
}

var focusQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Show the persisted focus work queue",
	Long: `Show tasks that have been accepted into the focus work queue.

The queue is managed through the TUI's focus view (press 'a' to accept
suggestions, 'x' to remove, J/K to reorder).

Examples:
  tt focus queue`,
	RunE: runFocusQueue,
}

func init() {
	rootCmd.AddCommand(focusCmd)
	focusCmd.Flags().BoolVar(&focusActiveOnly, "active", false, "Show only active-flagged tasks")

	focusCmd.AddCommand(focusSuggestCmd)
	focusSuggestCmd.Flags().IntVarP(&focusSuggestLimit, "number", "n", 10, "Number of suggestions to show")

	focusCmd.AddCommand(focusQueueCmd)
}

func runFocus(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	// Load and collect tasks from all active projects
	projects := loadActiveProjects(cfg)
	var tasks []focus.FocusTask

	if focusActiveOnly {
		// Legacy: only active-flagged, non-completed tasks
		for _, project := range projects {
			for _, task := range project.Tasks {
				if task.Status.IsComplete() || !task.Active {
					continue
				}
				tasks = append(tasks, focus.FocusTask{Task: task, Project: project})
			}
		}
	} else {
		tasks = focus.CollectActionableTasks(projects)
	}

	// Sort by priority (ascending, 1=highest), then by UpdatedAt (most recent first)
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
	})

	if len(tasks) == 0 {
		if focusActiveOnly {
			fmt.Println("No tasks in focus.")
			fmt.Println("Mark tasks as active with: tt task focus <id>")
		} else {
			fmt.Println("No actionable tasks found.")
			fmt.Println("Tasks appear here from all active projects (excluding completed, snoozed, and blocked).")
		}
		return nil
	}

	displayFocusTable(tasks)
	return nil
}

func displayFocusTable(tasks []focus.FocusTask) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Project", "ID", "Type", "Status", "Pri", "Title", "Due"})

	for _, ft := range tasks {
		task := ft.Task
		dueStr := focus.FormatRelativeDue(task.DueDate)

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
	if focusActiveOnly {
		fmt.Printf("\n%s in focus\n", Pluralize(len(tasks), "task", "tasks"))
	} else {
		fmt.Printf("\n%s\n", Pluralize(len(tasks), "actionable task", "actionable tasks"))
	}
}

// loadActiveProjects loads task data for all active projects, returning
// only those that loaded successfully.
func loadActiveProjects(cfg *config.Config) []*models.Project {
	var loaded []*models.Project
	for _, project := range cfg.GetActiveProjects() {
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load project %q: %v\n", project.Slug, err)
			continue
		}
		loaded = append(loaded, project)
	}
	return loaded
}

func runFocusSuggest(cmd *cobra.Command, args []string) error {
	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	tasks := focus.CollectActionableTasks(loadActiveProjects(cfg))
	if len(tasks) == 0 {
		fmt.Println("No actionable tasks found.")
		return nil
	}

	ranked := focus.RankCrossProjectTasks(tasks, nil, cfg)
	if len(ranked) == 0 {
		fmt.Println("No suggestions available.")
		return nil
	}

	limit := min(focusSuggestLimit, len(ranked))

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"#", "Project", "ID", "Pri", "Title", "Score", "Reason"})

	for i, sft := range ranked[:limit] {
		t.AppendRow(table.Row{
			i + 1,
			util.Truncate(sft.Project.Name, 15),
			sft.Task.ShortID,
			sft.Task.Priority,
			util.Truncate(sft.Task.Title, 40),
			sft.Score,
			util.Truncate(sft.Reason, 40),
		})
	}

	fmt.Println(t.Render())
	fmt.Printf("\nTop %d of %d suggestions\n", limit, len(ranked))
	return nil
}

func runFocusQueue(cmd *cobra.Command, args []string) error {
	cfg, cfgMgr, err := LoadConfigStrict()
	if err != nil {
		return err
	}

	queueItems := cfg.GetFocusQueue()
	if len(queueItems) == 0 {
		fmt.Println("Focus queue is empty.")
		fmt.Println("Accept suggestions in the TUI focus view (press 'a') to build your queue.")
		return nil
	}

	// Load projects and build task lookup
	projectMap := make(map[string]*models.Project)
	taskMap := make(map[string]*models.Task)
	for _, p := range cfg.GetActiveProjects() {
		if err := storage.LoadProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load project %q: %v\n", p.Slug, err)
			continue
		}
		projectMap[p.ID] = p
		for _, task := range p.Tasks {
			taskMap[task.ID] = task
		}
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"#", "Project", "ID", "Type", "Status", "Pri", "Title", "Due"})

	var activeItems []config.FocusQueueItem
	for _, item := range queueItems {
		project, ok := projectMap[item.ProjectID]
		if !ok {
			continue
		}
		task, ok := taskMap[item.TaskID]
		if !ok || task.Status.IsComplete() {
			continue
		}

		activeItems = append(activeItems, item)
		dueStr := focus.FormatRelativeDue(task.DueDate)

		t.AppendRow(table.Row{
			len(activeItems),
			util.Truncate(project.Name, 15),
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			task.Priority,
			util.Truncate(task.Title, 40),
			dueStr,
		})
	}

	// Prune stale entries from config if any were skipped
	if len(activeItems) < len(queueItems) {
		cfg.SetFocusQueue(activeItems)
		_ = cfgMgr.Save(cfg)
	}

	if len(activeItems) == 0 {
		fmt.Println("Focus queue is empty (all queued tasks have been completed).")
		return nil
	}

	fmt.Println(t.Render())
	fmt.Printf("\n%s in queue\n", Pluralize(len(activeItems), "task", "tasks"))
	return nil
}
