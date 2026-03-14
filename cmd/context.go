package cmd

import (
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:     "context",
	Aliases: []string{"ctx"},
	Short:   "Manage custom contexts",
	Long: `Commands for creating, listing, and managing custom contexts.

A context is a named grouping of projects, useful for focusing on a
subset of your work (e.g., "Work", "Side Projects", "Client X").
Running 'tt context' without a subcommand lists all contexts.

Examples:
  tt context                    # List all contexts
  tt context new Work           # Create a new context
  tt ctx new "Side Projects"    # Create using alias
  tt context delete Work        # Remove a context
  tt context switch Work        # Switch active context`,
	RunE: runContextList, // Default to listing custom contexts
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
