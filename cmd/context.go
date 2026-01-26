package cmd

import (
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Manage custom contexts",
	Long:    `Commands for creating, listing, and managing custom contexts (project groupings).`,
	RunE:    runContextList, // Default to listing custom contexts
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
