package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

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
- 'editor' field in ~/.tt.json
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
}

func runTaskEdit(cmd *cobra.Command, args []string) error {
	// Load project and config once
	project, cfg, configManager, err := LoadCurrentProject()
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

	editor := resolveEditor(editEditor, cfg.Editor)

	// Build content for editing
	content := buildEditContent(task)

	// Edit in external editor
	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	// Parse modified content
	newTitle, newDesc := parseEditedContent(modified, task)

	// Check if changed
	titleChanged := newTitle != task.Title
	descChanged := newDesc != task.Description
	if !titleChanged && !descChanged {
		fmt.Println("No changes made.")
		return nil
	}

	// Apply changes directly to the already-loaded task and save
	prevStates := CaptureTaskStates(project.Tasks)
	task.Title = newTitle
	task.Description = newDesc
	task.UpdatedAt = time.Now()

	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: configManager, PrevStates: prevStates}); err != nil {
		return err
	}

	fmt.Printf("Updated %s #%d: %s\n",
		task.Type.HeadingPrefix(),
		task.ShortID,
		task.Title)

	return nil
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

	for _, line := range lines {
		// Already in description section
		if inDescription {
			descLines = append(descLines, line)
			continue
		}

		// Look for title line (first # line)
		if len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			title = strings.TrimSpace(line[2:])
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
		}
	}

	// Trim trailing empty lines
	for len(descLines) > 0 && descLines[len(descLines)-1] == "" {
		descLines = descLines[:len(descLines)-1]
	}

	// If title was cleared, keep the original
	if title == "" {
		title = task.Title
	}

	return title, strings.Join(descLines, "\n")
}
