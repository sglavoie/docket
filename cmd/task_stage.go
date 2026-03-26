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
	addOutputFlags(taskStageCmd)
}

func runTaskStage(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	taskID := args[0]

	// With only 1 argument, show the current stage (read-only)
	if len(args) == 1 {
		project, _, _, loadErr := LoadCurrentProject()
		if loadErr != nil {
			return loadErr
		}
		task, findErr := FindTaskByIdentifier(project, taskID)
		if findErr != nil {
			return findErr
		}
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if !out.Quiet {
			if task.Stage == "" {
				fmt.Printf("Task #%d has no stage.\n", task.ShortID)
			} else {
				fmt.Printf("Task #%d stage: %s\n", task.ShortID, task.Stage)
			}
		}
		return nil
	}

	stageName := args[1]

	// Pre-check: skip the save if the stage is already the requested value
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}
	task, findErr := FindTaskByIdentifier(project, taskID)
	if findErr != nil {
		return findErr
	}
	if task.Stage == stageName {
		if !out.Quiet {
			if stageName == "" {
				fmt.Printf("Task #%d already has no stage.\n", task.ShortID)
			} else {
				fmt.Printf("Task #%d is already at stage '%s'.\n", task.ShortID, stageName)
			}
		}
		return nil
	}

	oldStage := task.Stage

	updatedTask, err := WithTaskUpdateInProject(taskID, func(project *models.Project, task *models.Task) error {
		task.Stage = stageName
		return nil
	})

	if err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputSingleTaskJSON(updatedTask)
	}

	if stageName == "" {
		fmt.Printf("Cleared stage from task #%d (was: %s)\n", task.ShortID, oldStage)
	} else if oldStage == "" {
		fmt.Printf("Set stage for task #%d: %s\n", task.ShortID, stageName)
	} else {
		fmt.Printf("Changed stage for task #%d from '%s' to '%s'\n", task.ShortID, oldStage, stageName)
	}

	return nil
}
