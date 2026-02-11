package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var scoreAssignCmd = &cobra.Command{
	Use:   "assign <profile-id>",
	Short: "Assign a scoring profile to the current project",
	Args:  cobra.ExactArgs(1),
	RunE:  runScoreAssign,
}

var scoreUnassignCmd = &cobra.Command{
	Use:   "unassign",
	Short: "Remove scoring profile from the current project",
	RunE:  runScoreUnassign,
}

func init() {
	scoreCmd.AddCommand(scoreAssignCmd)
	scoreCmd.AddCommand(scoreUnassignCmd)
}

func runScoreAssign(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return fmt.Errorf("%w. Select one with: tt select", tterrors.ErrNoCurrentProject)
	}

	profile := cfg.GetScoringProfile(args[0])
	if profile == nil {
		return fmt.Errorf("scoring profile %q not found", args[0])
	}

	cfg.SetProjectScoringProfile(project.ID, profile.ID)
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("Assigned profile %q to project %q\n", profile.Name, project.Name)
	return nil
}

func runScoreUnassign(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return fmt.Errorf("%w. Select one with: tt select", tterrors.ErrNoCurrentProject)
	}

	cfg.UnsetProjectScoringProfile(project.ID)
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("Removed scoring profile from project %q\n", project.Name)
	return nil
}
