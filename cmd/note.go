package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	noteGlobal       bool
	noteShortcutTags []string
)

var noteCmd = &cobra.Command{
	Use:     "note",
	Aliases: []string{"n"},
	Short:   "Manage structured notes",
	Long: `Commands for creating and managing structured notes within the current project.
Use --global to operate on global notes (not tied to any project).

When invoked as "tt n <title>", creates a global note directly.`,
	Args: cobra.ArbitraryArgs,
	RunE: runNoteShortcut,
}

func init() {
	rootCmd.AddCommand(noteCmd)
	noteCmd.PersistentFlags().BoolVar(&noteGlobal, "global", false, "operate on global notes")
	noteCmd.Flags().StringSliceVarP(&noteShortcutTags, "tag", "T", nil, "tags for the note (only with 'n' shortcut)")
}

func runNoteShortcut(cmd *cobra.Command, args []string) error {
	if cmd.CalledAs() != "n" || len(args) == 0 {
		return cmd.Help()
	}

	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return cmd.Help()
	}
	if len(title) > 200 {
		return tterrors.NewInvalidValueError("title", len(title), "max 200 characters")
	}

	note := models.NewNote(title)
	if len(noteShortcutTags) > 0 {
		note.SetTags(noteShortcutTags)
	}

	// Force global context for the shortcut.
	noteGlobal = true
	ctx, err := resolveNoteContext()
	if err != nil {
		return err
	}

	if err := saveNoteContext(ctx, note); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}
	fmt.Printf("[global] Created note: %s (%s)\n", title, note.Filename)
	return nil
}

// globalNotesDir returns the global notes directory path and its domain-native
// workspace snapshot.
func globalNotesDir() (string, *CLIWorkspace, error) {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return "", nil, err
	}
	return storage.GlobalNotesDir(workspace.Runtime.DataDir()), workspace, nil
}
