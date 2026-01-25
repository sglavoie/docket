package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var blockerDescription string

var blockerAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new blocker",
	Long: `Add a new blocker to the current project.

A blocker represents an impediment that blocks progress on the project.

Examples:
  tt blocker add "Waiting for API access"
  tt blocker add "Manager absent, cannot get review" -d "Need approval by next week"
  tt b a "Build server down"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runBlockerAdd,
}

func init() {
	blockerCmd.AddCommand(blockerAddCmd)
	blockerAddCmd.Flags().StringVarP(&blockerDescription, "description", "d", "", "blocker description")
}

func runBlockerAdd(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	title := strings.Join(args, " ")
	if title == "" {
		return tterrors.NewRequiredFieldError("title")
	}

	// Load existing blockers
	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	// Create new blocker
	blocker := models.NewBlocker(title)
	blocker.Description = blockerDescription

	// Add to list and save
	blockers = append(blockers, blocker)
	if err := storage.SaveBlockers(project, blockers); err != nil {
		return fmt.Errorf("failed to save blockers: %w", err)
	}

	fmt.Printf("Created blocker: %s\n", title)
	fmt.Printf("ID: %s\n", blocker.ID[:8])
	return nil
}
