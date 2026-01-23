package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskPriorityCmd = &cobra.Command{
	Use:   "priority <task-id> <priority>",
	Short: "Set task priority (1=highest, 5=lowest)",
	Long: `Set the priority of a task.

Priority levels:
  1 - Critical/Urgent
  2 - High
  3 - Medium (default)
  4 - Low
  5 - Minimal

Examples:
  tt task priority 5 1     # Set task #5 to highest priority
  tt task priority 12 3    # Set task #12 to medium priority`,
	Args: cobra.ExactArgs(2),
	RunE: runTaskPriority,
}

func init() {
	taskCmd.AddCommand(taskPriorityCmd)
}

func runTaskPriority(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	priority, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid priority: %s (must be %d-%d)", args[1], models.PriorityMin, models.PriorityMax)
	}

	if priority < models.PriorityMin || priority > models.PriorityMax {
		return fmt.Errorf("priority must be between %d and %d, got %d", models.PriorityMin, models.PriorityMax, priority)
	}

	var oldPriority int

	task, err := WithTaskUpdate(taskID, func(task *models.Task) error {
		oldPriority = task.Priority

		if oldPriority == priority {
			fmt.Printf("Task #%d already has priority %d\n", task.ShortID, priority)
			return nil
		}

		task.Priority = priority
		return nil
	})

	if err != nil {
		return err
	}

	if oldPriority != priority {
		fmt.Printf("Updated priority for task #%d: %d -> %d (%s)\n", task.ShortID, oldPriority, priority, task.Title)
	}

	return nil
}
