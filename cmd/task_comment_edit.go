package cmd

import (
	"fmt"
	"time"

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
- 'editor' field in the tt config file
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
	addOutputFlags(taskCommentEditCmd)
}

func runTaskCommentEdit(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("task ID and comment index")
	}

	project, cfg, configManager, err := LoadCurrentProject()
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
		if !out.Quiet {
			fmt.Println("No changes made.")
		}
		return nil
	}

	// Check if empty
	if newText == "" {
		return tterrors.NewRequiredFieldError("comment text")
	}

	// Update comment (preserve comment timestamp) and refresh task's UpdatedAt
	comment.Text = newText
	task.UpdatedAt = time.Now()

	// Save
	if err := SaveProject(project, &SaveOptions{
		Config: cfg, ConfigManager: configManager,
		UndoAction: "Edit Comment", UndoContext: fmt.Sprintf("#%d", task.ShortID),
	}); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	if !out.Quiet {
		fmt.Printf("Updated comment %d on #%d: %s\n", index, task.ShortID, task.Title)
	}
	return nil
}
