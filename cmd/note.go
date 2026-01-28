package cmd

import (
	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:     "note",
	Aliases: []string{"n"},
	Short:   "Manage structured notes",
	Long:    `Commands for creating and managing structured notes within the current project.`,
}

func init() {
	rootCmd.AddCommand(noteCmd)
}
