package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var blockerShowJSON bool

var blockerShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show blocker details",
	Long: `Show full details for a blocker.

Use the ID prefix from 'tt blocker list' to identify the blocker.
If glow is available, the description is rendered as markdown.

Examples:
  tt blocker show abc12345
  tt blocker show abc1 --json`,
	RunE: runBlockerShow,
}

func init() {
	blockerCmd.AddCommand(blockerShowCmd)
	blockerShowCmd.Flags().BoolVar(&blockerShowJSON, "json", false, "output as JSON")
}

func runBlockerShow(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("blocker ID")
	}

	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	blocker := storage.FindBlockerByPrefix(blockers, args[0])
	if blocker == nil {
		return fmt.Errorf("blocker not found: %s", args[0])
	}

	if blockerShowJSON {
		data, err := json.MarshalIndent(blocker, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal blocker to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	return printBlockerDetails(project.Name, blocker)
}

func printBlockerDetails(projectName string, blocker *models.Blocker) error {
	status := "Active"
	if blocker.IsResolved() {
		status = "Resolved"
	}

	fmt.Printf("Blocker: %s\n", blocker.Title)
	fmt.Printf("ID:      %s\n", blocker.ID)
	fmt.Printf("Status:  %s\n", status)
	fmt.Printf("Project: %s\n", projectName)
	fmt.Printf("Created: %s\n", storage.FormatRelativeTime(blocker.CreatedAt))

	if blocker.ResolvedAt != nil {
		fmt.Printf("Resolved: %s\n", storage.FormatRelativeTime(*blocker.ResolvedAt))
	}

	if blocker.Description != "" {
		fmt.Println()
		// Use glow if available for rendering
		if err := external.RenderMarkdown(blocker.Description); err != nil {
			// Fallback to plain text
			fmt.Println(blocker.Description)
		}
	}

	return nil
}
