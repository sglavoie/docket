package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
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
- 'editor' field in the tt config file
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

	editor := resolveEditor(noteEditEditor, ctx.Editor)

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

	updatedNote, err := parseNoteEdit(note, modified)
	if err != nil {
		return err
	}
	if err := saveNoteContext(ctx, updatedNote); err != nil {
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

// applyNoteEdit parses the modified content from an external editor and persists
// the result. If the modified content has invalid frontmatter or an effectively
// wiped title, the function returns an error rather than silently saving a
// sanitized copy.
//
// Background: storage.ParseNoteContent never returns a Go error — when the user
// corrupts or deletes the YAML frontmatter, it returns a Note with ParseError
// set and Title derived from the filename. Saving that Note silently rewrites
// the file with filename-derived metadata, throwing away whatever the user
// actually typed. Treating ParseError as a hard error gives the user a chance
// to fix their edit before the original file is clobbered.
//
// We also reject a title that has been collapsed to the filename when the
// original title was something else — storage.parseNote silently substitutes
// the filename when the YAML title is empty, so the only way to detect "user
// wiped the title" is to compare against the pre-edit title.
func parseNoteEdit(original *models.Note, modified string) (*models.Note, error) {
	updated, err := storage.ParseNoteContent(original.Filename, modified)
	if err != nil {
		return nil, fmt.Errorf("failed to parse edited note: %w", err)
	}
	if updated.ParseError != "" {
		return nil, fmt.Errorf("edited note has invalid frontmatter: %s (changes not saved)", updated.ParseError)
	}
	if strings.TrimSpace(updated.Title) == "" {
		return nil, fmt.Errorf("edited note has empty title (changes not saved)")
	}
	// Detect the filename-substituted title that storage.parseNote produces
	// when the user wipes the YAML title. The substitution is identifiable
	// because it equals the raw filename minus ".md" — a value no human
	// would normally pick as a title.
	if original.Title != "" && original.Title != strings.TrimSuffix(original.Filename, ".md") {
		substituted := strings.TrimSuffix(original.Filename, ".md")
		if updated.Title == substituted {
			return nil, fmt.Errorf("edited note has empty title (changes not saved)")
		}
	}
	return updated, nil
}
