package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
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

	if blocker.IsResolved() {
		fmt.Printf("Blocker '%s' is already resolved.\n", blocker.Title)
		return nil
	}

	blocker.Resolve()

	if err := storage.SaveBlockers(project, blockers); err != nil {
		return fmt.Errorf("failed to save blockers: %w", err)
	}

	fmt.Printf("Resolved blocker: %s\n", blocker.Title)
	return nil
}
