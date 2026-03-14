package cmd

import (
	"github.com/spf13/cobra"
)

var blockerCmd = &cobra.Command{
	Use:     "blocker",
	Aliases: []string{"b", "blockers"},
	Short:   "Manage project blockers",
	Long: `Commands for adding, listing, and resolving project blockers.

A blocker is an impediment that prevents progress on the project.
Running 'tt blocker' without a subcommand lists all current blockers.

Examples:
  tt blocker                                  # List all blockers
  tt blocker add "Waiting for API access"     # Add a blocker
  tt blocker resolve abc12345                 # Mark blocker as resolved
  tt blocker delete abc12345                  # Delete a blocker
  tt blockers                                 # Alias for list`,
	RunE: runBlockerList, // Default to listing blockers
}

func init() {
	rootCmd.AddCommand(blockerCmd)
}
