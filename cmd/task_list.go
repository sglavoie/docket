package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

var (
	listTaskType string
	listState    string
	listBlocked  bool
	listPriority int
	listTag      string
	listShowAll  bool
	listTreeView bool
	listSnoozed  bool
	listParked   bool
	listSort     string
	listJSON     bool

	// listFilterProject holds the project being listed so the derived
	// dependency-blocked filter (--blocked) can resolve a task's unresolved
	// dependencies. Set in runTaskList before filtering; nil disables the
	// project-scoped blocked check.
	listFilterProject *models.Project
)

var taskListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List tasks",
	Long: `List tasks in the current project with optional filters.

Examples:
  tt task list
  tt task list --type epic
  tt task list --state waiting
  tt task list --priority 1
  tt task list --tag backend
  tt task list --all        # Include completed tasks
  tt task list --tree       # Show hierarchical tree view
  tt task list --parked     # Show parked tasks only
  tt task list --sort priority
  tt task list --sort state
  tt task list --json       # Output as JSON
  tt task list --project myproject
  tt t ls`,
	RunE: runTaskList,
}

func init() {
	taskCmd.AddCommand(taskListCmd)

	taskListCmd.Flags().StringVarP(&listTaskType, "type", "t", "", "filter by type (epic, story, task, bug, idea)")
	taskListCmd.Flags().StringVar(&listState, "state", "", "filter by workflow state (inbox, ready, next, in_progress, waiting, paused, review, someday, done, canceled)")
	taskListCmd.Flags().BoolVar(&listBlocked, "blocked", false, "show only tasks blocked by an unresolved dependency")
	taskListCmd.Flags().IntVarP(&listPriority, "priority", "P", 0, "filter by priority (1-5)")
	taskListCmd.Flags().StringVarP(&listTag, "tag", "T", "", "filter by tag")
	taskListCmd.Flags().BoolVarP(&listShowAll, "all", "a", false, "include completed/canceled tasks")
	taskListCmd.Flags().BoolVar(&listTreeView, "tree", false, "show hierarchical tree view")
	taskListCmd.Flags().BoolVar(&listSnoozed, "snoozed", false, "include snoozed tasks (hidden by default)")
	taskListCmd.Flags().BoolVar(&listParked, "parked", false, "show parked tasks only")
	taskListCmd.Flags().StringVar(&listSort, "sort", "", "sort by: priority, created, updated, state, title")
	taskListCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")

	taskListCmd.Flags().StringP("project", "p", "", "target project slug (default: current project)")

	_ = taskListCmd.RegisterFlagCompletionFunc("type", completeEnum(taskTypeCompletions))
	_ = taskListCmd.RegisterFlagCompletionFunc("state", completeEnum(stateCompletions))
	_ = taskListCmd.RegisterFlagCompletionFunc("sort", completeEnum(sortCompletions))
	_ = taskListCmd.RegisterFlagCompletionFunc("project", completeProjectSlugs())
}

func runTaskList(cmd *cobra.Command, args []string) error {
	var (
		project   *models.Project
		workspace *CLIWorkspace
		err       error
	)

	if slug, _ := cmd.Flags().GetString("project"); slug != "" {
		workspace, err = LoadCLIWorkspace()
		if err != nil {
			return err
		}
		project = workspace.Runtime.GetProject(slug)
		if project == nil {
			return tterrors.NewProjectNotFoundError(slug)
		}
		if err = storage.LoadProject(project); err != nil {
			return fmt.Errorf("failed to load project %s: %w", slug, err)
		}
	} else {
		project, _, err = LoadRuntimeCurrentProject()
		if err != nil {
			return err
		}
	}

	// Validate filter flag values up front so a typo (e.g. --type fix) surfaces
	// as an error instead of silently returning an empty list.
	if listTaskType != "" {
		if _, ok := models.ParseTaskType(listTaskType); !ok {
			return tterrors.NewInvalidTaskTypeError(listTaskType)
		}
	}
	if listState != "" {
		if _, ok := models.ParseWorkflowState(listState); !ok {
			return fmt.Errorf("invalid state %q (valid: %s)", listState, strings.Join(stateCompletions, ", "))
		}
	}
	if listPriority != 0 {
		if err := ValidatePriority(listPriority, false); err != nil {
			return err
		}
	}

	// The derived dependency-blocked filter resolves each task's unresolved
	// dependencies within its project, so expose the loaded project to the
	// predicate. BuildTaskTree wires up dependency relationships.
	project.BuildTaskTree()
	listFilterProject = project

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
	ID           string     `json:"id"`
	ShortID      int        `json:"short_id"`
	Type         string     `json:"type"`
	Title        string     `json:"title"`
	State        string     `json:"state"`
	Priority     int        `json:"priority"`
	ParentID     *string    `json:"parent_id"`
	Tags         []string   `json:"tags"`
	Pinned       bool       `json:"pinned"`
	Dimmed       bool       `json:"dimmed"`
	SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`
	Parked       bool       `json:"parked"`
	ParkedNote   string     `json:"parked_note,omitempty"`
	ParkedAt     *time.Time `json:"parked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DueDate      *time.Time `json:"due_date"`
	HasTime      bool       `json:"has_time"`
}

// taskListJSONEntries builds public state-only task rows.
func taskListJSONEntries(tasks []*models.Task) []taskJSONEntry {
	entries := make([]taskJSONEntry, len(tasks))
	for i, t := range tasks {
		tags := t.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = taskJSONEntry{
			ID:           t.ID,
			ShortID:      t.ShortID,
			Type:         t.Type.String(),
			Title:        t.Title,
			State:        t.State().String(),
			Priority:     t.Priority,
			ParentID:     t.ParentID,
			Tags:         tags,
			Pinned:       t.Pinned,
			Dimmed:       t.Dimmed,
			SnoozedUntil: t.SnoozedUntil,
			Parked:       t.IsParked(),
			ParkedNote:   t.ParkedNote,
			ParkedAt:     t.ParkedAt,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
			DueDate:      t.DueDate,
			HasTime:      t.HasTime,
		}
	}
	return entries
}

func displayTaskJSON(tasks []*models.Task) error {
	entries := taskListJSONEntries(tasks)
	return printJSON(entries)
}

// taskMatchesFilters returns true if the task passes all active CLI filters.
func taskMatchesFilters(task *models.Task) bool {
	if !listShowAll && task.IsTerminalState() {
		return false
	}
	if !listSnoozed && task.IsSnoozed() {
		return false
	}
	if listParked && !task.IsParked() {
		return false
	}
	if listTaskType != "" {
		t, ok := models.ParseTaskType(listTaskType)
		if !ok || task.Type != t {
			return false
		}
	}
	if listState != "" {
		s, ok := models.ParseWorkflowState(listState)
		if !ok || task.State() != s {
			return false
		}
	}
	if listBlocked && !task.IsDependencyBlocked(listFilterProject) {
		return false
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

// canonicalStateOrder follows models.AllWorkflowStates, the accepted public
// order used by canonical state input, output, and completion.
var canonicalStateOrder = func() map[models.WorkflowState]int {
	order := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	for i, state := range models.AllWorkflowStates() {
		order[state] = i
	}
	return order
}()

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
	case "state":
		sort.SliceStable(tasks, func(i, j int) bool {
			return canonicalStateOrder[tasks[i].State()] < canonicalStateOrder[tasks[j].State()]
		})
	case "title":
		sort.SliceStable(tasks, func(i, j int) bool {
			return strings.ToLower(tasks[i].Title) < strings.ToLower(tasks[j].Title)
		})
	default:
		return fmt.Errorf("unknown sort field %q (valid: priority, created, updated, state, title)", field)
	}
	return nil
}

func displayTaskTable(tasks []*models.Task) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"ID", "Type", "State", "Pri", "Title", "Tags"})

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
			task.State().Icon(),
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

	// Style based on workflow state
	style := lipgloss.NewStyle()
	switch task.State() {
	case models.StateDone, models.StateCanceled:
		style = style.Foreground(lipgloss.Color("242")).Strikethrough(true)
	case models.StateInProgress:
		style = style.Foreground(lipgloss.Color("82"))
	case models.StateReview:
		style = style.Foreground(lipgloss.Color("214"))
	case models.StatePaused, models.StateSomeday:
		style = style.Foreground(lipgloss.Color("141"))
	case models.StateWaiting:
		style = style.Foreground(lipgloss.Color("196"))
	default:
	}

	line := fmt.Sprintf("%s%s%s %s #%d %s",
		indent.String(),
		prefix,
		task.Type.Icon(),
		task.State().Icon(),
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
