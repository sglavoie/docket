package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var taskPriorityCmd = &cobra.Command{
	Use:   "priority <task-id>... <priority>",
	Short: "Set task priority",
	Long: `Set the priority of one or more tasks. The last argument is the priority value.

Priority levels:
  1 - Critical/Urgent
  2 - High
  3 - Medium (default)
  4 - Low
  5 - Minimal

Examples:
  tt task priority 5 1       # Set task #5 to highest priority
  tt task priority 1 2 3 2   # Set tasks #1, #2, #3 to high priority`,
	Args: cobra.MinimumNArgs(2),
	RunE: runTaskPriority,
}

func init() {
	taskCmd.AddCommand(taskPriorityCmd)
}

func runTaskPriority(cmd *cobra.Command, args []string) error {
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
	skipped := 0
	for _, taskID := range taskIDs {
		var oldPriority int

		task, err := WithTaskUpdate(taskID, func(task *models.Task) error {
			oldPriority = task.Priority

			if oldPriority == priority {
				return nil
			}

			task.Priority = priority
			return nil
		})

		if err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %s", taskID, err))
			continue
		}

		if oldPriority != priority {
			fmt.Printf("Updated priority for task #%d: %d -> %d (%s)\n", task.ShortID, oldPriority, priority, task.Title)
			succeeded++
		} else {
			skipped++
		}
	}

	if skipped > 0 {
		fmt.Printf("%d updated, %d already at priority %d\n", succeeded, skipped, priority)
	}

	if len(errs) > 0 {
		if succeeded > 0 {
			// Partial success: print failures as warnings but exit 0
			fmt.Printf("\n%d succeeded, %d failed:\n", succeeded, len(errs))
			for _, e := range errs {
				fmt.Printf("  %s\n", e)
			}
			return nil
		}
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
