package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/storage"
)

var (
	blockerEditTitle       string
	blockerEditDescription string
	blockerUnresolve       bool
)

var blockerEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a blocker",
	Long: `Edit a blocker's title, description, or resolved status.

Use the ID prefix from 'tt blocker list' to identify the blocker.

Examples:
  tt blocker edit abc12345 --title "New title"
  tt blocker edit abc1 -d "Updated description"
  tt blocker edit abc1 --unresolve`,
	Args: cobra.ExactArgs(1),
	RunE: runBlockerEdit,
}

func init() {
	blockerCmd.AddCommand(blockerEditCmd)

	blockerEditCmd.Flags().StringVarP(&blockerEditTitle, "title", "t", "", "new title for the blocker")
	blockerEditCmd.Flags().StringVarP(&blockerEditDescription, "description", "d", "", "new description for the blocker")
	blockerEditCmd.Flags().BoolVar(&blockerUnresolve, "unresolve", false, "mark blocker as unresolved")
}

func runBlockerEdit(cmd *cobra.Command, args []string) error {
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

	modified := false

	if blockerEditTitle != "" {
		blocker.Title = strings.TrimSpace(blockerEditTitle)
		modified = true
	}

	if blockerEditDescription != "" {
		blocker.Description = strings.TrimSpace(blockerEditDescription)
		modified = true
	}

	if blockerUnresolve {
		if blocker.IsResolved() {
			blocker.Unresolve()
			modified = true
		}
	}

	if !modified {
		fmt.Println("No changes specified. Use --title, --description, or --unresolve flags.")
		return nil
	}

	if err := storage.SaveBlockers(project, blockers); err != nil {
		return fmt.Errorf("failed to save blockers: %w", err)
	}

	fmt.Printf("Updated blocker: %s\n", blocker.Title)
	return nil
}
