package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
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
	taskID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	priority, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid priority: %s (must be 1-5)", args[1])
	}

	if priority < 1 || priority > 5 {
		return fmt.Errorf("priority must be between 1 and 5, got %d", priority)
	}

	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := project.GetTaskByShortID(taskID)
	if task == nil {
		return fmt.Errorf("task #%d not found", taskID)
	}

	oldPriority := task.Priority
	if oldPriority == priority {
		fmt.Printf("Task #%d already has priority %d\n", taskID, priority)
		return nil
	}

	task.Priority = priority
	task.UpdatedAt = time.Now()

	if err := SaveProjectWithRules(project, cfg, configManager); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	fmt.Printf("Updated priority for task #%d: %d -> %d (%s)\n", taskID, oldPriority, priority, task.Title)
	return nil
}
