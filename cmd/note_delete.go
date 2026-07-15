package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ctx, err := resolveNoteContext()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		notes, err := notesNoteContext(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var completions []string
		for _, n := range notes {
			completions = append(completions, n.Filename+"\t"+n.Title)
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	noteCmd.AddCommand(noteDeleteCmd)
	noteDeleteCmd.Flags().BoolVarP(&noteDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(noteDeleteCmd)
}

func runNoteDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("note ID (use 'tt note list' to see notes)")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	ctx, err := resolveNoteContext()
	if err != nil {
		return err
	}

	note, err := findNoteContext(ctx, args[0])
	if err != nil {
		return err
	}

	if !ConfirmYN(fmt.Sprintf("Delete note '%s'? [y/N]: ", note.Title), noteDeleteForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	if _, err := deleteNoteContext(ctx, note); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		entry := struct {
			Filename string `json:"filename"`
			Title    string `json:"title"`
		}{
			Filename: note.Filename,
			Title:    note.Title,
		}
		return printJSON(entry)
	}

	fmt.Printf("Deleted note: %s\n", note.Title)
	return nil
}
