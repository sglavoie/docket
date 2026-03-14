package cmd

import (
	"github.com/spf13/cobra"
)

var blockerCmd = &cobra.Command{
	Use:     "blocker",
	Aliases: []string{"b", "blockers"},
	Short:   "Manage project blockers",
	Long: `Commands for adding, listing, and resolving task dependency blockers.

A blocker is a dependency relationship where one task blocks another.
Running 'tt blocker' without a subcommand lists all current blockers.

Examples:
  tt blocker                          # List all blockers
  tt blocker add 3 --blocked-by 1     # Task 1 blocks task 3
  tt blocker remove 3 --blocked-by 1  # Remove the dependency
  tt blockers                         # Alias for list`,
	RunE: runBlockerList, // Default to listing blockers
}

func init() {
	rootCmd.AddCommand(blockerCmd)
}
