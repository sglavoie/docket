package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui"
)

var taskCompleteCmd = &cobra.Command{
	Use:     "complete [id...]",
	Aliases: []string{"done", "c"},
	Short:   "Mark task(s) as complete",
	Long: `Mark one or more tasks as done.

If no ID is specified, opens an interactive selector.

Examples:
  tt task complete 5
  tt task complete 1 2 3
  tt task done 3
  tt t c`,
	RunE: runTaskComplete,
}

var taskStartCmd = &cobra.Command{
	Use:   "start [id...]",
	Short: "Mark task(s) as in progress",
	Long: `Mark one or more tasks as in progress.

Examples:
  tt task start 5
  tt task start 1 2 3`,
	RunE: runTaskStart,
}

var taskCancelCmd = &cobra.Command{
	Use:   "cancel [id...]",
	Short: "Cancel task(s)",
	Long: `Mark one or more tasks as canceled.

Examples:
  tt task cancel 5
  tt task cancel 1 2 3`,
	RunE: runTaskCancel,
}

var taskReopenCmd = &cobra.Command{
	Use:   "reopen [id...]",
	Short: "Reopen completed task(s)",
	Long: `Reopen one or more terminal tasks in Inbox for triage.

Examples:
  tt task reopen 5
  tt task reopen 1 2 3`,
	RunE: runTaskReopen,
}

var taskReviewCmd = &cobra.Command{
	Use:   "review [id...]",
	Short: "Send task(s) to review",
	Long: `Mark one or more tasks as in review (code review, QA, approval).

Examples:
  tt task review 5
  tt task review 1 2 3`,
	RunE: runTaskReview,
}

func init() {
	taskCmd.AddCommand(taskCompleteCmd)
	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskCancelCmd)
	taskCmd.AddCommand(taskReopenCmd)
	taskCmd.AddCommand(taskReviewCmd)

	for _, sub := range []*cobra.Command{
		taskCompleteCmd, taskStartCmd, taskCancelCmd, taskReopenCmd, taskReviewCmd,
	} {
		addOutputFlags(sub)
	}
}

func runTaskComplete(cmd *cobra.Command, args []string) error {
	return runStateCommand(cmd, args, "complete")
}
func runTaskStart(cmd *cobra.Command, args []string) error {
	return runStateCommand(cmd, args, "start")
}
func runTaskCancel(cmd *cobra.Command, args []string) error {
	return runStateCommand(cmd, args, "cancel")
}
func runTaskReopen(cmd *cobra.Command, args []string) error {
	return runStateCommand(cmd, args, "reopen")
}
func runTaskReview(cmd *cobra.Command, args []string) error {
	return runStateCommand(cmd, args, "review")
}

// actionToState maps a semantic CLI action name to its canonical workflow state.
var actionToState = map[string]models.WorkflowState{
	"complete": models.StateDone,
	"start":    models.StateInProgress,
	"cancel":   models.StateCanceled,
	"reopen":   models.StateInbox,
	"review":   models.StateReview,
}

// actionVerbs maps a CLI action to the past-tense verb shown in human output.
var actionVerbs = map[string]string{
	"complete": "Completed",
	"start":    "Started",
	"cancel":   "Canceled",
	"reopen":   "Reopened",
	"review":   "Set to review",
}

// runStateCommand routes a semantic command through the application layer's
// Transition use case. It preserves the UX — explicit references or an
// interactive selector, per-task error aggregation, and the recurring-instance
// notice on completion — while the mutation itself commits atomically,
// revision-checked, and undoably through app.Service. The workflow guards
// (unresolved dependencies on completion, incomplete children on any terminal
// transition) are enforced inside the use case, so no CLI-side pre-check remains.
func runStateCommand(cmd *cobra.Command, args []string, action string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	state := actionToState[action]
	verb := actionVerbs[action]
	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()

	var errs []string
	succeeded := 0

	transition := func(display, taskRef string) error {
		task, locator, err := resolveAppTaskTarget(workspace.Manager, taskRef)
		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", display, err))
			return nil
		}
		cascade, err := withResultRetry(func() (app.BulkResult, error) {
			return workflows.service.StateWithWorkflowByLocator(locator, state, workflows.options(true, "Change State", task))
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", display, translateAppError(err)))
			return nil
		}
		workflows.observe(cascade)
		res := workflowResultTask(cascade)
		succeeded++
		if err := emitMutation(out, verb, res); err != nil {
			return err
		}
		if action == "complete" && !out.Quiet && !out.JSON && res.RecurringGenerated > 0 {
			fmt.Printf("Created %s in Inbox\n", Pluralize(res.RecurringGenerated, "recurring task instance", "recurring task instances"))
		}
		return nil
	}

	// No references: open the interactive selector for a single target.
	if len(args) == 0 {
		taskID, err := selectStateTarget(action)
		if err != nil {
			return err
		}
		if err := transition(taskID, taskID); err != nil {
			return err
		}
		return ReportBatchErrors(succeeded, errs)
	}

	for _, ref := range args {
		if err := transition(ref, ref); err != nil {
			return err
		}
	}
	return ReportBatchErrors(succeeded, errs)
}

// selectStateTarget opens the interactive selector for a state command invoked
// with no explicit reference. It lists only tasks in the current project that
// can transition to the action's target state and returns the chosen task's
// project-qualified UUID.
func selectStateTarget(action string) (string, error) {
	project, _, err := LoadRuntimeCurrentProject()
	if err != nil {
		return "", err
	}

	targetState := actionToState[action]
	var candidates []*models.Task
	for _, t := range project.Tasks {
		if models.CanTransitionState(t.State(), targetState) == "" {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		if action == "reopen" {
			return "", fmt.Errorf("no completed tasks available to %s", action)
		}
		return "", fmt.Errorf("no active tasks available to %s", action)
	}

	items := TasksToSelectorItems(candidates)
	prompt := fmt.Sprintf("Select task to %s:", action)
	selected, err := tui.RunSelector(prompt, items)
	if err != nil {
		return "", err
	}
	task, err := GetTaskFromSelector(selected)
	if err != nil {
		return "", err
	}
	return project.ID + ":" + task.ID, nil
}

// countCrossProjectDeps counts the number of cross-project dependencies on a task.
func countCrossProjectDeps(task *models.Task) int {
	count := 0
	for _, depID := range task.DependsOn {
		if models.ParseTaskRef(depID).IsCrossProject() {
			count++
		}
	}
	return count
}
