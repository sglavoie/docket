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
	Long: `Show tasks ranked by the "Backlog" suggestion engine.

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

var focusTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show tasks planned for today",
	Long: `Show tasks that have been planned for today's work session.

Plan work from the Today cockpit or the command palette's Today commands.
Unfinished tasks carry over automatically to the next day.

Examples:
  tt focus today`,
	RunE: runFocusToday,
}

func init() {
	rootCmd.AddCommand(focusCmd)
	focusCmd.Flags().BoolVar(&focusActiveOnly, "active", false, "Show only active-flagged tasks")

	focusCmd.AddCommand(focusSuggestCmd)
	focusSuggestCmd.Flags().IntVarP(&focusSuggestLimit, "number", "n", 10, "Number of suggestions to show")

	focusCmd.AddCommand(focusQueueCmd)
	focusCmd.AddCommand(focusTodayCmd)

	// Read commands support a consistent --json envelope for scripting.
	focusCmd.Flags().Bool("json", false, "output result as JSON")
	focusSuggestCmd.Flags().Bool("json", false, "output result as JSON")
	focusQueueCmd.Flags().Bool("json", false, "output result as JSON")
	focusTodayCmd.Flags().Bool("json", false, "output result as JSON")
}

// focusTaskJSON is the JSON representation of a task in a focus/planning listing.
type focusTaskJSON struct {
	Project   string `json:"project"`
	ProjectID string `json:"project_id"`
	ShortID   int    `json:"short_id"`
	Ref       string `json:"ref"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	State     string `json:"state"`
	Priority  int    `json:"priority"`
	Title     string `json:"title"`
	Due       string `json:"due,omitempty"`
}

func focusTaskToJSON(task *models.Task, project *models.Project) focusTaskJSON {
	entry := focusTaskJSON{
		Project:   project.Name,
		ProjectID: project.ID,
		ShortID:   task.ShortID,
		Ref:       fmt.Sprintf("%s:#%d", project.Slug, task.ShortID),
		Type:      string(task.Type),
		Status:    string(models.LegacyStatusForState(task.State())),
		State:     string(task.State()),
		Priority:  task.Priority,
		Title:     task.Title,
	}
	if task.DueDate != nil {
		entry.Due = task.DueDate.Format("2006-01-02")
	}
	return entry
}

func jsonRequested(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	v, _ := cmd.Flags().GetBool("json")
	return v
}

func runFocus(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}

	// Load and collect tasks from all active projects
	projects := loadActiveProjects(workspace.Runtime.ActiveProjects())
	var tasks []focus.FocusTask

	if focusActiveOnly {
		// Legacy flag: now backed by Focus Queue membership (the planning
		// collection that replaced the per-task Active flag).
		fmt.Fprintf(os.Stderr, "warning: --active is deprecated; run 'tt focus' without flags to see all actionable tasks, or 'tt focus queue' for the focus queue\n")
		planning := workspace.Runtime.Planning()
		tasks = collectFocusQueueTasks(planning.FocusQueue, projects)
	} else {
		tasks = focus.CollectActionableTasks(projects)
	}

	sortFocusTasks(tasks)

	if jsonRequested(cmd) {
		entries := make([]focusTaskJSON, 0, len(tasks))
		for _, ft := range tasks {
			entries = append(entries, focusTaskToJSON(ft.Task, ft.Project))
		}
		return printJSON(entries)
	}

	if len(tasks) == 0 {
		if focusActiveOnly {
			fmt.Println("No tasks in focus.")
			fmt.Println("Accept suggestions in the TUI focus view (press 'a') to build your queue.")
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
			task.State().Icon(),
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

// collectFocusQueueTasks returns non-terminal tasks that are members of the
// persisted Focus Queue planning collection. It backs the deprecated
// `tt focus --active` flag, which historically read the per-task Active flag;
// Focus Queue membership is that flag's canonical replacement. The queue is read
// from the planning domain by the caller and passed in. Nil projects and nil
// tasks are skipped so callers can pass loosely-assembled slices without
// crashing on stray nil entries.
func collectFocusQueueTasks(queue []config.FocusQueueItem, projects []*models.Project) []focus.FocusTask {
	queued := make(map[string]bool)
	for _, item := range queue {
		queued[item.TaskID] = true
	}
	var tasks []focus.FocusTask
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if task.IsTerminalState() || !queued[task.ID] {
				continue
			}
			tasks = append(tasks, focus.FocusTask{Task: task, Project: project})
		}
	}
	return tasks
}

// sortFocusTasks sorts tasks by priority ascending (1=highest), then by
// UpdatedAt descending, and finally by Task.ID as a deterministic tiebreaker
// so the order does not flicker between invocations when prior keys are equal.
func sortFocusTasks(tasks []focus.FocusTask) {
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		if !tasks[i].Task.UpdatedAt.Equal(tasks[j].Task.UpdatedAt) {
			return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
		}
		return tasks[i].Task.ID < tasks[j].Task.ID
	})
}

// buildProjectAndTaskMaps walks the given projects and returns lookup maps
// keyed by project ID and task ID. Nil projects and nil tasks are skipped so
// downstream code can safely dereference the map values.
func buildProjectAndTaskMaps(projects []*models.Project) (map[string]*models.Project, map[string]*models.Task) {
	projectMap := make(map[string]*models.Project)
	taskMap := make(map[string]*models.Task)
	for _, p := range projects {
		if p == nil {
			continue
		}
		projectMap[p.ID] = p
		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			taskMap[task.ID] = task
		}
	}
	return projectMap, taskMap
}

// loadActiveProjects loads task data for all active projects, returning
// only those that loaded successfully.
func loadActiveProjects(projects []*models.Project) []*models.Project {
	var loaded []*models.Project
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load project %q: %v\n", project.Slug, err)
			continue
		}
		loaded = append(loaded, project)
	}
	return loaded
}

func runFocusSuggest(cmd *cobra.Command, args []string) error {
	if focusSuggestLimit < 1 {
		return fmt.Errorf("number of suggestions must be at least 1")
	}

	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}

	tasks := focus.CollectActionableTasks(loadActiveProjects(workspace.Runtime.ActiveProjects()))
	if len(tasks) == 0 {
		fmt.Println("No actionable tasks found.")
		return nil
	}

	ranked := focus.RankCrossProjectTasks(tasks, nil, workspace.Runtime)
	if len(ranked) == 0 {
		fmt.Println("No suggestions available.")
		return nil
	}

	limit := min(focusSuggestLimit, len(ranked))

	if jsonRequested(cmd) {
		type suggestionJSON struct {
			focusTaskJSON
			Rank   int    `json:"rank"`
			Score  int    `json:"score"`
			Reason string `json:"reason"`
		}
		entries := make([]suggestionJSON, 0, limit)
		for i, sft := range ranked[:limit] {
			entries = append(entries, suggestionJSON{
				focusTaskJSON: focusTaskToJSON(sft.Task, sft.Project),
				Rank:          i + 1,
				Score:         sft.Score,
				Reason:        sft.Reason,
			})
		}
		return printJSON(entries)
	}

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

func runFocusToday(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}

	planning := workspace.Runtime.Planning()
	planItems := planning.DailyPlan
	wantJSON := jsonRequested(cmd)
	if len(planItems) == 0 {
		if wantJSON {
			return printJSON([]focusTaskJSON{})
		}
		fmt.Println("No tasks planned for today.")
		fmt.Println("Press 'w' on any task in the TUI to plan it for today.")
		return nil
	}

	projectMap, taskMap := buildProjectAndTaskMaps(loadActiveProjects(workspace.Runtime.ActiveProjects()))

	if wantJSON {
		entries := make([]focusTaskJSON, 0, len(planItems))
		for _, item := range planItems {
			project, ok := projectMap[item.ProjectID]
			if !ok {
				continue
			}
			task, ok := taskMap[item.TaskID]
			if !ok {
				continue
			}
			entries = append(entries, focusTaskToJSON(task, project))
		}
		return printJSON(entries)
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"#", "Project", "ID", "Type", "Status", "Pri", "Title", "Due"})

	count := 0
	for _, item := range planItems {
		project, ok := projectMap[item.ProjectID]
		if !ok {
			continue
		}
		task, ok := taskMap[item.TaskID]
		if !ok {
			continue
		}

		count++
		dueStr := focus.FormatRelativeDue(task.DueDate)
		t.AppendRow(table.Row{
			count,
			util.Truncate(project.Name, 15),
			task.ShortID,
			task.Type.Icon(),
			task.State().Icon(),
			task.Priority,
			util.Truncate(task.Title, 40),
			dueStr,
		})
	}

	if count == 0 {
		fmt.Println("No tasks planned for today (all planned tasks belong to inactive or missing projects).")
		return nil
	}

	fmt.Println(t.Render())
	fmt.Printf("\n%s planned for today\n", Pluralize(count, "task", "tasks"))
	return nil
}

func runFocusQueue(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}

	planning := workspace.Runtime.Planning()
	queueItems := planning.FocusQueue
	wantJSON := jsonRequested(cmd)
	if len(queueItems) == 0 {
		if wantJSON {
			return printJSON([]focusTaskJSON{})
		}
		fmt.Println("Focus queue is empty.")
		fmt.Println("Accept suggestions in the TUI focus view (press 'a') to build your queue.")
		return nil
	}

	projectMap, taskMap := buildProjectAndTaskMaps(loadActiveProjects(workspace.Runtime.ActiveProjects()))

	if wantJSON {
		entries := make([]focusTaskJSON, 0, len(queueItems))
		for _, item := range queueItems {
			project, ok := projectMap[item.ProjectID]
			if !ok {
				continue
			}
			task, ok := taskMap[item.TaskID]
			if !ok || task.IsTerminalState() {
				continue
			}
			entries = append(entries, focusTaskToJSON(task, project))
		}
		return printJSON(entries)
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
		if !ok || task.IsTerminalState() {
			continue
		}

		activeItems = append(activeItems, item)
		dueStr := focus.FormatRelativeDue(task.DueDate)

		t.AppendRow(table.Row{
			len(activeItems),
			util.Truncate(project.Name, 15),
			task.ShortID,
			task.Type.Icon(),
			task.State().Icon(),
			task.Priority,
			util.Truncate(task.Title, 40),
			dueStr,
		})
	}

	// Prune stale entries from the planning domain if any were skipped
	if len(activeItems) < len(queueItems) {
		domains, cloneErr := cloneCLIWorkspaceDomains(workspace)
		if cloneErr != nil {
			return cloneErr
		}
		domains.Planning.FocusQueue = activeItems
		if err := commitCLIPlanning(workspace, domains.Planning); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save pruned focus queue: %v\n", err)
		}
	}

	if len(activeItems) == 0 {
		fmt.Println("Focus queue is empty (all queued tasks have been completed).")
		return nil
	}

	fmt.Println(t.Render())
	fmt.Printf("\n%s in queue\n", Pluralize(len(activeItems), "task", "tasks"))
	return nil
}
