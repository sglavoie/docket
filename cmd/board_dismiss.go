package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var boardDismissCmd = &cobra.Command{
	Use:   "dismiss <id>",
	Short: "Dismiss an action as irrelevant",
	Long: `Dismiss an action, marking it as canceled.

Examples:
  tt board dismiss 1
  tt board dismiss 3`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardDismiss,
}

func init() {
	boardCmd.AddCommand(boardDismissCmd)
}

func runBoardDismiss(cmd *cobra.Command, args []string) error {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project := FindActionTaskByID(cfg, args[0])
	if task == nil {
		return fmt.Errorf("action not found: %s", args[0])
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

	fmt.Printf("Dismissed action #%d: %s\n", task.ShortID, task.Title)
	return nil
}
