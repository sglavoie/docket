package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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

	var removedCount int
	var removedIndex int

	task, err := WithTaskUpdate(args[0], func(task *models.Task) error {
		// Handle --clear flag
		if commentClearFlag {
			if len(task.Comments) == 0 {
				fmt.Printf("Task #%d has no comments to clear\n", task.ShortID)
				return nil
			}
			removedCount = len(task.Comments)
			task.ClearComments()
			return nil
		}

		// Require index argument if not clearing all
		if len(args) < 2 {
			return fmt.Errorf("comment index required (use --clear to remove all)")
		}

		var index int
		index, err = parseItemIndex(args[1], len(task.Comments), "comment")
		if err != nil {
			return err
		}

		if !task.RemoveComment(index) {
			return fmt.Errorf("failed to remove comment at index %d", index)
		}

		removedIndex = index
		return nil
	})
	if err != nil {
		return err
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		if commentClearFlag {
			fmt.Printf("Cleared %d comment(s) from #%d: %s\n", removedCount, task.ShortID, task.Title)
		} else {
			fmt.Printf("Removed comment %d from #%d: %s\n", removedIndex, task.ShortID, task.Title)
		}
	}
	return nil
}
