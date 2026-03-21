package cmd

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var contextAddCmd = &cobra.Command{
	Use:   "add <context-name> <project-slug>",
	Short: "Add a project to a custom context",
	Long: `Add a project to an existing custom context.

Examples:
  tt context add Work my-project
  tt ctx add "Side Projects" cool-app`,
	RunE: runContextAdd,
}

func init() {
	contextCmd.AddCommand(contextAddCmd)
}

func runContextAdd(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("context name and project slug")
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
		return fmt.Errorf("custom context '%s' not found", contextName)
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

	fmt.Printf("Added '%s' to custom context '%s'\n", project.Name, contextName)
	return nil
}
