package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var boardDismissCmd = &cobra.Command{
	Use:   "dismiss [id]",
	Short: "Dismiss an action as irrelevant",
	Long: `Dismiss an action, marking it as canceled.

If no ID is provided, an interactive selector is shown.

Examples:
  tt board dismiss 1
  tt board dismiss 3
  tt board dismiss`,
	RunE: runBoardDismiss,
}

func init() {
	boardCmd.AddCommand(boardDismissCmd)
	addOutputFlags(boardDismissCmd)
}

func runBoardDismiss(cmd *cobra.Command, args []string) error {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	task, project, err := SelectActionTask(cfg, args, "Select action to dismiss:", "No actions found. Use 'tt board add' to create one.")
	if err != nil {
		return err
	}

	if msg := task.CanTransitionTo(models.StatusCanceled); msg != "" {
		return fmt.Errorf("cannot dismiss: %s", msg)
	}

	prevStates := CaptureTaskStates(project.Tasks)

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusCanceled)
	})

	project.CheckAndAutoUnblock()

	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: mgr,
		PrevStates: prevStates,
		UndoAction: "Board Dismiss", UndoContext: fmt.Sprintf("#%d %s", task.ShortID, task.Title),
	}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return out.OutputTask(project.Name, "Dismissed", task)
}
