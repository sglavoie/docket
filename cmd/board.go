package cmd

import (
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:     "board",
	Aliases: []string{"bd"},
	Short:   "Manage action board items",
	Long: `Commands for managing lightweight action board items
that live in the tt task system as Action-type tasks.

Running 'tt board' without a subcommand lists open actions.

Examples:
  tt board                                        # List open actions
  tt board add "Review PR" --source github        # Add an action
  tt board done 1                                 # Mark action done
  tt board dismiss 2                              # Dismiss as irrelevant
  tt board list --all                             # Include done/dismissed
  tt bd add "Quick note" --priority 1             # High priority action`,
	RunE: runBoardList,
}

func init() {
	rootCmd.AddCommand(boardCmd)
}
