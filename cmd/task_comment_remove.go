package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
)

var (
	commentClearFlag bool
)

var taskCommentRemoveCmd = &cobra.Command{
	Use:   "remove <task-id> [index]",
	Short: "Remove a comment from a task",
	Long: `Remove a comment from a task by its index.

Use 'tt task comment list' to see comment indices.

Examples:
  tt task comment remove 5 0        # Remove first comment from task 5
  tt task comment remove 5 --clear  # Remove all comments from task 5
  tt t c r 5 0`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTaskCommentRemove,
}

func init() {
	taskCommentRemoveCmd.Flags().BoolVar(&commentClearFlag, "clear", false, "Remove all comments")
	taskCommentCmd.AddCommand(taskCommentRemoveCmd)
	addOutputFlags(taskCommentRemoveCmd)
}

func runTaskCommentRemove(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Resolve the task in the current project; index validation runs against the
	// loaded copy so error messages match the legacy surface, and the mutation
	// itself persists through the application layer.
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}
	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}

	if commentClearFlag {
		// No-op fast path: skip the save entirely when there is nothing to clear.
		if len(task.Comments) == 0 {
			if !out.Quiet {
				fmt.Printf("Task #%d has no comments to clear\n", task.ShortID)
			}
			return nil
		}
		res, clearErr := withResultRetry(func() (app.CommentWorkflowResult, error) {
			return workflows.service.ClearCommentsWithWorkflowByLocator(locator, workflows.options(true, "Clear Comments", task))
		})
		if clearErr != nil {
			return translateAppError(clearErr)
		}
		workflows.observe(res.Cascade)
		if !workflowResultCommitted(res.Cascade) {
			if !out.Quiet {
				fmt.Printf("Task #%d has no comments to clear\n", task.ShortID)
			}
			return nil
		}
		if out.JSON {
			return printMutationJSON(res.Comment.Task)
		}
		if !out.Quiet {
			fmt.Printf("Cleared %d comment(s) from #%d: %s\n", res.Comment.Count, res.Comment.Task.ShortID, res.Comment.Task.Title)
		}
		return nil
	}

	// Require index argument if not clearing all
	if len(args) < 2 {
		return fmt.Errorf("comment index required (use --clear to remove all)")
	}
	index, err := parseItemIndex(args[1], len(task.Comments), "comment")
	if err != nil {
		return err
	}

	res, err := withResultRetry(func() (app.CommentWorkflowResult, error) {
		return workflows.service.RemoveCommentWithWorkflowByLocator(locator, index, workflows.options(true, "Remove Comment", task))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(res.Cascade)

	if out.JSON {
		return printMutationJSON(res.Comment.Task)
	}
	if !out.Quiet {
		fmt.Printf("Removed comment %d from #%d: %s\n", res.Comment.Index, res.Comment.Task.ShortID, res.Comment.Task.Title)
	}
	return nil
}
