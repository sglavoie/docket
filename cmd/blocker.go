package cmd

import (
	"github.com/spf13/cobra"
)

var blockerCmd = &cobra.Command{
	Use:     "blocker",
	Aliases: []string{"b", "blockers"},
	Short:   "Manage project blockers",
	Long:    `Commands for adding, listing, and resolving project blockers.`,
	RunE:    runBlockerList, // Default to listing blockers
}

func init() {
	rootCmd.AddCommand(blockerCmd)
}
