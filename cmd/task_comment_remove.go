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
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}
	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	svc := newAppService(cfg)

	if commentClearFlag {
		// No-op fast path: skip the save entirely when there is nothing to clear.
		if len(task.Comments) == 0 {
			if !out.Quiet {
				fmt.Printf("Task #%d has no comments to clear\n", task.ShortID)
			}
			return nil
		}
		res, clearErr := withResultRetry(func() (app.CommentResult, error) {
			return svc.ClearComments(task.ID)
		})
		if clearErr != nil {
			return translateAppError(clearErr)
		}
		if out.JSON {
			return printMutationJSON(res.Task)
		}
		if !out.Quiet {
			fmt.Printf("Cleared %d comment(s) from #%d: %s\n", res.Count, res.Task.ShortID, res.Task.Title)
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

	res, err := withResultRetry(func() (app.CommentResult, error) {
		return svc.RemoveComment(task.ID, index)
	})
	if err != nil {
		return translateAppError(err)
	}

	if out.JSON {
		return printMutationJSON(res.Task)
	}
	if !out.Quiet {
		fmt.Printf("Removed comment %d from #%d: %s\n", res.Index, res.Task.ShortID, res.Task.Title)
	}
	return nil
}
