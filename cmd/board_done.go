package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var boardDoneCmd = &cobra.Command{
	Use:   "done [id]",
	Short: "Mark an action as done",
	Long: `Mark an action item as done.

If no ID is provided, an interactive selector is shown.

Examples:
  tt board done 1
  tt board done 3
  tt board done`,
	RunE: runBoardDone,
}

func init() {
	boardCmd.AddCommand(boardDoneCmd)
	addOutputFlags(boardDoneCmd)
}

func runBoardDone(cmd *cobra.Command, args []string) error {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	task, project, err := SelectActionTask(cfg, args, "Select action to mark done:", "No actions found. Use 'tt board add' to create one.")
	if err != nil {
		return err
	}

	if msg := task.CanTransitionTo(models.StatusDone); msg != "" {
		return fmt.Errorf("cannot mark done: %s", msg)
	}

	prevStates := CaptureTaskStates(project.Tasks)

	project.MutateTask(task.ID, func(t *models.Task) {
		t.ApplyTransition(models.StatusDone)
	})

	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: mgr,
		PrevStates: prevStates,
		UndoAction: "Board Done", UndoContext: fmt.Sprintf("#%d %s", task.ShortID, task.Title),
	}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return out.OutputTask(project.Name, "Done", task)
}
