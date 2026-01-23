package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var taskCommentAddCmd = &cobra.Command{
	Use:   "add <task-id> <comment>",
	Short: "Add a comment to a task",
	Long: `Add a timestamped comment to a task.

Comments capture the WHY and decision history, separate from the task
description which captures the WHAT.

Examples:
  tt task comment add 5 "Found the root cause of the issue"
  tt task comment add abc123 "Tried approach X, didn't work because of Y"
  tt t c a 5 "Quick note"`,
	Args: cobra.MinimumNArgs(2),
	RunE: runTaskCommentAdd,
}

func init() {
	taskCommentCmd.AddCommand(taskCommentAddCmd)
}

func runTaskCommentAdd(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return tterrors.NewTaskNotFoundError(args[0])
	}

	// Join remaining args as the comment text
	commentText := strings.Join(args[1:], " ")
	commentText = strings.TrimSpace(commentText)
	if commentText == "" {
		return fmt.Errorf("comment text cannot be empty")
	}

	task.AddComment(commentText)

	if err := SaveProjectWithBackup(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Added comment to #%d: %s\n", task.ShortID, task.Title)
	return nil
}
