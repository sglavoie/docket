package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskDimCmd = &cobra.Command{
	Use:   "dim [id]",
	Short: "Toggle the dimmed flag on a task",
	Long: `Toggle a task's dimmed flag, visually muting it in all views.

Dimmed tasks remain visible but are rendered in a muted style.
Dimming clears the active/focus flag since the two states are contradictory.

If no ID is specified, opens an interactive selector.

Examples:
  tt task dim 5       # Toggle dim on task #5
  tt task dim         # Interactive selection`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskDim,
}

var taskUndimCmd = &cobra.Command{
	Use:   "undim [id]",
	Short: "Remove the dimmed flag from a task",
	Long: `Remove the dimmed flag from a task, restoring normal rendering.

Examples:
  tt task undim 5`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskUndim,
}

func init() {
	taskCmd.AddCommand(taskDimCmd)
	taskCmd.AddCommand(taskUndimCmd)
}

func runTaskDim(_ *cobra.Command, args []string) error {
	var taskID string
	if len(args) > 0 {
		taskID = args[0]
	} else {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}
		task, err := SelectTask(project, args,
			func(t *models.Task) bool { return !t.Status.IsComplete() },
			"Select task to toggle dim:", NoItemsMsg("tasks", "tt task add <title>"))
		if err != nil {
			return err
		}
		taskID = task.ID
	}

	task, err := WithTaskUpdate(taskID, func(t *models.Task) error {
		t.ToggleDimmed()
		return nil
	})
	if err != nil {
		return err
	}

	if task.Dimmed {
		fmt.Printf("Dimmed task #%d: %s\n", task.ShortID, task.Title)
	} else {
		fmt.Printf("Undimmed task #%d: %s\n", task.ShortID, task.Title)
	}
	return nil
}

func runTaskUndim(_ *cobra.Command, args []string) error {
	var taskID string
	if len(args) > 0 {
		taskID = args[0]
	} else {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}
		task, err := SelectTask(project, args,
			func(t *models.Task) bool { return t.Dimmed },
			"Select task to undim:", "no dimmed tasks found")
		if err != nil {
			return err
		}
		taskID = task.ID
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}
	preTask, findErr := FindTaskByIdentifier(project, taskID)
	if findErr != nil {
		return findErr
	}
	if !preTask.Dimmed {
		fmt.Printf("Task #%d is not dimmed\n", preTask.ShortID)
		return nil
	}

	task, err := WithTaskUpdate(taskID, func(t *models.Task) error {
		t.Undim()
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("Undimmed task #%d: %s\n", task.ShortID, task.Title)
	return nil
}
