package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// Focus Queue and Daily Plan mutations route through the application planning
// service so task state and planning membership commit atomically.

var (
	focusQueueAddCmd = &cobra.Command{
		Use:   "add <task-ref>",
		Short: "Add a task to the focus work queue",
		Long: `Add a task to the persisted focus work queue.

Use a numeric short ID within the current project, or a qualified
project-slug:#short-id reference to target a task in another project.

Examples:
  tt focus queue add 5
  tt focus queue add myproject:#5`,
		Args: cobra.ExactArgs(1),
		RunE: runFocusQueueAdd,
	}

	focusQueueRemoveCmd = &cobra.Command{
		Use:     "remove <task-ref>",
		Aliases: []string{"rm"},
		Short:   "Remove a task from the focus work queue",
		Args:    cobra.ExactArgs(1),
		RunE:    runFocusQueueRemove,
	}

	focusQueueMoveCmd = &cobra.Command{
		Use:   "move <task-ref> <position>",
		Short: "Move a task to a 1-based position in the focus queue",
		Long: `Move an already-queued task to a new 1-based position in the queue.

Positions are clamped to the queue bounds, so "1" always moves the task to
the front and a large number moves it to the end.

Examples:
  tt focus queue move 5 1
  tt focus queue move myproject:#5 3`,
		Args: cobra.ExactArgs(2),
		RunE: runFocusQueueMove,
	}

	focusTodayAddCmd = &cobra.Command{
		Use:   "add <task-ref>",
		Short: "Plan a task for today",
		Args:  cobra.ExactArgs(1),
		RunE:  runFocusTodayAdd,
	}

	focusTodayRemoveCmd = &cobra.Command{
		Use:     "remove <task-ref>",
		Aliases: []string{"rm"},
		Short:   "Remove a task from today's plan",
		Args:    cobra.ExactArgs(1),
		RunE:    runFocusTodayRemove,
	}

	focusTodayClearCmd = &cobra.Command{
		Use:   "clear",
		Short: "Clear all tasks from today's plan",
		Args:  cobra.NoArgs,
		RunE:  runFocusTodayClear,
	}
)

func init() {
	focusQueueCmd.AddCommand(focusQueueAddCmd)
	focusQueueCmd.AddCommand(focusQueueRemoveCmd)
	focusQueueCmd.AddCommand(focusQueueMoveCmd)
	for _, c := range []*cobra.Command{focusQueueAddCmd, focusQueueRemoveCmd, focusQueueMoveCmd} {
		addOutputFlags(c)
	}

	focusTodayCmd.AddCommand(focusTodayAddCmd)
	focusTodayCmd.AddCommand(focusTodayRemoveCmd)
	focusTodayCmd.AddCommand(focusTodayClearCmd)
	for _, c := range []*cobra.Command{focusTodayAddCmd, focusTodayRemoveCmd, focusTodayClearCmd} {
		addOutputFlags(c)
	}
}

// focusMutationResult is the JSON envelope emitted by focus planning mutations.
type focusMutationResult struct {
	Action  string `json:"action"`
	Task    string `json:"task,omitempty"`
	TaskID  string `json:"task_id,omitempty"`
	Project string `json:"project_id,omitempty"`
	Size    int    `json:"size"`
}

// resolveFocusTarget resolves a task reference across projects and returns the
// task together with its owning project, erroring when the reference matches no
// task so planning mutations never silently store a dangling reference.
func resolveFocusTarget(runtime *config.RuntimeState, ref string) (*models.Task, *models.Project, error) {
	task, project, err := FindTaskGloballyByID(runtime.AllLoadableProjects(), ref)
	if err != nil {
		return nil, nil, err
	}
	if task == nil || project == nil {
		return nil, nil, tterrors.NewTaskNotFoundError(ref)
	}
	return task, project, nil
}

func emitFocusMutation(out MutateOutput, action string, task *models.Task, project *models.Project, size int) error {
	if out.JSON {
		res := focusMutationResult{Action: action, Size: size}
		if task != nil && project != nil {
			res.Task = fmt.Sprintf("%s:#%d", project.Slug, task.ShortID)
			res.TaskID = task.ID
			res.Project = project.ID
		}
		return printJSON(res)
	}
	if out.Quiet {
		return nil
	}
	if task != nil && project != nil {
		fmt.Printf("%s %s:#%d: %s (%s)\n", action, project.Slug, task.ShortID, task.Title, Pluralize(size, "item", "items"))
	} else {
		fmt.Printf("%s (%s)\n", action, Pluralize(size, "item", "items"))
	}
	return nil
}

func runFocusQueueAdd(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	task, project, err := resolveFocusTarget(workspace.Runtime, args[0])
	if err != nil {
		return err
	}

	alreadyQueued := task.State() == models.StateNext &&
		focusQueueContains(workspace.Runtime.Planning(), task.ID, project.ID)
	if _, err := newWorkspaceAppService(workspace).ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningAddNext,
	}); err != nil {
		return translateAppError(err)
	}
	if err := workspace.Runtime.Reload(); err != nil {
		return err
	}
	action := "queued"
	if alreadyQueued {
		action = "already queued"
	}
	return emitFocusMutation(out, action, task, project, len(workspace.Runtime.Planning().FocusQueue))
}

func runFocusQueueRemove(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	task, project, err := resolveFocusTarget(workspace.Runtime, args[0])
	if err != nil {
		return err
	}

	wasNext := task.State() == models.StateNext
	inQueue := focusQueueContains(workspace.Runtime.Planning(), task.ID, project.ID)
	if !inQueue && !wasNext {
		return emitFocusMutation(out, "not in queue", task, project, len(workspace.Runtime.Planning().FocusQueue))
	}

	if _, err := newWorkspaceAppService(workspace).ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningRemoveNext,
	}); err != nil {
		return translateAppError(err)
	}
	if err := workspace.Runtime.Reload(); err != nil {
		return err
	}
	return emitFocusMutation(out, "dequeued", task, project, len(workspace.Runtime.Planning().FocusQueue))
}

// focusQueueContains reports whether the Next order index already lists the
// project-qualified task.
func focusQueueContains(planning *config.PlanningState, taskID, projectID string) bool {
	for _, item := range planning.FocusQueue {
		if item.TaskID == taskID && item.ProjectID == projectID {
			return true
		}
	}
	return false
}

// dailyPlanContains reports whether the Daily Plan already lists the
// project-qualified task.
func dailyPlanContains(planning *config.PlanningState, taskID, projectID string) bool {
	for _, item := range planning.DailyPlan {
		if item.TaskID == taskID && item.ProjectID == projectID {
			return true
		}
	}
	return false
}

func runFocusQueueMove(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	position, err := strconv.Atoi(strings.TrimSpace(args[1]))
	if err != nil || position < 1 {
		return fmt.Errorf("position must be a positive integer, got %q", args[1])
	}
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	task, project, err := resolveFocusTarget(workspace.Runtime, args[0])
	if err != nil {
		return err
	}

	// Next membership is the StateNext workflow state; the canonical order is the
	// shortlist nextTasks() produces (indexed members first, then unindexed
	// StateNext tasks). Reorder that shortlist rather than the raw order index, so
	// an unindexed StateNext task can be moved and a non-Next task cannot be moved
	// merely because a stale index entry lingers. Migrate legacy queue members to
	// StateNext before reading membership so a reorder never prunes an unmigrated
	// legacy queue (mirrors the `tt focus queue` listing path). The migration
	// commits its own workspace transaction, so clone the planning domain AFTER it
	// runs to build the reorder on the post-migration state.
	projects := loadActiveProjects(workspace.Runtime.ActiveProjects())
	if migrationErr := migrateLegacyFocusQueueMembers(workspace, workspace.Runtime.Planning().GetFocusQueue(), projects); migrationErr != nil {
		return migrationErr
	}

	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	ordered := nextTasks(projects, planning.GetFocusQueue())

	idx := -1
	for i, ft := range ordered {
		if ft.Task.ID == task.ID && ft.Project.ID == project.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		// The task is not StateNext (a stale index entry does not qualify it), so
		// there is nothing to reorder. The size reflects the StateNext shortlist.
		return emitFocusMutation(out, "not in queue", task, project, len(ordered))
	}

	// Clamp the requested 1-based position to the shortlist bounds.
	target := position - 1
	if target < 0 {
		target = 0
	}
	if target > len(ordered)-1 {
		target = len(ordered) - 1
	}

	// Reorder the canonical shortlist and persist it as the full order index, so
	// every StateNext task gains an explicit entry (previously-unindexed members
	// included) at its canonical position and stale non-Next entries are pruned.
	moved := ordered[idx]
	ordered = append(ordered[:idx], ordered[idx+1:]...)
	ordered = append(ordered[:target], append([]focus.FocusTask{moved}, ordered[target:]...)...)

	rebuilt := make([]config.FocusQueueItem, 0, len(ordered))
	for _, ft := range ordered {
		rebuilt = append(rebuilt, config.FocusQueueItem{TaskID: ft.Task.ID, ProjectID: ft.Project.ID})
	}
	order := make([]app.TaskLocator, 0, len(rebuilt))
	for _, item := range rebuilt {
		order = append(order, app.TaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID})
	}
	if err := newWorkspaceAppService(workspace).ReorderNext(order); err != nil {
		return err
	}
	return emitFocusMutation(out, fmt.Sprintf("moved to position %d", target+1), task, project, len(ordered))
}

func runFocusTodayAdd(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	task, project, err := resolveFocusTarget(workspace.Runtime, args[0])
	if err != nil {
		return err
	}

	alreadyPlanned := dailyPlanContains(workspace.Runtime.Planning(), task.ID, project.ID)
	if _, err := newWorkspaceAppService(workspace).ApplyPlanning(app.PlanningRequest{
		Locator:     app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:      app.PlanningAddToday,
		PlannedDate: time.Now().Format("2006-01-02"),
	}); err != nil {
		return translateAppError(err)
	}
	if err := workspace.Runtime.Reload(); err != nil {
		return err
	}
	action := "planned"
	if alreadyPlanned {
		action = "already planned"
	}
	return emitFocusMutation(out, action, task, project, len(workspace.Runtime.Planning().DailyPlan))
}

func runFocusTodayRemove(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	task, project, err := resolveFocusTarget(workspace.Runtime, args[0])
	if err != nil {
		return err
	}

	removed := dailyPlanContains(workspace.Runtime.Planning(), task.ID, project.ID)
	if !removed {
		return emitFocusMutation(out, "not planned", task, project, len(workspace.Runtime.Planning().DailyPlan))
	}
	if _, err := newWorkspaceAppService(workspace).ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Action:  app.PlanningRemoveToday,
	}); err != nil {
		return translateAppError(err)
	}
	if err := workspace.Runtime.Reload(); err != nil {
		return err
	}
	return emitFocusMutation(out, "unplanned", task, project, len(workspace.Runtime.Planning().DailyPlan))
}

func runFocusTodayClear(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	if err := newWorkspaceAppService(workspace).ClearToday(); err != nil {
		return err
	}
	return emitFocusMutation(out, "cleared today's plan", nil, nil, 0)
}
