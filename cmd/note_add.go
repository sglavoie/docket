package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var noteAddTags []string

var noteAddCmd = &cobra.Command{
	Use:     "add <title>",
	Aliases: []string{"a", "new"},
	Short:   "Create a new note",
	Long: `Create a new structured note in the current project.

Notes are project-level markdown documents stored in the notes/ directory.

Examples:
  tt note add "Meeting notes"
  tt note add "Design ideas" --tag design --tag architecture
  tt note add "Quick thought" --edit`,
	Args: cobra.MinimumNArgs(1),
	RunE: runNoteAdd,
}

func init() {
	noteCmd.AddCommand(noteAddCmd)
	noteAddCmd.Flags().StringSliceVarP(&noteAddTags, "tag", "t", nil, "tags for the note")
}

func runNoteAdd(cmd *cobra.Command, args []string) error {
	title := strings.Join(args, " ")
	if title == "" {
		return tterrors.NewRequiredFieldError("note title")
	}

	note := models.NewNote(title)
	if len(noteAddTags) > 0 {
		note.SetTags(noteAddTags)
	}

	ctx, err := resolveNoteContext()
	if err != nil {
		return err
	}

	if ctx.Project == nil {
		if err := storage.SaveNoteTo(ctx.Dir, note); err != nil {
			return fmt.Errorf("failed to save note: %w", err)
		}
		fmt.Printf("[global] Created note: %s (%s)\n", title, note.Filename)
	} else {
		if err := storage.SaveNote(ctx.Project, note); err != nil {
			return fmt.Errorf("failed to save note: %w", err)
		}
		fmt.Printf("[%s] Created note: %s (%s)\n", ctx.Project.Name, title, note.Filename)
	}
	return nil
}
