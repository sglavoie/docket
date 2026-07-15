package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var noteShowNoGlow bool

var noteShowCmd = &cobra.Command{
	Use:     "show <filename-or-title>",
	Aliases: []string{"view"},
	Short:   "Show a note",
	Long: `Display a note using glow for markdown rendering (if available).

Examples:
  tt note show 2026-01-28-meeting-notes.md
  tt note show meeting
  tt note show --no-glow meeting`,
	RunE: runNoteView,
}

func init() {
	noteCmd.AddCommand(noteShowCmd)
	noteShowCmd.Flags().BoolVar(&noteShowNoGlow, "no-glow", false, "disable glow rendering")
}

func runNoteView(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("note ID")
	}

	ctx, err := resolveNoteContext()
	if err != nil {
		return err
	}
	note, err := findNoteContext(ctx, args[0])
	if err != nil {
		return err
	}
	glowEnabled := ctx.Workspace.Runtime.Settings().GlowEnabled

	content := fmt.Sprintf("# %s\n\n%s", note.Title, note.Body)
	if len(note.Tags) > 0 {
		content = fmt.Sprintf("# %s\n\nTags: %s\n\n%s", note.Title, strings.Join(note.Tags, ", "), note.Body)
	}

	useGlow := glowEnabled && Deps.Renderer.Available() && !noteShowNoGlow
	if useGlow {
		return Deps.Renderer.PageMarkdown(content)
	}

	fmt.Println(content)
	return nil
}
