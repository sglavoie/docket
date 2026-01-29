package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

var noteEditEditor string

var noteEditCmd = &cobra.Command{
	Use:   "edit <filename-or-title>",
	Short: "Edit a note in an external editor",
	Long: `Open a note in an external editor.

The editor can be configured via:
- --editor flag
- 'editor' field in ~/.tt.json
- EDITOR environment variable
- Falls back to 'code' (VS Code)

Examples:
  tt note edit meeting
  tt note edit 2025-01-28-meeting-notes.md
  tt note edit meeting --editor vim`,
	Args: cobra.ExactArgs(1),
	RunE: runNoteEdit,
}

func init() {
	noteCmd.AddCommand(noteEditCmd)
	noteEditCmd.Flags().StringVar(&noteEditEditor, "editor", "", "editor to use")
}

func runNoteEdit(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	note, err := findNote(project, args[0])
	if err != nil {
		return err
	}

	editor := noteEditEditor
	if editor == "" {
		editor = cfg.Editor
	}

	// Read current content
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		return fmt.Errorf("failed to render note: %w", err)
	}

	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	if modified == content {
		fmt.Println("No changes made.")
		return nil
	}

	// Re-parse and save
	updatedNote, err := storage.ParseNoteContent(note.Filename, modified)
	if err != nil {
		return fmt.Errorf("failed to parse edited note: %w", err)
	}

	if err := storage.SaveNote(project, updatedNote); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	fmt.Println("Note updated.")
	return nil
}
