package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// taskStateCmd is the canonical workflow-state command: it moves one or more
// tasks to an explicit target state, routing through the application layer's
// Transition use case rather than mutating status/section/stage directly. It is
// the general form the state-convenience commands (ready, next, start, wait,
// pause, review, someday, complete, cancel, reopen) specialize.
var taskStateCmd = &cobra.Command{
	Use:   "state <state> <ref...>",
	Short: "Move task(s) to a workflow state",
	Long: `Move one or more tasks to an explicit workflow state.

Valid states: inbox, ready, next, in_progress, waiting, paused, review,
someday, done, canceled.

Task references use the strict qualified rules: a bare numeric short ID is
accepted only when it is unique in the current workspace; cross-project targets
must be qualified as project-slug:#short-id (or an unambiguous UUID prefix).

Examples:
  tt task state in_progress 5
  tt task state waiting web:#12
  tt task state ready 1 2 3`,
	Args:              cobra.MinimumNArgs(2),
	RunE:              runTaskState,
	ValidArgsFunction: completeTaskStateArgs,
}

func runTaskState(cmd *cobra.Command, args []string) error {
	target, ok := models.ParseWorkflowState(args[0])
	if !ok {
		if replacement, retired := retiredStateReplacement(args[0]); retired {
			return fmt.Errorf("workflow state %q was removed; use %s", args[0], replacement)
		}
		return fmt.Errorf("invalid workflow state %q (valid: %s)", args[0], joinStates())
	}
	return runAppTransition(cmd, args[1:], target)
}

// stateConvenience defines a convenience subcommand that transitions tasks to a
// fixed target state through the application layer.
type stateConvenience struct {
	use    string
	short  string
	verb   string
	target models.WorkflowState
}

// stateConveniences returns fixed-target commands for the most common canonical
// workflow transitions.
func stateConveniences() []stateConvenience {
	return []stateConvenience{
		{use: "ready [ref...]", short: "Mark task(s) as ready", verb: "Ready", target: models.StateReady},
		{use: "next [ref...]", short: "Move task(s) to Next", verb: "Next", target: models.StateNext},
		{use: "wait [ref...]", short: "Mark task(s) as waiting", verb: "Waiting", target: models.StateWaiting},
		{use: "pause [ref...]", short: "Pause task(s)", verb: "Paused", target: models.StatePaused},
		{use: "someday [ref...]", short: "Mark task(s) as someday", verb: "Someday", target: models.StateSomeday},
	}
}

// runAppTransition transitions each referenced task to target through the
// application service, applying the strict qualified-reference rules at the
// adapter edge and emitting the standardized mutation result. It aggregates
// per-task errors so a batch reports partial success like the legacy path.
func runAppTransition(cmd *cobra.Command, refs []string, target models.WorkflowState) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()

	verb := capitalizeState(string(target))
	var errs []string
	succeeded := 0
	for _, ref := range refs {
		task, locator, err := resolveAppTaskTarget(workspace.Manager, ref)
		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", ref, err))
			continue
		}
		cascade, err := withResultRetry(func() (app.BulkResult, error) {
			return workflows.service.StateWithWorkflowByLocator(locator, target, workflows.options(true, "Change State", task))
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", ref, translateAppError(err)))
			continue
		}
		workflows.observe(cascade)
		res := workflowResultTask(cascade)
		succeeded++
		if err := emitMutation(out, verb, res); err != nil {
			return err
		}
	}
	return ReportBatchErrors(succeeded, errs)
}

// capitalizeState renders a canonical state name as a display verb (e.g.
// "in_progress" -> "In progress").
func capitalizeState(state string) string {
	if state == "" {
		return state
	}
	return strings.ToUpper(state[:1]) + strings.ReplaceAll(state[1:], "_", " ")
}

func joinStates() string {
	names := make([]string, 0, len(models.AllWorkflowStates()))
	for _, s := range models.AllWorkflowStates() {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}

// completeTaskStateArgs completes the canonical transition target first, then
// task references from the current project. The first positional completion is
// intentionally state-only.
func completeTaskStateArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return stateCompletions, cobra.ShellCompDirectiveNoFileComp
	}
	return completeCurrentProjectTaskIDs()(cmd, args[1:], toComplete)
}

func init() {
	taskCmd.AddCommand(taskStateCmd)
	addOutputFlags(taskStateCmd)

	for _, c := range stateConveniences() {
		sc := c
		sub := &cobra.Command{
			Use:   sc.use,
			Short: sc.short,
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runAppTransition(cmd, args, sc.target)
			},
		}
		addOutputFlags(sub)
		sub.ValidArgsFunction = completeCurrentProjectTaskIDs()
		taskCmd.AddCommand(sub)
	}
}
