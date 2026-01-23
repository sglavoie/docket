package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
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
}

func runTaskCommentRemove(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return tterrors.NewTaskNotFoundError(args[0])
	}

	// Handle --clear flag
	if commentClearFlag {
		if len(task.Comments) == 0 {
			fmt.Printf("Task #%d has no comments to clear\n", task.ShortID)
			return nil
		}
		count := len(task.Comments)
		task.ClearComments()
		if err := SaveProjectWithBackup(project); err != nil {
			return fmt.Errorf("failed to save: %w", err)
		}
		fmt.Printf("Cleared %d comment(s) from #%d: %s\n", count, task.ShortID, task.Title)
		return nil
	}

	// Require index argument if not clearing all
	if len(args) < 2 {
		return fmt.Errorf("comment index required (use --clear to remove all)")
	}

	index, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid comment index: %s", args[1])
	}

	if index < 0 || index >= len(task.Comments) {
		return fmt.Errorf("comment index %d out of range (task has %d comments)", index, len(task.Comments))
	}

	removed := task.RemoveComment(index)
	if !removed {
		return fmt.Errorf("failed to remove comment at index %d", index)
	}

	if err := SaveProjectWithBackup(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Removed comment %d from #%d: %s\n", index, task.ShortID, task.Title)
	return nil
}
