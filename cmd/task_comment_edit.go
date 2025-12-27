package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/external"
	"github.com/spf13/cobra"
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
	Args: cobra.ExactArgs(2),
	RunE: runTaskCommentEdit,
}

var commentEditEditor string

func init() {
	taskCommentEditCmd.Flags().StringVar(&commentEditEditor, "editor", "", "editor to use")
	taskCommentCmd.AddCommand(taskCommentEditCmd)
}

func runTaskCommentEdit(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task := FindTaskByIdentifier(project, args[0])
	if task == nil {
		return fmt.Errorf("task '%s' not found", args[0])
	}

	index, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid comment index: %s", args[1])
	}

	if index < 0 || index >= len(task.Comments) {
		return fmt.Errorf("comment index %d out of range (task has %d comments)", index, len(task.Comments))
	}

	comment := task.Comments[index]

	// Determine editor
	editor := commentEditEditor
	if editor == "" {
		editor = cfg.Editor
	}

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
	newText := parseEditedCommentContent(modified)

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
	if err := SaveProjectWithBackup(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Updated comment %d on #%d: %s\n", index, task.ShortID, task.Title)
	return nil
}

// parseEditedCommentContent extracts the comment text from edited content.
func parseEditedCommentContent(content string) string {
	lines := strings.Split(content, "\n")
	var textLines []string
	foundComment := false

	for _, line := range lines {
		// Already collecting text
		if foundComment {
			textLines = append(textLines, line)
			continue
		}

		// Skip title line
		if len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			continue
		}

		// Look for the HTML comment marker
		if len(line) >= 4 && line[:4] == "<!--" {
			foundComment = true
			continue
		}
	}

	// Trim leading and trailing empty lines
	for len(textLines) > 0 && textLines[0] == "" {
		textLines = textLines[1:]
	}
	for len(textLines) > 0 && textLines[len(textLines)-1] == "" {
		textLines = textLines[:len(textLines)-1]
	}

	return strings.Join(textLines, "\n")
}
