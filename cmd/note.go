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

// globalNotesDir returns the global notes directory path from the config.
func globalNotesDir() (string, error) {
	mgr := config.NewManager(GetConfigPath())
	cfg, err := mgr.Load()
	if err != nil {
		return "", err
	}
	return storage.GlobalNotesDir(cfg.DataDir), nil
}
