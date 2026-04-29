package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var epicBaselineCmd = &cobra.Command{
	Use:   "baseline <id>",
	Short: "Baseline an epic's scope",
	Long: `Baseline the scope of an epic by recording its current child count.

After baselining, ScopeDelta() reports growth or shrinkage relative to the
baseline. Re-running baseline updates both fields (idempotent).

The epic must have at least one child task.

Examples:
  tt epic baseline 3
  tt epic baseline abc12345`,
	Args: cobra.ExactArgs(1),
	RunE: runEpicBaseline,
}

func init() {
	epicCmd.AddCommand(epicBaselineCmd)
	addOutputFlags(epicBaselineCmd)
}

func runEpicBaseline(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	taskID := args[0]

	updatedTask, err := WithTaskUpdate(taskID, func(task *models.Task) error {
		if task.Type != models.TaskTypeEpic {
			return fmt.Errorf("task #%d is a %s, not an epic", task.ShortID, task.Type)
		}
		if len(task.Children) == 0 {
			return tterrors.NewInvalidValueError("epic", task.ShortID, "epic must have at least one child to baseline")
		}
		now := time.Now()
		task.EnsureEpicAttrs().InitialChildCount = len(task.Children)
		task.EnsureEpicAttrs().ScopeBaselinedAt = &now
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

	fmt.Printf("Baselined Epic #%d: %s (%d children)\n",
		updatedTask.ShortID,
		updatedTask.Title,
		updatedTask.EpicAttrs.InitialChildCount,
	)
	return nil
}
