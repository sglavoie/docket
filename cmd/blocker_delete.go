package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var blockerDeleteForce bool

var blockerDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a blocker",
	Long: `Delete a blocker permanently.

Use the ID prefix from 'tt blocker list' to identify the blocker.

Examples:
  tt blocker delete abc12345
  tt blocker delete abc1 --force`,
	RunE: runBlockerDelete,
}

func init() {
	blockerCmd.AddCommand(blockerDeleteCmd)
	blockerDeleteCmd.Flags().BoolVarP(&blockerDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runBlockerDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("blocker ID")
	}

	return WithBlockerUpdate(args[0], func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		// Confirmation
		if !ConfirmYN(fmt.Sprintf("Delete blocker '%s'? [y/N]: ", blocker.Title), blockerDeleteForce) {
			fmt.Println("Canceled")
			return blockers, nil
		}

		// Remove the blocker from list
		var newBlockers []*models.Blocker
		for _, b := range blockers {
			if b.ID != blocker.ID {
				newBlockers = append(newBlockers, b)
			}
		}

		fmt.Printf("Deleted blocker: %s\n", blocker.Title)
		return newBlockers, nil
	})
}
