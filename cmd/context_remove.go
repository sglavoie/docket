package cmd

import (
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

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}
	machine := domains.Machine

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Check if custom context exists
	projectIDs := machine.GetCustomContext(contextName)
	if projectIDs == nil {
		return fmt.Errorf("custom context '%s' not found", contextName)
	}

	// Find project by slug
	project := workspace.Runtime.GetProject(projectSlug)
	if project == nil {
		return tterrors.NewProjectNotFoundError(projectSlug)
	}

	// Find and remove project from context
	found := false
	newIDs := make([]string, 0, len(projectIDs))
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

	machine.SetCustomContext(contextName, newIDs)

	if err := workspace.Runtime.CommitMachine(machine); err != nil {
		return err
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(struct {
			Context string `json:"context"`
			Project string `json:"project"`
		}{Context: contextName, Project: project.Name})
	}
	fmt.Printf("Removed '%s' from custom context '%s'\n", project.Name, contextName)
	return nil
}
