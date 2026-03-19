package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var noteDeleteForce bool

var noteDeleteCmd = &cobra.Command{
	Use:     "delete <filename-or-title>",
	Aliases: []string{"rm"},
	Short:   "Delete a note",
	Long: `Delete a note from the current project.

Examples:
  tt note delete meeting
  tt note delete 2026-01-28-meeting-notes.md
  tt note delete meeting --force`,
	RunE: runNoteDelete,
}

func init() {
	noteCmd.AddCommand(noteDeleteCmd)
	noteDeleteCmd.Flags().BoolVarP(&noteDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runNoteDelete(cmd *cobra.Command, args []string) error {
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

	if !ConfirmYN(fmt.Sprintf("Delete note '%s'? [y/N] ", note.Title), noteDeleteForce) {
		fmt.Println("Canceled")
		return nil
	}

	if err := storage.DeleteNoteFrom(ctx.Dir, note.Filename); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	fmt.Printf("Deleted note: %s\n", note.Title)
	return nil
}
