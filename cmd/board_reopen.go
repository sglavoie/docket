package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var boardReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a done or dismissed action",
	Long: `Reopen an action that was previously marked done or dismissed.

Examples:
  tt board reopen 1
  tt board reopen 3`,
	RunE: runBoardReopen,
}

func init() {
	boardCmd.AddCommand(boardReopenCmd)
}

func runBoardReopen(cmd *cobra.Command, args []string) error {
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

	if !task.Status.IsComplete() {
		fmt.Printf("Action #%d is already open.\n", task.ShortID)
		return nil
	}

	if msg := task.CanTransitionTo(models.StatusTodo); msg != "" {
		return fmt.Errorf("cannot reopen: %s", msg)
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusTodo)
	})

	if err := SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Reopened action #%d: %s\n", task.ShortID, task.Title)
	return nil
}
