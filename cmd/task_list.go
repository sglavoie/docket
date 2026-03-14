package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

var (
	listTaskType   string
	listTaskStatus string
	listPriority   int
	listTag        string
	listShowAll    bool
	listTreeView   bool
	listSnoozed    bool
	listSort       string
)

var taskListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List tasks",
	Long: `List tasks in the current project with optional filters.

Examples:
  tt task list
  tt task list --type epic
  tt task list --status todo
  tt task list --priority 1
  tt task list --tag backend
  tt task list --all        # Include completed tasks
  tt task list --tree       # Show hierarchical tree view
  tt task list --sort priority
  tt t ls`,
	RunE: runTaskList,
}

func init() {
	taskCmd.AddCommand(taskListCmd)

	taskListCmd.Flags().StringVarP(&listTaskType, "type", "t", "", "filter by type (epic, story, task, bug, idea)")
	taskListCmd.Flags().StringVarP(&listTaskStatus, "status", "s", "", "filter by status (todo, in_progress, done, blocked, canceled)")
	taskListCmd.Flags().IntVarP(&listPriority, "priority", "p", 0, "filter by priority (1-5)")
	taskListCmd.Flags().StringVarP(&listTag, "tag", "g", "", "filter by tag")
	taskListCmd.Flags().BoolVarP(&listShowAll, "all", "a", false, "include completed/canceled tasks")
	taskListCmd.Flags().BoolVar(&listTreeView, "tree", false, "show hierarchical tree view")
	taskListCmd.Flags().BoolVar(&listSnoozed, "snoozed", false, "include snoozed tasks (hidden by default)")
	taskListCmd.Flags().StringVar(&listSort, "sort", "", "sort by: priority, created, updated, status, title")
}

func runTaskList(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	if len(project.Tasks) == 0 {
		fmt.Println("No tasks yet. Add one with: tt task add <title>")
		return nil
	}

	// Apply filters
	tasks := filterTasks(project.Tasks)

	if len(tasks) == 0 {
		fmt.Println("No tasks match the filters.")
		return nil
	}

	// Apply sorting if requested
	if listSort != "" {
		if err := sortTasks(tasks, listSort); err != nil {
			return err
		}
	}

	if listTreeView {
		displayTaskTree(project)
	} else {
		displayTaskTable(tasks)
	}

	return nil
}

func filterTasks(tasks []*models.Task) []*models.Task {
	var result []*models.Task

	for _, task := range tasks {
		// Filter by completed status
		if !listShowAll && task.Status.IsComplete() {
			continue
		}

		// Hide snoozed tasks unless --snoozed flag is set
		if !listSnoozed && task.IsSnoozed() {
			continue
		}

		// Filter by type
		if listTaskType != "" {
			t, ok := models.ParseTaskType(listTaskType)
			if !ok || task.Type != t {
				continue
			}
		}

		// Filter by status
		if listTaskStatus != "" {
			s, ok := models.ParseTaskStatus(listTaskStatus)
			if !ok || task.Status != s {
				continue
			}
		}

		// Filter by priority
		if listPriority > 0 && task.Priority != listPriority {
			continue
		}

		// Filter by tag
		if listTag != "" && !task.HasTag(listTag) {
			continue
		}

		result = append(result, task)
	}

	return result
}

// statusOrder maps statuses to sort order (active statuses first, then complete).
var statusOrder = map[models.TaskStatus]int{
	models.StatusInProgress: 0,
	models.StatusReview:     1,
	models.StatusBlocked:    2,
	models.StatusTodo:       3,
	models.StatusStandby:    4,
	models.StatusDone:       5,
	models.StatusCanceled:   6,
}

func sortTasks(tasks []*models.Task, field string) error {
	switch field {
	case "priority":
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].Priority < tasks[j].Priority
		})
	case "created":
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		})
	case "updated":
		sort.SliceStable(tasks, func(i, j int) bool {
			return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
		})
	case "status":
		sort.SliceStable(tasks, func(i, j int) bool {
			return statusOrder[tasks[i].Status] < statusOrder[tasks[j].Status]
		})
	case "title":
		sort.SliceStable(tasks, func(i, j int) bool {
			return strings.ToLower(tasks[i].Title) < strings.ToLower(tasks[j].Title)
		})
	default:
		return fmt.Errorf("unknown sort field %q (valid: priority, created, updated, status, title)", field)
	}
	return nil
}

func displayTaskTable(tasks []*models.Task) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Type", "Status", "Pri", "Title", "Tags"})

	for _, task := range tasks {
		tagsStr := ""
		if len(task.Tags) > 0 {
			if len(task.Tags) <= 3 {
				tagsStr = strings.Join(task.Tags, ", ")
			} else {
				tagsStr = fmt.Sprintf("%s +%d", strings.Join(task.Tags[:3], ", "), len(task.Tags)-3)
			}
		}

		t.AppendRow(table.Row{
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			task.Priority,
			util.Truncate(task.Title, 40),
			tagsStr,
		})
	}

	fmt.Println(t.Render())
}

func displayTaskTree(project *models.Project) {
	project.BuildTaskTree()
	roots := project.GetRootTasks()

	if len(roots) == 0 {
		fmt.Println("No tasks to display.")
		return
	}

	for _, root := range roots {
		if !listShowAll && root.Status.IsComplete() {
			continue
		}
		printTaskNode(root, 0)
	}
}

func printTaskNode(task *models.Task, depth int) {
	var indent strings.Builder
	for range depth {
		indent.WriteString("  ")
	}

	// Build prefix with expand indicator
	prefix := ""
	if len(task.Children) > 0 {
		prefix = "▼ "
	} else {
		prefix = "  "
	}

	// Style based on status
	style := lipgloss.NewStyle()
	switch task.Status {
	case models.StatusDone:
		style = style.Foreground(lipgloss.Color("242")).Strikethrough(true)
	case models.StatusInProgress:
		style = style.Foreground(lipgloss.Color("82"))
	case models.StatusReview:
		style = style.Foreground(lipgloss.Color("214"))
	case models.StatusBlocked:
		style = style.Foreground(lipgloss.Color("196"))
	}

	line := fmt.Sprintf("%s%s%s %s #%d %s",
		indent.String(),
		prefix,
		task.Type.Icon(),
		task.Status.Icon(),
		task.ShortID,
		task.Title,
	)

	fmt.Println(style.Render(line))

	// Print children
	for _, child := range task.Children {
		if !listShowAll && child.Status.IsComplete() {
			continue
		}
		printTaskNode(child, depth+1)
	}
}
