package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

var taskEditCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit a task's title and description",
	Long: `Edit a task's title and description in an external editor.

If no ID is specified, opens an interactive selector.

The editor can be configured via:
- --editor flag
- 'editor' field in the tt config file
- EDITOR environment variable
- Falls back to 'code' (VS Code)

Examples:
  tt task edit 5
  tt task edit 3 --editor vim
  tt t edit`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTaskEdit,
}

var editEditor string

func init() {
	taskCmd.AddCommand(taskEditCmd)
	taskEditCmd.Flags().StringVar(&editEditor, "editor", "", "editor to use")
	addOutputFlags(taskEditCmd)
	taskEditCmd.ValidArgsFunction = completeCurrentProjectTaskIDs()
}

func runTaskEdit(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Load project and config once (the config manager is no longer needed here:
	// the mutation is persisted through the application layer, not a direct save).
	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}

	// Handle interactive selection if no ID provided
	var task *models.Task
	if len(args) > 0 {
		var findErr error
		task, findErr = FindTaskByIdentifier(project, args[0])
		if findErr != nil {
			return findErr
		}
	} else {
		var selectErr error
		task, selectErr = SelectTask(project, args,
			func(t *models.Task) bool { return true },
			"Select task to edit:", "no tasks available. Add one with: tt task add <title>")
		if selectErr != nil {
			return selectErr
		}
	}

	editor := resolveEditor(editEditor, workspace.Runtime.Machine().GetEditor())

	// Build content for editing
	content := buildEditContent(task)

	// Edit in external editor
	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	// Parse modified content
	newTitle, newDesc := parseEditedContent(modified, task)

	// Validate edited title
	if validationErr := models.ValidateTaskTitle(newTitle); validationErr != nil {
		return fmt.Errorf("invalid title: %w", validationErr)
	}

	// Check if changed
	titleChanged := newTitle != task.Title
	descChanged := newDesc != task.Description
	if !titleChanged && !descChanged {
		if out.JSON {
			return outputSingleTaskJSON(task)
		}
		if !out.Quiet {
			fmt.Println("No changes made.")
		}
		return nil
	}

	// Route the persistence through the application layer so the edit commits
	// atomically, revision-checked, and undoably via app.Service (with
	// stale-revision retry). The interactive editor and content parse above stay
	// local because they need the loaded task; only the mutation moves. The task
	// retains the current project's identity through the application boundary.
	workflows := newCLIWorkflowRunner(workspace)
	defer workflows.flush()
	undoContext := fmt.Sprintf("#%d %s", task.ShortID, newTitle)
	cascade, err := withResultRetry(func() (app.BulkResult, error) {
		return workflows.service.EditWithWorkflow(app.EditRequest{
			Locator:     app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:       &newTitle,
			Description: &newDesc,
		}, workflows.optionsWithContext(true, "Edit Task", undoContext))
	})
	if err != nil {
		return translateAppError(err)
	}
	workflows.observe(cascade)
	res := workflowResultTask(cascade)

	return emitMutation(out, "Updated", res)
}

// buildEditContent creates the content shown in the editor.
func buildEditContent(task *models.Task) string {
	content := fmt.Sprintf("# %s\n\n", task.Title)
	content += "<!-- Edit the title above and description below. -->\n\n"
	content += task.Description
	return content
}

// parseEditedContent extracts the title and description from edited content.
func parseEditedContent(content string, task *models.Task) (title, description string) {
	lines := strings.Split(content, "\n")
	var descLines []string
	title = task.Title // Default to original title
	inDescription := false
	foundComment := false
	foundTitle := false

	// Collect lines that come after the title but before the comment marker.
	// If no comment marker is found, these become the description (fallback).
	var preCommentLines []string

	for _, line := range lines {
		// Already in description section
		if inDescription {
			descLines = append(descLines, line)
			continue
		}

		// Look for title line (first # line, only before the comment marker)
		if !foundComment && !foundTitle && len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			title = strings.TrimSpace(line[2:])
			foundTitle = true
			continue
		}

		// Look for the HTML comment marker
		if len(line) >= 4 && line[:4] == "<!--" {
			foundComment = true
			continue
		}

		// After comment, skip empty lines until description starts
		if foundComment && line == "" {
			continue
		}

		// Start of description content
		if foundComment {
			inDescription = true
			descLines = append(descLines, line)
			continue
		}

		// Before comment marker: accumulate lines in case comment is missing
		if foundTitle {
			preCommentLines = append(preCommentLines, line)
		}
	}

	// If no comment marker was found, use pre-comment lines as description
	// so that accidentally deleting the marker doesn't silently drop content.
	if !foundComment && len(preCommentLines) > 0 {
		descLines = preCommentLines
	}

	// Trim trailing empty lines
	for len(descLines) > 0 && descLines[len(descLines)-1] == "" {
		descLines = descLines[:len(descLines)-1]
	}

	// Trim leading empty lines
	for len(descLines) > 0 && descLines[0] == "" {
		descLines = descLines[1:]
	}

	// If title was cleared, keep the original
	if title == "" {
		title = task.Title
	}

	return title, strings.Join(descLines, "\n")
}
