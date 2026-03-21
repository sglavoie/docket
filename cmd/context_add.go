package cmd

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var contextAddCmd = &cobra.Command{
	Use:   "add <context-name> <project-slug>",
	Short: "Add a project to a custom context",
	Long: `Add a project to an existing custom context.

Custom contexts are named groups of projects that you can switch between
(see 'tt context new' and 'tt context list').

Examples:
  tt context add Work my-project
  tt ctx add "Side Projects" cool-app`,
	RunE: runContextAdd,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cfg, _, err := LoadConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		if len(args) == 0 {
			return cfg.GetCustomContextNames(), cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			var slugs []string
			for _, p := range cfg.Projects {
				slugs = append(slugs, p.Slug)
			}
			return slugs, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	contextCmd.AddCommand(contextAddCmd)
	addOutputFlags(contextAddCmd)
}

func runContextAdd(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("context name and project slug")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	contextName := args[0]
	projectSlug := args[1]

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check if custom context exists
	projectIDs := cfg.GetCustomContext(contextName)
	if projectIDs == nil {
		return fmt.Errorf("custom context '%s' not found; use 'tt context list' to see contexts", contextName)
	}

	// Find project by slug
	project := cfg.GetProject(projectSlug)
	if project == nil {
		return tterrors.NewProjectNotFoundError(projectSlug)
	}

	// Check if already in context
	if slices.Contains(projectIDs, project.ID) {
		return fmt.Errorf("project '%s' is already in custom context '%s'", project.Name, contextName)
	}

	// Add project
	projectIDs = append(projectIDs, project.ID)
	cfg.SetCustomContext(contextName, projectIDs)

	if err := saveConfig(mgr, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		entry := struct {
			Context string `json:"context"`
			Project string `json:"project"`
		}{
			Context: contextName,
			Project: project.Slug,
		}
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling context: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Added project '%s' (%s) to custom context '%s'\n", project.Name, project.Slug, contextName)
	return nil
}
