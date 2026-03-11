package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var taskStageCmd = &cobra.Command{
	Use:   "stage <task-id> [stage-name]",
	Short: "Set or clear a task's workflow stage",
	Long: `Set or clear the workflow stage for a task.

Use an empty string "" to clear the stage.

Examples:
  tt task stage 5 Brainstorm
  tt task stage 12 Design
  tt task stage 3 ""`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTaskStage,
}

func init() {
	taskCmd.AddCommand(taskStageCmd)
}

func runTaskStage(cmd *cobra.Command, args []string) error {
	taskID := args[0]
	stageName := ""
	if len(args) > 1 {
		stageName = args[1]
	}

	var oldStage string

	task, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
		oldStage = task.Stage
		task.Stage = stageName
		return nil
	})

	if err != nil {
		return err
	}

	if stageName == "" {
		if oldStage != "" {
			fmt.Printf("Cleared stage from task #%d (was: %s)\n", task.ShortID, oldStage)
		} else {
			fmt.Printf("Task #%d already has no stage\n", task.ShortID)
		}
	} else if oldStage == "" {
		fmt.Printf("Set stage for task #%d: %s\n", task.ShortID, stageName)
	} else {
		fmt.Printf("Changed stage for task #%d from '%s' to '%s'\n", task.ShortID, oldStage, stageName)
	}

	return nil
}
