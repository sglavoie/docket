package cmd

import (
	"fmt"

	"github.com/sglavoie/dev-helpers/go/tt/internal/external"
	"github.com/sglavoie/dev-helpers/go/tt/internal/models"
	"github.com/sglavoie/dev-helpers/go/tt/internal/storage"
	"github.com/sglavoie/dev-helpers/go/tt/internal/tui"
	"github.com/spf13/cobra"
)

var taskEditCmd = &cobra.Command{
	Use:   "edit [id]",
	Short: "Edit a task's description",
	Long: `Edit a task's description in an external editor.

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
	project, cfg, _, err := getCurrentProject()
	if err != nil {
		return err
	}

	var task *models.Task

	if len(args) > 0 {
		task = findTask(project, args[0])
		if task == nil {
			return fmt.Errorf("task '%s' not found", args[0])
		}
	} else {
		// Interactive selection
		if len(project.Tasks) == 0 {
			return fmt.Errorf("no tasks available")
		}

		items := make([]tui.SelectorItem, len(project.Tasks))
		for i, t := range project.Tasks {
			items[i] = tui.SelectorItem{
				ID:   t.ID,
				Data: t,
				Columns: []string{
					fmt.Sprintf("#%d", t.ShortID),
					t.Type.Icon(),
					t.Status.Icon(),
					t.Title,
				},
			}
		}

		selected, err := tui.RunSelector("Select task to edit:", items)
		if err != nil {
			return err
		}

		task = selected.Data.(*models.Task)
	}

	// Determine editor
	editor := editEditor
	if editor == "" {
		editor = cfg.Editor
	}

	// Build content for editing
	content := buildEditContent(task)

	// Edit in external editor
	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	// Parse modified content
	newDesc := parseEditedContent(modified, task)

	// Check if changed
	if newDesc == task.Description {
		fmt.Println("No changes made.")
		return nil
	}

	// Update task
	task.Description = newDesc

	// Save
	if err := storage.SaveProject(project); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	fmt.Printf("Updated %s #%d: %s\n",
		task.Type.HeadingPrefix(),
		task.ShortID,
		task.Title)

	return nil
}

// buildEditContent creates the content shown in the editor.
func buildEditContent(task *models.Task) string {
	header := fmt.Sprintf("# %s #%d: %s\n\n", task.Type.HeadingPrefix(), task.ShortID, task.Title)
	header += "<!-- Edit the description below. Lines starting with # or <!-- are ignored. -->\n\n"
	return header + task.Description
}

// parseEditedContent extracts the description from edited content.
func parseEditedContent(content string, task *models.Task) string {
	lines := splitLines(content)
	var descLines []string
	inDescription := false

	for _, line := range lines {
		// Skip header line
		if !inDescription {
			if line == "" || line[0] == '#' || (len(line) >= 4 && line[:4] == "<!--") {
				if line == "" && len(descLines) == 0 {
					inDescription = true
				}
				continue
			}
			inDescription = true
		}

		descLines = append(descLines, line)
	}

	// Trim trailing empty lines
	for len(descLines) > 0 && descLines[len(descLines)-1] == "" {
		descLines = descLines[:len(descLines)-1]
	}

	return joinLines(descLines)
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
