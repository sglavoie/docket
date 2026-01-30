package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
)

var blockerResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "Mark a blocker as resolved",
	Long: `Mark a blocker as resolved.

Use the ID prefix from 'tt blocker list' to identify the blocker.

Examples:
  tt blocker resolve abc12345
  tt b resolve abc1`,
	Args: cobra.ExactArgs(1),
	RunE: runBlockerResolve,
}

func init() {
	blockerCmd.AddCommand(blockerResolveCmd)
}

func runBlockerResolve(cmd *cobra.Command, args []string) error {
	return WithBlockerUpdate(args[0], func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		if blocker.IsResolved() {
			fmt.Printf("Blocker '%s' is already resolved.\n", blocker.Title)
			return blockers, nil
		}

		blocker.Resolve()
		fmt.Printf("Resolved blocker: %s\n", blocker.Title)
		return blockers, nil
	})
}
