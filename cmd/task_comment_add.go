package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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
	addOutputFlags(taskCommentAddCmd)
}

func runTaskCommentAdd(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

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

	// Resolve the task in the current project, then persist through the
	// application layer so the comment commits atomically, revision-checked,
	// and undoably via the workspace ledger.
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}
	task, findErr := FindTaskByIdentifier(project, taskID)
	if findErr != nil {
		return findErr
	}

	svc := newAppService(cfg)
	res, err := withResultRetry(func() (app.CommentResult, error) {
		return svc.AddComment(task.ID, commentText)
	})
	if err != nil {
		return translateAppError(err)
	}

	if out.JSON {
		return printMutationJSON(res.Task)
	}
	if !out.Quiet {
		fmt.Printf("Added comment to #%d: %s\n", res.Task.ShortID, res.Task.Title)
	}
	return nil
}
