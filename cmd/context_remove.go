package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tterrors"
)

var contextRemoveCmd = &cobra.Command{
	Use:   "remove <context-name> <project-slug>",
	Short: "Remove a project from a custom context",
	Long: `Remove a project from a custom context.

Examples:
  tt context remove Work my-project
  tt ctx remove "Side Projects" cool-app`,
	RunE: runContextRemove,
}

func init() {
	contextCmd.AddCommand(contextRemoveCmd)
	addOutputFlags(contextRemoveCmd)
}

func runContextRemove(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return tterrors.NewRequiredFieldError("context name and project slug")
	}

	contextName := args[0]
	projectSlug := args[1]

	cfg, mgr, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
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

	// Find and remove project from context
	found := false
	var newIDs []string
	for _, id := range projectIDs {
		if id == project.ID {
			found = true
			continue
		}
		newIDs = append(newIDs, id)
	}

	if !found {
		return fmt.Errorf("project '%s' is not in custom context '%s'", project.Name, contextName)
	}

	cfg.SetCustomContext(contextName, newIDs)

	if err := saveConfig(mgr, cfg); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		data, _ := json.MarshalIndent(struct {
			Context string `json:"context"`
			Project string `json:"project"`
		}{Context: contextName, Project: project.Name}, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Removed '%s' from custom context '%s'\n", project.Name, contextName)
	return nil
}
