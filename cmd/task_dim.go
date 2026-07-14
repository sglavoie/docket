package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task
	if len(args) > 0 {
		found, findErr := FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
		task = found
	} else {
		selected, selErr := SelectTask(project, args,
			func(t *models.Task) bool { return !t.IsTerminalState() },
			"Select task to toggle dim:", NoItemsMsg("tasks", "tt task add <title>"))
		if selErr != nil {
			return selErr
		}
		task = selected
	}

	// The toggle target is derived from the loaded copy; the application service
	// mutates a freshly-snapshotted task, so its result carries no dimmed flag.
	nowDimmed := !task.Dimmed

	svc := newWorkspaceAppService(workspace)
	res, err := withTaskRetry(func() (app.TaskResult, error) {
		return svc.ToggleDimmed(task.ID)
	})
	if err != nil {
		return translateAppError(err)
	}

	if nowDimmed {
		fmt.Printf("Dimmed task #%d: %s\n", task.ShortID, res.Title)
	} else {
		fmt.Printf("Undimmed task #%d: %s\n", task.ShortID, res.Title)
	}
	return nil
}

func runTaskUndim(_ *cobra.Command, args []string) error {
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task
	if len(args) > 0 {
		found, findErr := FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
		task = found
	} else {
		selected, selErr := SelectTask(project, args,
			func(t *models.Task) bool { return t.Dimmed },
			"Select task to undim:", "no dimmed tasks found")
		if selErr != nil {
			return selErr
		}
		task = selected
	}

	// Read-only adapter precheck: undimming a non-dimmed task is a friendly no-op.
	if !task.Dimmed {
		fmt.Printf("Task #%d is not dimmed\n", task.ShortID)
		return nil
	}

	svc := newWorkspaceAppService(workspace)
	res, err := withTaskRetry(func() (app.TaskResult, error) {
		return svc.Undim(task.ID)
	})
	if err != nil {
		return translateAppError(err)
	}

	fmt.Printf("Undimmed task #%d: %s\n", task.ShortID, res.Title)
	return nil
}
