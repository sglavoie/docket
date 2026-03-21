package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var boardDoneCmd = &cobra.Command{
	Use:   "done <id>",
	Short: "Mark an action as done",
	Long: `Mark an action item as done.

Examples:
  tt board done 1
  tt board done 3`,
	RunE: runBoardDone,
}

func init() {
	boardCmd.AddCommand(boardDoneCmd)
	addOutputFlags(boardDoneCmd)
}

func runBoardDone(cmd *cobra.Command, args []string) error {
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

	if msg := task.CanTransitionTo(models.StatusDone); msg != "" {
		return fmt.Errorf("cannot mark done: %s", msg)
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusDone)
	})

	if err := SaveProject(project, &SaveOptions{Config: cfg, ConfigManager: mgr}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return out.OutputTask(project.Name, "Done", task)
}
