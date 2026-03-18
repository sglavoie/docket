package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskFocusCmd = &cobra.Command{
	Use:   "focus [id]",
	Short: "Toggle the active/focus flag on a task",
	Long: `Toggle a task's active flag, adding or removing it from focus view.

If no ID is specified, opens an interactive selector.

Examples:
  tt task focus 5       # Toggle focus on task #5
  tt task focus         # Interactive selection`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskFocus,
}

var taskUnfocusCmd = &cobra.Command{
	Use:   "unfocus [id]",
	Short: "Remove a task from focus",
	Long: `Remove the active flag from a task, removing it from focus view.

Examples:
  tt task unfocus 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskUnfocus,
}

func init() {
	taskCmd.AddCommand(taskFocusCmd)
	taskCmd.AddCommand(taskUnfocusCmd)
}

func runTaskFocus(cmd *cobra.Command, args []string) error {
	// Handle interactive selection if no ID provided
	var taskID string
	if len(args) > 0 {
		taskID = args[0]
	} else {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}

		task, err := SelectTask(project, args,
			func(t *models.Task) bool { return t.Status.IsActive() },
			"Select task to toggle focus:", NoItemsMsg("active tasks", "tt task add <title>"))
		if err != nil {
			return err
		}
		taskID = task.ID
	}

	// Use WithTaskUpdate to toggle focus
	task, err := WithTaskUpdate(taskID, func(t *models.Task) error {
		t.ToggleActive()
		return nil
	})

	if err != nil {
		return err
	}

	if task.Active {
		fmt.Printf("Added #%d to focus: %s\n", task.ShortID, task.Title)
	} else {
		fmt.Printf("Removed #%d from focus: %s\n", task.ShortID, task.Title)
	}

	return nil
}

func runTaskUnfocus(cmd *cobra.Command, args []string) error {
	// Handle interactive selection if no ID provided
	var taskID string
	if len(args) > 0 {
		taskID = args[0]
	} else {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}

		task, err := SelectTask(project, args,
			func(t *models.Task) bool { return t.Active && t.Status.IsActive() },
			"Select task to remove from focus:", "no tasks in focus")
		if err != nil {
			return err
		}
		taskID = task.ID
	}

	// Use WithTaskUpdate to remove focus
	task, err := WithTaskUpdate(taskID, func(t *models.Task) error {
		if !t.Active {
			fmt.Printf("Task #%d is not in focus\n", t.ShortID)
			return nil
		}
		t.Deactivate()
		return nil
	})

	if err != nil {
		return err
	}

	if !task.Active {
		fmt.Printf("Removed #%d from focus: %s\n", task.ShortID, task.Title)
	}

	return nil
}
