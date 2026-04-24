package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var noteEditEditor string

var noteEditCmd = &cobra.Command{
	Use:   "edit <filename-or-title>",
	Short: "Edit a note in an external editor",
	Long: `Open a note in an external editor.

The editor can be configured via:
- --editor flag
- 'editor' field in ~/.tt/.tt.json
- EDITOR environment variable
- Falls back to 'code' (VS Code)

Examples:
  tt note edit meeting
  tt note edit 2026-01-28-meeting-notes.md
  tt note edit meeting --editor vim`,
	RunE: runNoteEdit,
}

func init() {
	noteCmd.AddCommand(noteEditCmd)
	noteEditCmd.Flags().StringVar(&noteEditEditor, "editor", "", "editor to use")
	addOutputFlags(noteEditCmd)
}

func runNoteEdit(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("note ID")
	}

	ctx, err := resolveNoteContext()
	if err != nil {
		return err
	}

	note, err := findNoteIn(ctx.Dir, args[0])
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	var cfgEditor string
	if ctx.Config != nil {
		cfgEditor = ctx.Config.Editor
	}
	editor := resolveEditor(noteEditEditor, cfgEditor)

	content, err := storage.RenderNoteContent(note)
	if err != nil {
		return fmt.Errorf("failed to render note: %w", err)
	}

	modified, err := external.EditInExternalEditor(editor, content, ".md")
	if err != nil {
		return fmt.Errorf("failed to edit: %w", err)
	}

	if modified == content {
		if !out.Quiet {
			fmt.Println("No changes made.")
		}
		return nil
	}

	updatedNote, err := storage.ParseNoteContent(note.Filename, modified)
	if err != nil {
		return fmt.Errorf("failed to parse edited note: %w", err)
	}

	if err := storage.SaveNoteTo(ctx.Dir, updatedNote); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			Filename string `json:"filename"`
			Updated  bool   `json:"updated"`
		}{Filename: updatedNote.Filename, Updated: true})
	}
	fmt.Println("Note updated.")
	return nil
}
