package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var scoreAssignCmd = &cobra.Command{
	Use:   "assign <profile-id>",
	Short: "Assign a scoring profile to the current project",
	Long: `Assign a scoring profile to the current project. Once assigned,
tasks can be scored and ranked using the profile's formula and dimensions.

Use 'tt score profile list' to see available profiles.

Examples:
  tt score assign rice
  tt score assign my-custom-profile`,
	RunE: runScoreAssign,
}

var scoreUnassignCmd = &cobra.Command{
	Use:   "unassign",
	Short: "Remove scoring profile from the current project",
	Long: `Remove the scoring profile assignment from the current project.
Existing dimension values on tasks are preserved but scoring/ranking
will not be available until a profile is reassigned.

Examples:
  tt score unassign`,
	RunE: runScoreUnassign,
}

func init() {
	scoreCmd.AddCommand(scoreAssignCmd)
	scoreCmd.AddCommand(scoreUnassignCmd)
	addOutputFlags(scoreAssignCmd)
	addOutputFlags(scoreUnassignCmd)
}

func runScoreAssign(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("profile ID")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return NoCurrentProjectError()
	}

	profile := cfg.GetScoringProfile(args[0])
	if profile == nil {
		return fmt.Errorf("scoring profile %q not found", args[0])
	}

	cfg.SetProjectScoringProfile(project.ID, profile.ID)
	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ProfileID   string `json:"profile_id"`
			ProfileName string `json:"profile_name"`
			ProjectID   string `json:"project_id"`
			ProjectName string `json:"project_name"`
		}{ProfileID: profile.ID, ProfileName: profile.Name, ProjectID: project.ID, ProjectName: project.Name})
	}
	fmt.Printf("Assigned profile %q to project %q\n", profile.Name, project.Name)
	return nil
}

func runScoreUnassign(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return NoCurrentProjectError()
	}

	cfg.UnsetProjectScoringProfile(project.ID)
	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			ProjectID   string `json:"project_id"`
			ProjectName string `json:"project_name"`
		}{ProjectID: project.ID, ProjectName: project.Name})
	}
	fmt.Printf("Removed scoring profile from project %q\n", project.Name)
	return nil
}
