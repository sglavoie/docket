package cmd

import (
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:     "board",
	Aliases: []string{"bd"},
	Short:   "Manage focus board actions",
	Long: `Commands for managing focus board actions — lightweight action items
that live in the tt task system as Action-type tasks.

Running 'tt board' without a subcommand lists open actions.`,
	RunE: runBoardList,
}

func init() {
	rootCmd.AddCommand(boardCmd)
}
