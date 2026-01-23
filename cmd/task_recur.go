package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskRecurClear bool

var taskRecurCmd = &cobra.Command{
	Use:   "recur <task-id> [pattern]",
	Short: "Set or clear task recurrence",
	Long: `Set or clear the recurrence pattern for a task.

When a recurring task is completed, the system automatically generates
the next instance with an updated due date.

Recurrence patterns:
  daily              - Every day
  weekly             - Every week
  biweekly           - Every 2 weeks
  monthly            - Every month
  yearly             - Every year
  Nd                 - Every N days (e.g., 3d = every 3 days)
  Nw                 - Every N weeks (e.g., 2w = every 2 weeks)
  Nm                 - Every N months (e.g., 6m = every 6 months)
  Ny                 - Every N years (e.g., 2y = every 2 years)
  weekly:mon,wed,fri - Weekly on specific days

Examples:
  tt task recur 5 weekly        # Task #5 recurs weekly
  tt task recur 12 2w           # Task #12 recurs every 2 weeks
  tt task recur 8 monthly       # Task #8 recurs monthly
  tt task recur 3 weekly:mon,fri # Task #3 recurs on Mondays and Fridays
  tt task recur 5 --clear       # Remove recurrence from task #5`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTaskRecur,
}

func init() {
	taskCmd.AddCommand(taskRecurCmd)
	taskRecurCmd.Flags().BoolVar(&taskRecurClear, "clear", false, "Remove recurrence pattern")
}

func runTaskRecur(cmd *cobra.Command, args []string) error {
	taskID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := project.GetTaskByShortID(taskID)
	if task == nil {
		return fmt.Errorf("task #%d not found", taskID)
	}

	// Handle --clear flag
	if taskRecurClear {
		if task.Recurrence == nil {
			fmt.Printf("Task #%d has no recurrence pattern\n", taskID)
			return nil
		}
		task.ClearRecurrence()
		if err := SaveProjectWithBackup(project); err != nil {
			return fmt.Errorf("failed to save project: %w", err)
		}
		fmt.Printf("Removed recurrence from task #%d (%s)\n", taskID, task.Title)
		return nil
	}

	// Require pattern argument when not clearing
	if len(args) < 2 {
		if task.Recurrence != nil {
			fmt.Printf("Task #%d recurs %s\n", taskID, task.Recurrence.String())
		} else {
			fmt.Printf("Task #%d has no recurrence pattern\n", taskID)
		}
		return nil
	}

	pattern := models.ParseRecurrence(args[1])
	if pattern == nil {
		return fmt.Errorf("invalid recurrence pattern: %s", args[1])
	}

	oldPattern := ""
	if task.Recurrence != nil {
		oldPattern = task.Recurrence.String()
	}

	task.SetRecurrence(pattern)

	if err := SaveProjectWithBackup(project); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	if oldPattern != "" {
		fmt.Printf("Updated recurrence for task #%d: %s -> %s (%s)\n", taskID, oldPattern, pattern.String(), task.Title)
	} else {
		fmt.Printf("Set recurrence for task #%d: %s (%s)\n", taskID, pattern.String(), task.Title)
	}
	return nil
}
