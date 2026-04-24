package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

var boardReopenCmd = &cobra.Command{
	Use:   "reopen [id]",
	Short: "Reopen a done or dismissed action",
	Long: `Reopen an action that was previously marked done or dismissed.

If no ID is provided, an interactive selector is shown.

Examples:
  tt board reopen 1
  tt board reopen 3
  tt board reopen`,
	RunE: runBoardReopen,
}

func init() {
	boardCmd.AddCommand(boardReopenCmd)
	addOutputFlags(boardReopenCmd)
}

func runBoardReopen(cmd *cobra.Command, args []string) error {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	task, project, err := SelectActionTask(cfg, args, "Select action to reopen:", "No actions found. Use 'tt board add' to create one.")
	if err != nil {
		return err
	}

	if !task.Status.IsComplete() {
		if out.Quiet {
			return nil
		}
		fmt.Printf("Action #%d is already open.\n", task.ShortID)
		return nil
	}

	if msg := task.CanTransitionTo(models.StatusTodo); msg != "" {
		return fmt.Errorf("cannot reopen: %s", msg)
	}

	prevStates := workflow.CaptureTaskStates(project.Tasks)

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusTodo)
	})

	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: mgr,
		PrevStates: prevStates,
		UndoAction: "Board Reopen", UndoContext: fmt.Sprintf("#%d %s", task.ShortID, task.Title),
	}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return out.OutputTask(project.Name, "Reopened", task)
}
