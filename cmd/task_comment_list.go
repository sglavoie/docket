package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var commentListAllFlag bool

var taskCommentListCmd = &cobra.Command{
	Use:   "list <task-id>",
	Short: "List comments on a task",
	Long: `List comments on a task in chronological order.

By default, resolved comments are hidden. Use --all to show them.

Examples:
  tt task comment list 5
  tt task comment list 5 --all
  tt t c l 5`,
	Args: cobra.ExactArgs(1),
	RunE: runTaskCommentList,
}

func init() {
	taskCommentListCmd.Flags().BoolVar(&commentListAllFlag, "all", false, "Show all comments including resolved")
	taskCommentCmd.AddCommand(taskCommentListCmd)
}

func runTaskCommentList(cmd *cobra.Command, args []string) error {
	return WithTaskRead(args[0], func(project *models.Project, task *models.Task) error {
		return displayCommentList(task)
	})
}

func displayCommentList(task *models.Task) error {
	fmt.Printf("#%d: %s\n", task.ShortID, task.Title)
	fmt.Println(strings.Repeat("-", 40))

	if len(task.Comments) == 0 {
		fmt.Println("No comments.")
		return nil
	}

	resolvedCount := len(task.Comments) - task.UnresolvedCommentCount()

	// Check if all comments are resolved and --all not set
	if !commentListAllFlag && task.UnresolvedCommentCount() == 0 && resolvedCount > 0 {
		fmt.Printf("No unresolved comments. (%d resolved, use --all to show)\n", resolvedCount)
		return nil
	}

	displayed := 0
	for i, comment := range task.Comments {
		if !commentListAllFlag && comment.Resolved {
			continue
		}

		timestamp := storage.FormatRelativeTime(comment.CreatedAt)
		resolvedMarker := ""
		if comment.Resolved {
			resolvedMarker = " [resolved]"
		}
		fmt.Printf("[%d] %s (%s)%s\n", i, comment.CreatedAt.Format("2006-01-02 15:04"), timestamp, resolvedMarker)
		fmt.Printf("    %s\n\n", comment.Text)
		displayed++
	}

	if displayed == 0 {
		fmt.Printf("No unresolved comments. (%d resolved, use --all to show)\n", resolvedCount)
	} else if commentListAllFlag {
		fmt.Printf("%d comment(s)", len(task.Comments))
		if resolvedCount > 0 {
			fmt.Printf(" (%d resolved)", resolvedCount)
		}
		fmt.Println()
	} else {
		fmt.Printf("%d comment(s)", displayed)
		if resolvedCount > 0 {
			fmt.Printf(" (%d resolved, use --all to show)", resolvedCount)
		}
		fmt.Println()
	}

	return nil
}
