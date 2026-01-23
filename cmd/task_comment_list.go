package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var taskCommentListCmd = &cobra.Command{
	Use:   "list <task-id>",
	Short: "List comments on a task",
	Long: `List all comments on a task in chronological order.

Examples:
  tt task comment list 5
  tt task comment list abc123
  tt t c l 5`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskCommentList,
}

func init() {
	taskCommentCmd.AddCommand(taskCommentListCmd)
}

func runTaskCommentList(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return tterrors.NewTaskNotFoundError(args[0])
	}

	fmt.Printf("#%d: %s\n", task.ShortID, task.Title)
	fmt.Println(strings.Repeat("-", 40))

	if len(task.Comments) == 0 {
		fmt.Println("No comments.")
		return nil
	}

	fmt.Printf("%d comment(s):\n\n", len(task.Comments))

	for i, comment := range task.Comments {
		timestamp := storage.FormatRelativeTime(comment.CreatedAt)
		fmt.Printf("[%d] %s (%s)\n", i, comment.CreatedAt.Format("2006-01-02 15:04"), timestamp)
		fmt.Printf("    %s\n\n", comment.Text)
	}

	return nil
}
