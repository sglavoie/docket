package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
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
	listJSON       bool
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
  tt task list --json       # Output as JSON
  tt task list --project myproject
  tt t ls`,
	RunE: runTaskList,
}

func init() {
	taskCmd.AddCommand(taskListCmd)

	taskListCmd.Flags().StringVarP(&listTaskType, "type", "t", "", "filter by type (epic, story, task, bug, idea)")
	taskListCmd.Flags().StringVarP(&listTaskStatus, "status", "s", "", "filter by status (todo, in_progress, review, standby, done, blocked, canceled)")
	taskListCmd.Flags().IntVarP(&listPriority, "priority", "P", 0, "filter by priority (1-5)")
	taskListCmd.Flags().StringVarP(&listTag, "tag", "T", "", "filter by tag")
	taskListCmd.Flags().BoolVarP(&listShowAll, "all", "a", false, "include completed/canceled tasks")
	taskListCmd.Flags().BoolVar(&listTreeView, "tree", false, "show hierarchical tree view")
	taskListCmd.Flags().BoolVar(&listSnoozed, "snoozed", false, "include snoozed tasks (hidden by default)")
	taskListCmd.Flags().StringVar(&listSort, "sort", "", "sort by: priority, created, updated, status, title")
	taskListCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")

	taskListCmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")

	_ = taskListCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = taskListCmd.RegisterFlagCompletionFunc("status", completeEnum(statusCompletions))
	_ = taskListCmd.RegisterFlagCompletionFunc("sort", completeEnum(sortCompletions))
	_ = taskListCmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())
}

func runTaskList(cmd *cobra.Command, args []string) error {
	var (
		project       *models.Project
		cfg           *config.Config
		configManager *config.Manager
		err           error
	)

	if slug, _ := cmd.Flags().GetString("project"); slug != "" {
		cfg, configManager, err = LoadConfig()
		if err != nil {
			return err
		}
		project = cfg.GetProject(slug)
		if project == nil {
			return tterrors.NewProjectNotFoundError(slug)
		}
		if err = storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to load project %s: %w", slug, err)
		}
	} else {
		project, cfg, configManager, err = LoadCurrentProject()
		if err != nil {
			return err
		}
	}

	_, _ = cfg, configManager

	// Apply filters
	tasks := filterTasks(project.Tasks)

	// Apply sorting if requested
	if listSort != "" {
		if err := sortTasks(tasks, listSort); err != nil {
			return err
		}
	}

	if listJSON {
		return displayTaskJSON(tasks)
	}

	if len(tasks) == 0 {
		if len(project.Tasks) == 0 {
			fmt.Println(NoItemsMsg("tasks", "tt task add <title>"))
		} else {
			fmt.Println("No tasks match the current filters.")
		}
		return nil
	}

	if listTreeView {
		displayTaskTree(project, taskMatchesFilters)
	} else {
		displayTaskTable(tasks)
	}

	return nil
}

// taskJSONEntry is a slim representation of a task for JSON output,
// avoiding circular references from Children/Parent/Project.
type taskJSONEntry struct {
	ID        string     `json:"id"`
	ShortID   int        `json:"short_id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Priority  int        `json:"priority"`
	Tags      []string   `json:"tags"`
	Section   string     `json:"section"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DueDate   *time.Time `json:"due_date"`
}

func displayTaskJSON(tasks []*models.Task) error {
	entries := make([]taskJSONEntry, len(tasks))
	for i, t := range tasks {
		tags := t.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = taskJSONEntry{
			ID:        t.ID,
			ShortID:   t.ShortID,
			Type:      t.Type.String(),
			Title:     t.Title,
			Status:    t.Status.String(),
			Priority:  t.Priority,
			Tags:      tags,
			Section:   t.Section,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
			DueDate:   t.DueDate,
		}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// taskMatchesFilters returns true if the task passes all active CLI filters.
func taskMatchesFilters(task *models.Task) bool {
	if !listShowAll && task.Status.IsComplete() {
		return false
	}
	if !listSnoozed && task.IsSnoozed() {
		return false
	}
	if listTaskType != "" {
		t, ok := models.ParseTaskType(listTaskType)
		if !ok || task.Type != t {
			return false
		}
	}
	if listTaskStatus != "" {
		s, ok := models.ParseTaskStatus(listTaskStatus)
		if !ok || task.Status != s {
			return false
		}
	}
	if listPriority > 0 && task.Priority != listPriority {
		return false
	}
	if listTag != "" && !task.HasTag(listTag) {
		return false
	}
	return true
}

func filterTasks(tasks []*models.Task) []*models.Task {
	var result []*models.Task
	for _, task := range tasks {
		if taskMatchesFilters(task) {
			result = append(result, task)
		}
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

func displayTaskTree(project *models.Project, matches func(*models.Task) bool) {
	project.BuildTaskTree()
	roots := project.GetRootTasks()

	if len(roots) == 0 {
		fmt.Println("No tasks to display.")
		return
	}

	for _, root := range roots {
		if !subtreeHasMatch(root, matches) {
			continue
		}
		printTaskNode(root, 0, matches)
	}
}

// subtreeHasMatch returns true if the task itself matches the filter
// or any of its descendants do. This ensures ancestor nodes are shown
// as context when their children match (e.g., a completed epic with
// active child tasks is still displayed in tree view).
func subtreeHasMatch(task *models.Task, matches func(*models.Task) bool) bool {
	if matches(task) {
		return true
	}
	for _, child := range task.Children {
		if subtreeHasMatch(child, matches) {
			return true
		}
	}
	return false
}

func printTaskNode(task *models.Task, depth int, matches func(*models.Task) bool) {
	var indent strings.Builder
	for range depth {
		indent.WriteString("  ")
	}

	// Build prefix with expand indicator
	var prefix string
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
	default:
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

	// Print children that match or have matching descendants
	for _, child := range task.Children {
		if !subtreeHasMatch(child, matches) {
			continue
		}
		printTaskNode(child, depth+1, matches)
	}
}
