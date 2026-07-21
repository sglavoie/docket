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
	"github.com/sglavoie/tt/internal/storage"
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

	// Queue membership is the StateNext workflow state. When the task is not yet
	// Next this is a Today->Next move: transition to StateNext and fold the
	// exclusivity drop (leave Today) plus the Next order append into one ledger
	// transaction, so state and planning commit and undo atomically.
	if task.State() != models.StateNext {
		projectID := project.ID
		if transitionErr := setFocusQueueState(workspace, task, models.StateNext, "Move to Next", func(p *config.PlanningState) {
			if !focusQueueContains(p, task.ID) {
				p.SetFocusQueue(append(p.GetFocusQueue(), config.FocusQueueItem{TaskID: task.ID, ProjectID: projectID}))
			}
		}); transitionErr != nil {
			return transitionErr
		}
		return emitFocusMutation(out, "queued", task, project, len(workspace.Runtime.Planning().FocusQueue))
	}

	// Already Next: no state move to be atomic with — record ordering (and any
	// stale Today drop) through a standalone planning commit.
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	dropped := planning.RemoveFromDailyPlan(task.ID)
	alreadyQueued := focusQueueContains(planning, task.ID)
	if !alreadyQueued {
		planning.FocusQueue = append(planning.FocusQueue, config.FocusQueueItem{TaskID: task.ID, ProjectID: project.ID})
	}
	if !alreadyQueued || dropped {
		if err := commitCLIPlanning(workspace, planning); err != nil {
			return err
		}
	}
	action := "queued"
	if alreadyQueued {
		action = "already queued"
	}
	return emitFocusMutation(out, action, task, project, len(planning.FocusQueue))
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
	inQueue := focusQueueContains(workspace.Runtime.Planning(), task.ID)
	if !inQueue && !wasNext {
		return emitFocusMutation(out, "not in queue", task, project, len(workspace.Runtime.Planning().FocusQueue))
	}

	// Leaving Next is a Next->none move: transition to StateReady and fold the
	// order-entry drop (Today/Next exclusivity for the non-Next result) into the
	// same ledger transaction.
	if wasNext {
		if transitionErr := setFocusQueueState(workspace, task, models.StateReady, "Leave Next", nil); transitionErr != nil {
			return transitionErr
		}
		return emitFocusMutation(out, "dequeued", task, project, len(workspace.Runtime.Planning().FocusQueue))
	}

	// Not Next but has a stale order entry: pure planning cleanup.
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	planning.RemoveFromFocusQueue(task.ID)
	if err := commitCLIPlanning(workspace, planning); err != nil {
		return err
	}
	return emitFocusMutation(out, "dequeued", task, project, len(planning.FocusQueue))
}

// setFocusQueueState moves a task to target through the workflow-aware
// application transaction (one ledger-undoable ChangeSet), folding the planning
// change the move implies (Today/Next exclusivity plus the caller's additive
// order/plan edit) into the same transaction so state and planning commit — and
// undo — together. It refreshes the workspace runtime so callers read
// post-mutation planning state. A nil planningMutate folds only the exclusivity
// reconciliation.
func setFocusQueueState(workspace *CLIWorkspace, task *models.Task, target models.WorkflowState, undoAction string, planningMutate func(*config.PlanningState)) error {
	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	opts := workflows.options(true, undoAction, task)
	opts.PlanningArtifacts = focusPlanningArtifactBuilder(workspace, planningMutate)
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		return workflows.service.StateWithWorkflow(task.ID, target, opts)
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	return nil
}

// focusPlanningArtifactBuilder renders the planning.json artifact that rides a
// focus-command state move: it loads the current planning, applies Today/Next
// exclusivity for the transitioned task, then applies the caller's additive edit
// (append a Next order entry, add a Daily Plan entry) on top, and returns one
// revision-checked artifact for the transition's transaction.
func focusPlanningArtifactBuilder(workspace *CLIWorkspace, extra func(*config.PlanningState)) app.PlanningArtifactBuilder {
	if workspace == nil || workspace.Manager == nil {
		return nil
	}
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coord := workspace.Manager.DomainCoordinator()
		planning, err := coord.LoadPlanning()
		if err != nil {
			return nil, err
		}
		reconcilePlanningExclusivity(planning, targets)
		if extra != nil {
			extra(planning)
		}
		artifact, err := coord.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}

// focusQueueContains reports whether the Next order index already lists taskID.
func focusQueueContains(planning *config.PlanningState, taskID string) bool {
	for _, item := range planning.FocusQueue {
		if item.TaskID == taskID {
			return true
		}
	}
	return false
}

// dailyPlanContains reports whether the Daily Plan already lists taskID.
func dailyPlanContains(planning *config.PlanningState, taskID string) bool {
	for _, item := range planning.DailyPlan {
		if item.TaskID == taskID {
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
		if ft.Task.ID == task.ID {
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
	planning.SetFocusQueue(rebuilt)
	if err := commitCLIPlanning(workspace, planning); err != nil {
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

	// Mutual exclusivity: Today and Next are exclusive horizons, so a task planned
	// for Today must leave Next. When the task is StateNext this is a Next->Today
	// move: transition out through the ledger and fold both the order-entry drop
	// (exclusivity for the non-Next result) and the Daily Plan add into one
	// transaction so state and planning commit and undo atomically.
	wasNext := task.State() == models.StateNext
	plannedDate := time.Now().Format("2006-01-02")
	projectID := project.ID
	addPlan := func(p *config.PlanningState) {
		if !dailyPlanContains(p, task.ID) {
			p.SetDailyPlan(append(p.GetDailyPlan(), config.DailyPlanItem{
				TaskID:      task.ID,
				ProjectID:   projectID,
				PlannedDate: plannedDate,
			}))
		}
	}
	if wasNext {
		if transitionErr := setFocusQueueState(workspace, task, models.StateReady, "Plan for Today", addPlan); transitionErr != nil {
			return transitionErr
		}
		return emitFocusMutation(out, "planned", task, project, len(workspace.Runtime.Planning().DailyPlan))
	}

	// Not Next: pure planning add (no state move to be atomic with).
	if dailyPlanContains(workspace.Runtime.Planning(), task.ID) {
		return emitFocusMutation(out, "already planned", task, project, len(workspace.Runtime.Planning().DailyPlan))
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	planning := domains.Planning
	addPlan(planning)
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
