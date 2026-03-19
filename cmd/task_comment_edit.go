package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

var taskCommentEditCmd = &cobra.Command{
	Use:   "edit <task-id> <comment-index>",
	Short: "Edit a comment on a task",
	Long: `Edit a comment on a task using an external editor.

Use 'tt task comment list' to see comment indices.

The editor can be configured via:
- --editor flag
- 'editor' field in ~/.tt.json
- EDITOR environment variable
- Falls back to 'code' (VS Code)

Examples:
  tt task comment edit 5 0        # Edit first comment on task 5
  tt task comment edit 5 0 --editor vim
  tt t c e 5 0`,
	RunE: runTaskCommentEdit,
}

var commentEditEditor string

func init() {
	taskCommentEditCmd.Flags().StringVar(&commentEditEditor, "editor", "", "editor to use")
	taskCommentCmd.AddCommand(taskCommentEditCmd)
}

func runTaskCommentEdit(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and comment index")
	}

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, args[0])
	if findErr != nil {
		return findErr
	}

	index, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid comment index: %s", args[1])
	}

	if index < 0 || index >= len(task.Comments) {
		return fmt.Errorf("comment index %d out of range (task has %d comments)", index, len(task.Comments))
	}

	comment := task.Comments[index]

	editor := resolveEditor(commentEditEditor, cfg.Editor)

	// Build content for editing
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n", index, task.ShortID, task.Title)
	content += "<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n"
	content += comment.Text

	// Edit in external editor
	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	// Parse modified content
	newText := util.ParseEditedCommentContent(modified)

	// Check if changed
	if newText == comment.Text {
		fmt.Println("No changes made.")
		return nil
	}

	// Check if empty
	if newText == "" {
		return fmt.Errorf("comment text cannot be empty")
	}

	// Update comment (preserve timestamp)
	comment.Text = newText

	// Save
	if err := SaveProject(project, nil); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Updated comment %d on #%d: %s\n", index, task.ShortID, task.Title)
	return nil
}
