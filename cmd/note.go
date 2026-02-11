package cmd

import (
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

var noteGlobal bool

var noteCmd = &cobra.Command{
	Use:     "note",
	Aliases: []string{"n"},
	Short:   "Manage structured notes",
	Long: `Commands for creating and managing structured notes within the current project.
Use --global to operate on global notes (not tied to any project).`,
}

func init() {
	rootCmd.AddCommand(noteCmd)
	noteCmd.PersistentFlags().BoolVar(&noteGlobal, "global", false, "operate on global notes")
}

// globalNotesDir returns the global notes directory path and config.
func globalNotesDir() (string, *config.Config, error) {
	cfg, _, err := LoadConfigStrict()
	if err != nil {
		return "", nil, err
	}
	return storage.GlobalNotesDir(cfg.DataDir), cfg, nil
}
