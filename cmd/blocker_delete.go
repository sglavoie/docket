package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	idPrefix := args[0]

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	blocker := storage.FindBlockerByPrefix(blockers, idPrefix)
	if blocker == nil {
		return fmt.Errorf("blocker not found: %s", idPrefix)
	}

	// Confirmation
	if !blockerDeleteForce {
		fmt.Printf("Delete blocker '%s'? (y/N): ", blocker.Title)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Canceled.")
			return nil
		}
	}

	// Find and remove the blocker
	var newBlockers []*models.Blocker
	for _, b := range blockers {
		if b.ID != blocker.ID {
			newBlockers = append(newBlockers, b)
		}
	}

	if err := storage.SaveBlockers(project, newBlockers); err != nil {
		return fmt.Errorf("failed to save blockers: %w", err)
	}

	fmt.Printf("Deleted blocker: %s\n", blocker.Title)
	return nil
}
