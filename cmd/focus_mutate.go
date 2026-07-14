package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// Focus Queue and Daily Plan are planning collections persisted in config, not
// task workflow states. Their mutation commands operate purely on config through
// config.Manager.Save and never touch project storage, so they stay off the
// application-layer task-mutation boundary.

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

	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	for _, item := range planning.FocusQueue {
		if item.TaskID == task.ID {
			return emitFocusMutation(out, "already queued", task, project, len(planning.FocusQueue))
		}
	}
	planning.FocusQueue = append(planning.FocusQueue, config.FocusQueueItem{TaskID: task.ID, ProjectID: project.ID})
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, "queued", task, project, len(planning.FocusQueue))
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

	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	filtered := make([]config.FocusQueueItem, 0, len(planning.FocusQueue))
	removed := false
	for _, item := range planning.FocusQueue {
		if item.TaskID == task.ID {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return emitFocusMutation(out, "not in queue", task, project, len(planning.FocusQueue))
	}
	planning.FocusQueue = filtered
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, "dequeued", task, project, len(filtered))
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

	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	queue := planning.FocusQueue
	idx := -1
	for i, item := range queue {
		if item.TaskID == task.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return emitFocusMutation(out, "not in queue", task, project, len(queue))
	}

	// Clamp the requested 1-based position to the queue bounds.
	target := position - 1
	if target < 0 {
		target = 0
	}
	if target > len(queue)-1 {
		target = len(queue) - 1
	}

	moved := queue[idx]
	queue = append(queue[:idx], queue[idx+1:]...)
	queue = append(queue[:target], append([]config.FocusQueueItem{moved}, queue[target:]...)...)
	planning.FocusQueue = queue
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, fmt.Sprintf("moved to position %d", target+1), task, project, len(queue))
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

	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	for _, item := range planning.DailyPlan {
		if item.TaskID == task.ID {
			return emitFocusMutation(out, "already planned", task, project, len(planning.DailyPlan))
		}
	}
	planning.DailyPlan = append(planning.DailyPlan, config.DailyPlanItem{
		TaskID:      task.ID,
		ProjectID:   project.ID,
		PlannedDate: time.Now().Format("2006-01-02"),
	})
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, "planned", task, project, len(planning.DailyPlan))
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

	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	filtered := make([]config.DailyPlanItem, 0, len(planning.DailyPlan))
	removed := false
	for _, item := range planning.DailyPlan {
		if item.TaskID == task.ID {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return emitFocusMutation(out, "not planned", task, project, len(planning.DailyPlan))
	}
	planning.DailyPlan = filtered
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, "unplanned", task, project, len(filtered))
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
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	planning.DailyPlan = nil
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, "cleared today's plan", nil, nil, 0)
}
