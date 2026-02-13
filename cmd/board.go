package cmd

import (
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:     "board",
	Aliases: []string{"bd"},
	Short:   "Manage focus board items",
	Long: `Commands for managing focus board items — external tasks, follow-ups,
and lightweight items that live outside the tt task system.

Running 'tt board' without a subcommand lists open items.`,
	RunE: runBoardList,
}

func init() {
	rootCmd.AddCommand(boardCmd)
}
