package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/models"
	"github.com/spf13/cobra"
)

var (
	noteParentFlag string
)

var noteAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new note",
	Long: `Add a new note to the current project.

Notes are informational items without status, priority, or tags.
They can be attached to any item type as children.

Examples:
  tt note add "Meeting notes from standup"
  tt note add "Design considerations" --parent 1
  tt n a "Quick note"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runNoteAdd,
}

func init() {
	noteCmd.AddCommand(noteAddCmd)

	noteAddCmd.Flags().StringVarP(&noteParentFlag, "parent", "p", "", "parent task ID or short ID")
}

func runNoteAdd(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	title := strings.Join(args, " ")
	if title == "" {
		return fmt.Errorf("note title is required")
	}

	// Create note
	noteID := uuid.New().String()
	shortID := project.GetNextShortID()
	note := models.NewTask(noteID, shortID, models.TaskTypeNote, title)

	// Handle parent
	if noteParentFlag != "" {
		parent := FindTaskByIdentifier(project, noteParentFlag)
		if parent == nil {
			return fmt.Errorf("parent task '%s' not found", noteParentFlag)
		}

		if !models.TaskTypeNote.CanBeChildOf(parent.Type) {
			return fmt.Errorf("note cannot be a child of %s", parent.Type.HeadingPrefix())
		}

		note.ParentID = &parent.ID
	}

	// Add to project
	project.Tasks = append(project.Tasks, note)

	// Save
	if err := SaveProjectWithBackup(project); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	msg := fmt.Sprintf("[%s] Created Note #%d: %s", project.Name, shortID, title)
	fmt.Println(msg)

	return nil
}
