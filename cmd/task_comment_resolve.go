package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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

	// Resolve the task in the current project; index validation runs against the
	// loaded copy so error messages match the legacy surface, and the toggle
	// itself persists through the application layer.
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}
	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	index, err := parseItemIndex(args[1], len(task.Comments), "comment")
	if err != nil {
		return err
	}

	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	res, err := withResultRetry(func() (app.CommentWorkflowResult, error) {
		return workflows.service.ToggleCommentResolvedWithWorkflow(task.ID, index, workflows.options(true, "Resolve Comment", task))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(res.Cascade)

	state := "resolved"
	if !res.Comment.Resolved {
		state = "unresolved"
	}

	if out.JSON {
		return printMutationJSON(res.Comment.Task)
	}
	if !out.Quiet {
		fmt.Printf("Comment %d marked as %s on #%d: %s\n", res.Comment.Index, state, res.Comment.Task.ShortID, res.Comment.Task.Title)
	}
	return nil
}
