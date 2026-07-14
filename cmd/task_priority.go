package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var taskPriorityCmd = &cobra.Command{
	Use:   "priority <task-id>... <priority>",
	Short: "Set task priority",
	Long: `Set the priority of one or more tasks. The last argument is the priority value.

The last argument is always the priority value; all preceding arguments are task IDs.

Priority levels:
  1 - Critical/Urgent
  2 - High
  3 - Medium (default)
  4 - Low
  5 - Minimal

Examples:
  tt task priority 5 1       # Set task #5 to highest priority
  tt task priority 1 2 3 2   # Set tasks #1, #2, #3 to high priority`,
	RunE: runTaskPriority,
}

func init() {
	taskCmd.AddCommand(taskPriorityCmd)
	addOutputFlags(taskPriorityCmd)
	taskPriorityCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 1 {
			return []string{"1", "2", "3", "4", "5"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTaskPriority(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and priority")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Last argument is the priority value, everything before is task IDs
	taskIDs := args[:len(args)-1]
	priorityArg := args[len(args)-1]

	priority, err := strconv.Atoi(priorityArg)
	if err != nil {
		return tterrors.NewInvalidValueError("priority", priorityArg, fmt.Sprintf("must be %d-%d", models.PriorityMin, models.PriorityMax))
	}

	if errValidate := ValidatePriority(priority, false); errValidate != nil {
		return errValidate
	}

	// Resolve every referenced task within the current project, then route each
	// priority change through the application service so it commits atomically,
	// revision-checked, and undoably (via the workspace ledger) with stale-revision
	// retry. Batch semantics are preserved: per-task errors are aggregated and
	// reported as partial success, matching the legacy loop.
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}
	svc := newWorkspaceAppService(workspace)

	var errs []string
	succeeded := 0
	for _, taskID := range taskIDs {
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, findErr))
			continue
		}

		oldPriority := task.Priority
		if oldPriority == priority {
			// Treat "already at target priority" as a no-op success: skip the commit
			// entirely so UpdatedAt is untouched, and surface a friendly line only in
			// single-task human mode (matching the legacy idempotent behavior).
			if !out.Quiet && !out.JSON && len(taskIDs) == 1 {
				fmt.Printf("Task #%d is already at priority %d.\n", task.ShortID, priority)
			}
			continue
		}

		res, mutErr := withTaskRetry(func() (app.TaskResult, error) {
			return svc.SetPriority(task.ID, priority)
		})
		if mutErr != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, translateAppError(mutErr)))
			continue
		}

		succeeded++
		if out.JSON {
			if err := emitMutation(out, "", res); err != nil {
				return err
			}
		} else if !out.Quiet {
			fmt.Println(successMsg("Updated priority for task #%d: %d -> %d (%s)", task.ShortID, oldPriority, priority, res.Title))
		}
	}

	return ReportBatchErrors(succeeded, errs)
}
