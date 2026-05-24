package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

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

	var errs []string
	succeeded := 0
	var updated []*models.Task
	for _, taskID := range taskIDs {
		var oldPriority int

		alreadyAtPriority := false
		task, err := WithTaskUpdate(taskID, func(task *models.Task) error {
			oldPriority = task.Priority

			if oldPriority == priority {
				alreadyAtPriority = true
				return fmt.Errorf("already at priority %d", priority)
			}

			task.Priority = priority
			return nil
		})

		if err != nil {
			if alreadyAtPriority {
				// Treat "already at target priority" as a no-op success in both
				// single-task and batch modes; surfacing it as an error caused
				// non-zero exit codes for what is effectively idempotent input.
				if !out.Quiet && !out.JSON && len(taskIDs) == 1 {
					// FindTaskByIdentifier already validated the ID exists, so
					// re-look up the short ID for a friendlier message.
					if project, _, _, lookupErr := LoadCurrentProject(); lookupErr == nil {
						if t, findErr := FindTaskByIdentifier(project, taskID); findErr == nil {
							fmt.Printf("Task #%d is already at priority %d.\n", t.ShortID, priority)
						}
					}
				}
				continue
			}
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		updated = append(updated, task)
		if !out.Quiet && !out.JSON {
			fmt.Println(successMsg("Updated priority for task #%d: %d -> %d (%s)", task.ShortID, oldPriority, priority, task.Title))
		}
		succeeded++
	}

	if out.JSON {
		if updated == nil {
			updated = []*models.Task{}
		}
		if err := outputTasksJSON(updated); err != nil {
			return err
		}
		return ReportBatchErrors(succeeded, errs)
	}

	return ReportBatchErrors(succeeded, errs)
}
