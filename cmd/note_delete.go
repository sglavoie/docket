package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var noteDeleteForce bool

var noteDeleteCmd = &cobra.Command{
	Use:     "delete <filename-or-title>",
	Aliases: []string{"rm"},
	Short:   "Delete a note",
	Long: `Delete a note from the current project.

Examples:
  tt note delete meeting
  tt note delete 2025-01-28-meeting-notes.md
  tt note delete meeting --force`,
	Args: cobra.ExactArgs(1),
	RunE: runNoteDelete,
}

func init() {
	noteCmd.AddCommand(noteDeleteCmd)
	noteDeleteCmd.Flags().BoolVarP(&noteDeleteForce, "force", "f", false, "skip confirmation")
}

func runNoteDelete(cmd *cobra.Command, args []string) error {
	var note *models.Note
	var deleteDir string

	if noteGlobal {
		dir, gErr := globalNotesDir()
		if gErr != nil {
			return gErr
		}
		var err error
		note, err = findNoteIn(dir, args[0])
		if err != nil {
			return err
		}
		deleteDir = dir
	} else {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return err
		}
		note, err = findNote(project, args[0])
		if err != nil {
			return err
		}
		deleteDir = storage.NotesDir(project)
	}

	if !noteDeleteForce {
		fmt.Printf("Delete note '%s'? [y/N] ", note.Title)
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Canceled.")
			return nil
		}
	}

	if err := storage.DeleteNoteFrom(deleteDir, note.Filename); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	fmt.Printf("Deleted note: %s\n", note.Title)
	return nil
}
