package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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
	RunE: runTaskCommentAdd,
}

func init() {
	taskCommentCmd.AddCommand(taskCommentAddCmd)
}

func runTaskCommentAdd(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and comment text")
	}

	taskID := args[0]

	// Join remaining args as the comment text
	commentText := strings.Join(args[1:], " ")
	commentText = strings.TrimSpace(commentText)
	if commentText == "" {
		return tterrors.NewRequiredFieldError("comment text")
	}

	// Use WithTaskUpdate to add comment
	task, err := WithTaskUpdate(taskID, func(t *models.Task) error {
		t.AddComment(commentText)
		return nil
	})

	if err != nil {
		return err
	}

	fmt.Printf("Added comment to #%d: %s\n", task.ShortID, task.Title)
	return nil
}
