package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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

	// Resolve the task in the current project, then persist through the
	// application layer; the epic-type and has-children guards live in the use
	// case so every adapter enforces them uniformly.
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}
	task, findErr := FindTaskByIdentifier(project, taskID)
	if findErr != nil {
		return findErr
	}

	svc := newWorkspaceAppService(workspace)
	res, err := withResultRetry(func() (app.EpicBaselineResult, error) {
		return svc.BaselineEpic(task.ID)
	})
	if err != nil {
		return translateAppError(err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printMutationJSON(res.Task)
	}

	fmt.Printf("Baselined Epic #%d: %s (%d children)\n",
		res.Task.ShortID,
		res.Task.Title,
		res.ChildCount,
	)
	return nil
}
