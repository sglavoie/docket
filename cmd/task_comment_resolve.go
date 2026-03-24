package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var taskCommentResolveCmd = &cobra.Command{
	Use:   "resolve <task-id> <comment-index>",
	Short: "Toggle resolved state of a comment",
	Long: `Toggle the resolved/unresolved state of a comment by its index.

Resolved comments are hidden by default in 'tt task comment list'.
Use 'tt task comment list --all' to see resolved comments.

Examples:
  tt task comment resolve 5 0        # Toggle resolved state of first comment
  tt t c resolve 5 0`,
	RunE: runTaskCommentResolve,
}

func init() {
	taskCommentCmd.AddCommand(taskCommentResolveCmd)
	addOutputFlags(taskCommentResolveCmd)
}

func runTaskCommentResolve(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and comment index")
	}

	var commentIndex int
	var state string

	task, err := WithTaskUpdate(args[0], func(task *models.Task) error {
		index, err := parseItemIndex(args[1], len(task.Comments), "comment")
		if err != nil {
			return err
		}

		if !task.ToggleCommentResolved(index) {
			return fmt.Errorf("failed to toggle comment at index %d", index)
		}

		commentIndex = index
		state = "resolved"
		if !task.Comments[index].Resolved {
			state = "unresolved"
		}
		return nil
	})
	if err != nil {
		return err
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		fmt.Printf("Comment %d marked as %s on #%d: %s\n", commentIndex, state, task.ShortID, task.Title)
	}
	return nil
}
