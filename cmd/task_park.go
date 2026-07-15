package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	task, note, err := resolveTaskParkArgs(project, args)
	if err != nil {
		return err
	}

	// Route the mutation through the application service (atomic, revision-checked,
	// undoable via the workspace ledger). The completed-task guard now lives in
	// Service.Park; the CLI keeps only reference resolution and presentation.
	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		return workflows.service.ParkWithWorkflow(task.ID, note, workflows.options(true, "Park Task", task))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	res := workflowResultTask(cascade)

	if out.JSON {
		return emitMutation(out, "", res)
	}
	if !out.Quiet {
		// Print the note we requested (the app mutates a freshly-snapshotted task, not
		// this loaded copy). models.Park trims the note, matching resolveTaskParkArgs.
		if note == "" {
			fmt.Printf("Parked task #%d: %s\n", task.ShortID, res.Title)
		} else {
			fmt.Printf("Parked task #%d: %s\n", task.ShortID, note)
		}
	}
	return nil
}

func runTaskUnpark(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	task, err := resolveTaskUnparkArgs(project, args)
	if err != nil {
		return err
	}

	// Read-only adapter precheck: unparking a non-parked task is a friendly no-op
	// that must not record an undo entry.
	if !task.IsParked() {
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if !out.Quiet {
			fmt.Printf("Task #%d is not parked.\n", task.ShortID)
		}
		return nil
	}

	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		return workflows.service.UnparkWithWorkflow(task.ID, workflows.options(true, "Unpark Task", task))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	if !workflowResultCommitted(cascade) {
		if out.JSON {
			freshProject, _, loadErr := LoadRuntimeCurrentProject()
			if loadErr != nil {
				return loadErr
			}
			freshTask, findErr := FindTaskByIdentifier(freshProject, task.ID)
			if findErr != nil {
				return findErr
			}
			return outputSingleTaskJSON(freshTask)
		}
		if !out.Quiet {
			fmt.Printf("Task #%d is not parked.\n", task.ShortID)
		}
		return nil
	}
	res := workflowResultTask(cascade)

	if out.JSON {
		return emitMutation(out, "", res)
	}
	if !out.Quiet {
		fmt.Printf("Unparked task #%d: %s\n", task.ShortID, res.Title)
	}
	return nil
}

func resolveTaskParkArgs(project *models.Project, args []string) (*models.Task, string, error) {
	if len(args) > 0 {
		task, findErr := FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return nil, "", findErr
		}
		return task, strings.TrimSpace(strings.Join(args[1:], " ")), nil
	}
	task, err := SelectTask(project, nil,
		func(t *models.Task) bool { return !t.IsTerminalState() },
		"Select task to park:", NoItemsMsg("active tasks", "tt task add <title>"))
	if err != nil {
		return nil, "", err
	}
	return task, "", nil
}

func resolveTaskUnparkArgs(project *models.Project, args []string) (*models.Task, error) {
	if len(args) > 0 {
		return FindTaskByIdentifier(project, args[0])
	}
	task, err := SelectTask(project, nil,
		func(t *models.Task) bool { return t.IsParked() },
		"Select task to unpark:", "No parked tasks found.")
	if err != nil {
		return nil, err
	}
	return task, nil
}
