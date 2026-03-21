package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var boardDismissCmd = &cobra.Command{
	Use:   "dismiss <id>",
	Short: "Dismiss an action as irrelevant",
	Long: `Dismiss an action, marking it as canceled.

Examples:
  tt board dismiss 1
  tt board dismiss 3`,
	RunE: runBoardDismiss,
}

func init() {
	boardCmd.AddCommand(boardDismissCmd)
	addOutputFlags(boardDismissCmd)
}

func runBoardDismiss(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("action ID")
	}

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project := FindActionTaskByID(cfg, args[0])
	if task == nil {
		return fmt.Errorf("action not found: %s", args[0])
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if msg := task.CanTransitionTo(models.StatusCanceled); msg != "" {
		return fmt.Errorf("cannot dismiss: %s", msg)
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusCanceled)
	})

	if err := SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return out.OutputTask(project.Name, "Dismissed", task)
}
