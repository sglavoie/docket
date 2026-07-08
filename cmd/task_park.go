package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskParkCmd = &cobra.Command{
	Use:   "park [id] [note...]",
	Short: "Park a task with a resume note",
	Long: `Park a task with an optional one-line resume note.

Parking records where you left off without changing task status.
If no ID is specified, opens an interactive selector.

Examples:
  tt task park 5 blocked on API review
  tt task park 5
  tt task park`,
	Args: cobra.ArbitraryArgs,
	RunE: runTaskPark,
}

var taskUnparkCmd = &cobra.Command{
	Use:   "unpark [id]",
	Short: "Clear a task's parking note",
	Long: `Clear a task's parking note.

If no ID is specified, opens an interactive selector for parked tasks.

Examples:
  tt task unpark 5
  tt task unpark`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskUnpark,
}

func init() {
	taskCmd.AddCommand(taskParkCmd)
	taskCmd.AddCommand(taskUnparkCmd)
	addOutputFlags(taskParkCmd)
	addOutputFlags(taskUnparkCmd)
}

func runTaskPark(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	taskID, note, err := resolveTaskParkArgs(args)
	if err != nil {
		return err
	}

	task, err := withTaskUpdateInProjectUndoAction(taskID, "Park Task", func(_ *models.Project, task *models.Task) error {
		if task.Status.IsComplete() {
			return fmt.Errorf("cannot park a %s task", task.Status)
		}
		task.Park(note)
		return nil
	})
	if err != nil {
		return err
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		if task.ParkedNote == "" {
			fmt.Printf("Parked task #%d: %s\n", task.ShortID, task.Title)
		} else {
			fmt.Printf("Parked task #%d: %s\n", task.ShortID, task.ParkedNote)
		}
	}
	return nil
}

func runTaskUnpark(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	taskID, err := resolveTaskUnparkArgs(args)
	if err != nil {
		return err
	}

	project, _, _, loadErr := LoadCurrentProject()
	if loadErr != nil {
		return loadErr
	}
	preTask, findErr := FindTaskByIdentifier(project, taskID)
	if findErr != nil {
		return findErr
	}
	if !preTask.IsParked() {
		if out.JSON {
			return outputSingleTaskJSON(preTask)
		}
		if !out.Quiet {
			fmt.Printf("Task #%d is not parked.\n", preTask.ShortID)
		}
		return nil
	}

	task, err := withTaskUpdateInProjectUndoAction(taskID, "Unpark Task", func(_ *models.Project, task *models.Task) error {
		task.Unpark(false)
		return nil
	})
	if err != nil {
		return err
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		fmt.Printf("Unparked task #%d: %s\n", task.ShortID, task.Title)
	}
	return nil
}

func resolveTaskParkArgs(args []string) (string, string, error) {
	if len(args) > 0 {
		return args[0], strings.TrimSpace(strings.Join(args[1:], " ")), nil
	}
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return "", "", err
	}
	task, err := SelectTask(project, nil,
		func(t *models.Task) bool { return !t.Status.IsComplete() },
		"Select task to park:", NoItemsMsg("active tasks", "tt task add <title>"))
	if err != nil {
		return "", "", err
	}
	return task.ID, "", nil
}

func resolveTaskUnparkArgs(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return "", err
	}
	task, err := SelectTask(project, nil,
		func(t *models.Task) bool { return t.IsParked() },
		"Select task to unpark:", "No parked tasks found.")
	if err != nil {
		return "", err
	}
	return task.ID, nil
}
