package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var noteViewNoGlow bool

var noteViewCmd = &cobra.Command{
	Use:   "view <filename-or-title>",
	Short: "View a note",
	Long: `Display a note using glow for markdown rendering (if available).

Examples:
  tt note view 2025-01-28-meeting-notes.md
  tt note view meeting
  tt note view --no-glow meeting`,
	Args: cobra.ExactArgs(1),
	RunE: runNoteView,
}

func init() {
	noteCmd.AddCommand(noteViewCmd)
	noteViewCmd.Flags().BoolVar(&noteViewNoGlow, "no-glow", false, "disable glow rendering")
}

func runNoteView(cmd *cobra.Command, args []string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	note, err := findNote(project, args[0])
	if err != nil {
		return err
	}

	content := fmt.Sprintf("# %s\n\n%s", note.Title, note.Body)
	if len(note.Tags) > 0 {
		content = fmt.Sprintf("# %s\n\nTags: %s\n\n%s", note.Title, strings.Join(note.Tags, ", "), note.Body)
	}

	useGlow := cfg.GlowEnabled && Deps.Renderer.Available() && !noteViewNoGlow
	if useGlow {
		return Deps.Renderer.PageMarkdown(content)
	}

	fmt.Println(content)
	return nil
}
