package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/tterrors"
)

var scoreProfileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available scoring profiles",
	RunE:  runScoreProfileList,
}

var scoreProfileAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a scoring profile from a preset or interactively",
	Long: `Add a scoring profile.

Use --preset to add a built-in profile (rice, wsjf, ice).
Without --preset, creates a custom weighted-sum profile interactively.

Examples:
  tt score profile add --preset rice
  tt score profile add --preset wsjf
  tt score profile add --id custom --name "My Score" --dim "impact:1-10:5:2" --dim "effort:1-5:3:1"`,
	RunE: runScoreProfileAdd,
}

var scoreProfileShowCmd = &cobra.Command{
	Use:   "show <profile-id>",
	Short: "Show profile dimensions and formula",
	RunE:  runScoreProfileShow,
}

var scoreProfileEditCmd = &cobra.Command{
	Use:   "edit <profile-id>",
	Short: "Edit a profile's dimensions and weights",
	Long: `Edit a scoring profile's dimensions.

Modify dimensions with --dim flags. Each dimension is specified as:
  <id>:<min>-<max>:<default>:<weight>

Examples:
  tt score profile edit rice --dim "reach:1-10:5:2" --dim "effort:1-10:5:1"
  tt score profile edit custom --name "Updated Name"`,
	RunE: runScoreProfileEdit,
}

var scoreProfileDeleteCmd = &cobra.Command{
	Use:   "delete <profile-id>",
	Short: "Remove a scoring profile",
	RunE:  runScoreProfileDelete,
}

func init() {
	scoreProfileCmd.AddCommand(scoreProfileListCmd)
	scoreProfileCmd.AddCommand(scoreProfileAddCmd)
	scoreProfileCmd.AddCommand(scoreProfileShowCmd)
	scoreProfileCmd.AddCommand(scoreProfileEditCmd)
	scoreProfileCmd.AddCommand(scoreProfileDeleteCmd)

	scoreProfileAddCmd.Flags().String("preset", "", "use a built-in preset (rice, wsjf, ice)")
	scoreProfileAddCmd.Flags().String("id", "", "custom profile ID")
	scoreProfileAddCmd.Flags().String("name", "", "custom profile name")
	scoreProfileAddCmd.Flags().StringSlice("dim", nil, "dimension spec: id:min-max:default:weight (repeatable)")

	scoreProfileEditCmd.Flags().String("name", "", "new profile name")
	scoreProfileEditCmd.Flags().StringSlice("dim", nil, "dimension spec: id:min-max:default:weight (replaces matching dim)")
}

func runScoreProfileList(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	automation := workspace.Runtime.Automation()
	if len(automation.ScoringProfiles) == 0 {
		fmt.Println("No scoring profiles configured.")
		fmt.Println("Add a preset with: tt score profile add --preset rice")
		return nil
	}

	for _, p := range automation.ScoringProfiles {
		builtinTag := ""
		if p.Builtin {
			builtinTag = " (built-in)"
		}
		fmt.Printf("  %s - %s%s\n", p.ID, scoring.FormatProfileSummary(&p), builtinTag)
	}
	return nil
}

func runScoreProfileAdd(cmd *cobra.Command, args []string) error {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	automation := domains.Automation

	preset, _ := cmd.Flags().GetString("preset")
	if preset != "" {
		factories := scoring.PresetProfiles()
		factory, ok := factories[preset]
		if !ok {
			return fmt.Errorf("unknown preset %q (available: %s)", preset, strings.Join(scoring.PresetNames(), ", "))
		}
		profile := factory()
		if addErr := automation.AddScoringProfile(profile); addErr != nil {
			return addErr
		}
		err = commitCLIAutomation(workspace, automation)
		if err != nil {
			return err
		}
		fmt.Printf("Added scoring profile: %s\n", scoring.FormatProfileSummary(&profile))
		return nil
	}

	// Custom profile creation
	id, _ := cmd.Flags().GetString("id")
	name, _ := cmd.Flags().GetString("name")
	dims, _ := cmd.Flags().GetStringSlice("dim")

	if id == "" {
		return fmt.Errorf("--id is required for custom profiles")
	}
	if name == "" {
		name = id
	}
	if len(dims) == 0 {
		return fmt.Errorf("at least one --dim is required for custom profiles")
	}

	dimensions, err := parseDimensionSpecs(dims)
	if err != nil {
		return err
	}

	profile := scoring.ScoringProfile{
		ID:         id,
		Name:       name,
		Formula:    scoring.FormulaWeightedSum,
		Dimensions: dimensions,
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if err := automation.AddScoringProfile(profile); err != nil {
		return err
	}
	if err := commitCLIAutomation(workspace, automation); err != nil {
		return err
	}
	fmt.Printf("Added scoring profile: %s\n", scoring.FormatProfileSummary(&profile))
	return nil
}

func runScoreProfileShow(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("profile name")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	profile := workspace.Runtime.Automation().GetScoringProfile(args[0])
	if profile == nil {
		return fmt.Errorf("scoring profile %q not found", args[0])
	}

	fmt.Printf("Profile: %s (%s)\n", profile.Name, profile.ID)
	if profile.Description != "" {
		fmt.Printf("Description: %s\n", profile.Description)
	}
	if profile.UsageHint != "" {
		fmt.Printf("Best for: %s\n", profile.UsageHint)
	}
	fmt.Printf("Formula: %s - %s\n", profile.Formula, scoring.FormulaDescription(profile.Formula))
	if profile.Builtin {
		fmt.Println("Type: built-in preset")
	}
	fmt.Println("\nDimensions:")
	for _, d := range profile.Dimensions {
		sourceStr := string(d.Source)
		if d.DerivedMapping != nil {
			sourceStr += fmt.Sprintf(" (from %s)", d.DerivedMapping.Field)
		}
		roleStr := ""
		if d.Role != "" {
			roleStr = fmt.Sprintf(" [%s]", d.Role)
		}
		fmt.Printf("  %s (%s): range %d-%d, default %d, weight %.1f, %s%s\n",
			d.Name, d.ID, d.Min, d.Max, d.Default, d.Weight, sourceStr, roleStr)
	}
	return nil
}

func runScoreProfileEdit(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("profile name")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	automation := domains.Automation

	profile := automation.GetScoringProfile(args[0])
	if profile == nil {
		return fmt.Errorf("scoring profile %q not found", args[0])
	}

	if profile.Builtin {
		return fmt.Errorf("scoring profile %q is a built-in preset and cannot be edited", args[0])
	}

	if name, _ := cmd.Flags().GetString("name"); name != "" {
		profile.Name = name
	}

	if dims, _ := cmd.Flags().GetStringSlice("dim"); len(dims) > 0 {
		newDims, err := parseDimensionSpecs(dims)
		if err != nil {
			return err
		}
		// Replace matching dimensions by ID, add new ones
		for _, nd := range newDims {
			replaced := false
			for i := range profile.Dimensions {
				if profile.Dimensions[i].ID == nd.ID {
					profile.Dimensions[i] = nd
					replaced = true
					break
				}
			}
			if !replaced {
				profile.Dimensions = append(profile.Dimensions, nd)
			}
		}
	}

	if err := profile.Validate(); err != nil {
		return err
	}
	if err := commitCLIAutomation(workspace, automation); err != nil {
		return err
	}
	fmt.Printf("Updated profile: %s\n", scoring.FormatProfileSummary(profile))
	return nil
}

func runScoreProfileDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("profile name")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		return err
	}
	automation := domains.Automation

	if existing := automation.GetScoringProfile(args[0]); existing != nil && existing.Builtin {
		return fmt.Errorf("scoring profile %q is a built-in preset and cannot be deleted", args[0])
	}

	if !automation.RemoveScoringProfile(args[0]) {
		return fmt.Errorf("scoring profile %q not found", args[0])
	}

	if err := commitCLIAutomation(workspace, automation); err != nil {
		return err
	}
	fmt.Printf("Deleted scoring profile %q\n", args[0])
	return nil
}

// parseDimensionSpecs parses dimension spec strings in the format "id:min-max:default:weight".
func parseDimensionSpecs(specs []string) ([]scoring.ScoringDimension, error) {
	var dims []scoring.ScoringDimension
	for _, spec := range specs {
		parts := strings.Split(spec, ":")
		if len(parts) != 4 {
			return nil, fmt.Errorf("invalid dimension spec %q: expected id:min-max:default:weight", spec)
		}

		id := parts[0]
		rangeParts := strings.Split(parts[1], "-")
		if len(rangeParts) != 2 {
			return nil, fmt.Errorf("invalid range in dimension spec %q: expected min-max", spec)
		}

		min, err := strconv.Atoi(rangeParts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid min in dimension spec %q: %w", spec, err)
		}
		max, err := strconv.Atoi(rangeParts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid max in dimension spec %q: %w", spec, err)
		}
		def, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid default in dimension spec %q: %w", spec, err)
		}
		weight, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid weight in dimension spec %q: %w", spec, err)
		}

		dims = append(dims, scoring.ScoringDimension{
			ID:      id,
			Name:    id, // Use ID as name for custom dimensions
			Min:     min,
			Max:     max,
			Default: def,
			Weight:  weight,
			Source:  scoring.SourceManual,
		})
	}
	return dims, nil
}
