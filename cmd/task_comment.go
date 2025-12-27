package cmd

import (
	"github.com/spf13/cobra"
)

var taskCommentCmd = &cobra.Command{
	Use:     "comment",
	Aliases: []string{"c"},
	Short:   "Manage task comments",
	Long: `Commands for adding, listing, and removing comments on tasks.

Comments are timestamped notes that capture the WHY and decision history,
separate from the task description which captures the WHAT.

Examples:
  tt task comment list 5           # List comments on task 5
  tt task comment add 5 "note"     # Add comment to task 5
  tt task comment remove 5 0       # Remove first comment from task 5`,
}

func init() {
	taskCmd.AddCommand(taskCommentCmd)
}
