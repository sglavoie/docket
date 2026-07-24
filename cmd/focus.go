package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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

Examples:
  tt focus            # all actionable tasks`,
	RunE: runFocus,
}

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
	Short: "Show the Outliner's Next shortlist (StateNext tasks)",
	Long: `Show the Outliner's "Next" on-deck shortlist across projects.

Next membership is the workflow state: a task is in Next exactly when its
state is "next" (StateNext). The persisted focus queue list is ordering-only
— it just orders the StateNext tasks (indexed members first, then any
unindexed StateNext task). Today (the Daily Plan) is a mutually exclusive
planning list, so a task is never in both Next and Today.

In the TUI, press Tab to cycle the Outliner's work modes. Press 'q' to toggle
the selected task's Next membership or 'w' to toggle Today membership. From
the CLI use 'tt focus queue add' (Next) or 'tt focus today add' (Today), and
reorder Next with 'tt focus queue move'.

Examples:
  tt focus queue`,
	RunE: runFocusQueue,
}

var focusTodayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show tasks planned for today",
	Long: `Show tasks that have been planned for today's work session.

Planned tasks appear in the built-in Today section at the top of the Outliner's
All mode. Press 'w' on a task to toggle Today membership. Unfinished tasks carry
over automatically to the next day.

Examples:
  tt focus today`,
	RunE: runFocusToday,
}

func init() {
	rootCmd.AddCommand(focusCmd)

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
	tasks := focus.CollectActionableTasks(projects)

	sortFocusTasks(tasks)

	if jsonRequested(cmd) {
		entries := make([]focusTaskJSON, 0, len(tasks))
		for _, ft := range tasks {
			entries = append(entries, focusTaskToJSON(ft.Task, ft.Project))
		}
		return printJSON(entries)
	}

	if len(tasks) == 0 {
		fmt.Println("No actionable tasks found.")
		fmt.Println("Tasks appear here from all active projects (excluding completed, snoozed, and blocked).")
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
	fmt.Printf("\n%s\n", Pluralize(len(tasks), "actionable task", "actionable tasks"))
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
// keyed by project ID and project-qualified task identity. Nil projects and nil
// tasks are skipped so downstream code can safely dereference the map values.
func buildProjectAndTaskMaps(projects []*models.Project) (map[string]*models.Project, map[config.ProjectTaskLocator]*models.Task) {
	projectMap := make(map[string]*models.Project)
	taskMap := make(map[config.ProjectTaskLocator]*models.Task)
	for _, p := range projects {
		if p == nil {
			continue
		}
		projectMap[p.ID] = p
		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			taskMap[config.ProjectTaskLocator{ProjectID: p.ID, TaskID: task.ID}] = task
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
			task, ok := taskMap[config.ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}]
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
		task, ok := taskMap[config.ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}]
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

// nextTasks gathers every StateNext task across the given projects, ordered by
// the Focus Queue index: indexed members first in queue order, then unindexed
// StateNext tasks (deterministically by priority then short ID). Membership is
// the workflow state, so unindexed StateNext tasks are included and stale
// indexed non-Next entries are excluded.
func nextTasks(projects []*models.Project, queue []config.FocusQueueItem) []focus.FocusTask {
	var tasks []focus.FocusTask
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.State() != models.StateNext {
				continue
			}
			tasks = append(tasks, focus.FocusTask{Task: task, Project: project})
		}
	}
	rank := focus.FocusQueueRank(queue)
	sort.SliceStable(tasks, func(i, j int) bool {
		ri := focus.RankOrUnindexed(rank, config.ProjectTaskLocator{ProjectID: tasks[i].Project.ID, TaskID: tasks[i].Task.ID})
		rj := focus.RankOrUnindexed(rank, config.ProjectTaskLocator{ProjectID: tasks[j].Project.ID, TaskID: tasks[j].Task.ID})
		if ri != rj {
			return ri < rj
		}
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.ShortID < tasks[j].Task.ShortID
	})
	return tasks
}

// migrateLegacyFocusQueueMembers backfills StateNext onto Focus Queue members
// that predate the state-driven queue before the ordering-only list is pruned, so
// the first `tt focus queue` invocation cannot delete an unmigrated legacy queue.
//
// The backfill commits through the application layer's BulkTransition, so every
// changed project lands in ONE revision-checked workspace transaction with a
// durable ledger entry — the same journaled, undoable boundary as an ordinary
// state change — rather than raw per-project SaveProjectLocked writes. The batch is
// all-or-nothing: on a commit failure nothing is written, so every project state
// and the FocusQueue index stay at their pre-migration values and the caller skips
// pruning (leaving the unmigrated queue intact for a later retry). On success the
// service committed against its own fresh snapshot, so the caller's in-memory
// projects are reloaded here to observe the migrated StateNext membership. Returns
// an error only when the transaction itself fails; a queue with no legacy members
// is a no-op.
func migrateLegacyFocusQueueMembers(workspace *CLIWorkspace, queue []config.FocusQueueItem, projects []*models.Project) error {
	locators := focus.LegacyNextQueueMemberLocators(queue, projects)
	if len(locators) == 0 {
		return nil
	}
	// A bare application service (no default planning-artifact builder) keeps the
	// migration a pure state backfill, matching `tt doctor`'s repair semantics: it
	// journals the StateNext transition without folding Today/Next reconciliation.
	svc := newAppService(workspace.Manager)
	if _, err := svc.BulkTransitionWithWorkflow(app.BulkTransitionRequest{
		TaskLocators: appTaskLocatorsFromConfig(locators),
		Target:       models.StateNext,
		UndoAction:   "Migrate legacy Next queue",
	}); err != nil {
		return translateAppError(err)
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to reload project %s after focus queue migration: %v\n", project.Name, err)
		}
	}
	return nil
}

// pruneStaleFocusQueue drops order-index entries whose task has left Next so the
// ordering-only list stays aligned with the state-driven membership. It is
// best-effort maintenance: a save failure is a warning, not a command failure.
func pruneStaleFocusQueue(workspace *CLIWorkspace, queue []config.FocusQueueItem, ordered []focus.FocusTask) {
	nextIDs := make(map[config.ProjectTaskLocator]bool, len(ordered))
	for _, ft := range ordered {
		nextIDs[config.ProjectTaskLocator{ProjectID: ft.Project.ID, TaskID: ft.Task.ID}] = true
	}
	pruned := make([]config.FocusQueueItem, 0, len(queue))
	for _, item := range queue {
		if nextIDs[config.ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}] {
			pruned = append(pruned, item)
		}
	}
	if len(pruned) == len(queue) {
		return
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to prune focus queue: %v\n", err)
		return
	}
	domains.Planning.SetFocusQueue(pruned)
	if err := commitCLIPlanning(workspace, domains.Planning); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save pruned focus queue: %v\n", err)
	}
}

func runFocusQueue(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}

	wantJSON := jsonRequested(cmd)
	queue := workspace.Runtime.Planning().GetFocusQueue()
	projects := loadActiveProjects(workspace.Runtime.ActiveProjects())

	// Migrate legacy (pre-state-driven) queue members to StateNext BEFORE pruning,
	// so a first listing can never delete an unmigrated queue that is the only
	// evidence of its members.
	migrationErr := migrateLegacyFocusQueueMembers(workspace, queue, projects)
	if migrationErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to migrate legacy focus queue members: %v\n", migrationErr)
	}
	ordered := nextTasks(projects, queue)

	// Keep the ordering-only index aligned with state-driven membership — but only
	// after a successful migration. A failed migration must leave the index at its
	// pre-migration value; pruning here would drop the still-legacy members (they
	// read as non-Next) exactly when their only evidence is the queue entry.
	if migrationErr == nil {
		pruneStaleFocusQueue(workspace, queue, ordered)
	}

	if wantJSON {
		entries := make([]focusTaskJSON, 0, len(ordered))
		for _, ft := range ordered {
			entries = append(entries, focusTaskToJSON(ft.Task, ft.Project))
		}
		return printJSON(entries)
	}

	if len(ordered) == 0 {
		fmt.Println("Focus queue is empty.")
		fmt.Println("Move a task to Next (press 'q' in the TUI, or `tt focus queue add`) to build your queue.")
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"#", "Project", "ID", "Type", "Status", "Pri", "Title", "Due"})

	for i, ft := range ordered {
		task := ft.Task
		t.AppendRow(table.Row{
			i + 1,
			util.Truncate(ft.Project.Name, 15),
			task.ShortID,
			task.Type.Icon(),
			task.State().Icon(),
			task.Priority,
			util.Truncate(task.Title, 40),
			focus.FormatRelativeDue(task.DueDate),
		})
	}

	fmt.Println(t.Render())
	fmt.Printf("\n%s in queue\n", Pluralize(len(ordered), "task", "tasks"))
	return nil
}
