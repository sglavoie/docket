package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var boardDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark an action as done",
	Long: `Mark an action item as done.

Examples:
  tt board done 1
  tt board done 3`,
	Args: cobra.ExactArgs(1),
	RunE: runBoardDone,
}

func init() {
	boardCmd.AddCommand(boardDoneCmd)
}

func runBoardDone(cmd *cobra.Command, args []string) error {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	task, project := FindActionTaskByID(cfg, args[0])
	if task == nil {
		return fmt.Errorf("action not found: %s", args[0])
	}

	if msg := task.CanTransitionTo(models.StatusDone); msg != "" {
		return fmt.Errorf("cannot mark done: %s", msg)
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusDone)
	})

	if err := SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Marked action #%d as done: %s\n", task.ShortID, task.Title)
	return nil
}
