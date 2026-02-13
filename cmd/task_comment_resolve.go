package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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
	Args: cobra.ExactArgs(2),
	RunE: runTaskCommentResolve,
}

func init() {
	taskCommentCmd.AddCommand(taskCommentResolveCmd)
}

func runTaskCommentResolve(cmd *cobra.Command, args []string) error {
	_, err := WithTaskUpdate(args[0], func(task *models.Task) error {
		index, err := parseItemIndex(args[1], len(task.Comments), "comment")
		if err != nil {
			return err
		}

		if !task.ToggleCommentResolved(index) {
			return fmt.Errorf("failed to toggle comment at index %d", index)
		}

		state := "resolved"
		if !task.Comments[index].Resolved {
			state = "unresolved"
		}
		fmt.Printf("Comment %d marked as %s on #%d: %s\n", index, state, task.ShortID, task.Title)
		return nil
	})
	return err
}
