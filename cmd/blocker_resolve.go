package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var blockerResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "Mark a blocker as resolved",
	Long: `Mark a blocker as resolved.

Use the ID prefix from 'tt blocker list' to identify the blocker.

Examples:
  tt blocker resolve abc12345
  tt b resolve abc1`,
	RunE: runBlockerResolve,
}

func init() {
	blockerCmd.AddCommand(blockerResolveCmd)
	addOutputFlags(blockerResolveCmd)
}

func runBlockerResolve(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("blocker ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	var resolved *models.Blocker
	alreadyResolved := false
	if err := WithBlockerUpdate(args[0], func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		if blocker.IsResolved() {
			alreadyResolved = true
			resolved = blocker
			return blockers, nil
		}
		blocker.Resolve()
		resolved = blocker
		return blockers, nil
	}); err != nil {
		return err
	}

	if out.Quiet || resolved == nil {
		return nil
	}
	if out.JSON {
		return printJSON(resolved)
	}
	if alreadyResolved {
		fmt.Printf("Blocker '%s' is already resolved.\n", resolved.Title)
	} else {
		fmt.Printf("Resolved blocker: %s\n", resolved.Title)
	}
	return nil
}
