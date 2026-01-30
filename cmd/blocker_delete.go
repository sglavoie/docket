package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
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
	Args: cobra.ExactArgs(1),
	RunE: runBlockerDelete,
}

func init() {
	blockerCmd.AddCommand(blockerDeleteCmd)
	blockerDeleteCmd.Flags().BoolVarP(&blockerDeleteForce, "force", "f", false, "skip confirmation")
}

func runBlockerDelete(cmd *cobra.Command, args []string) error {
	return WithBlockerUpdate(args[0], func(blockers []*models.Blocker, blocker *models.Blocker) ([]*models.Blocker, error) {
		// Confirmation
		if !blockerDeleteForce {
			fmt.Printf("Delete blocker '%s'? (y/N): ", blocker.Title)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Canceled.")
				return blockers, nil
			}
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
