package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	editName        string
	editContext     string
	editDescription string
)

var projectEditCmd = &cobra.Command{
	Use:   "edit [project]",
	Short: "Edit project details",
	Long: `Edit a project's name, context, or description.

If no project is specified, edits the current project.
Projects can be specified by name, slug, or ID.

Examples:
  tt project edit --name "New Name"
  tt project edit --context work
  tt project edit my-project --description "Updated description"
  tt project edit --name "New Name" --context personal`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectEdit,
}

func init() {
	projectCmd.AddCommand(projectEditCmd)

	projectEditCmd.Flags().StringVarP(&editName, "name", "n", "", "new project name")
	projectEditCmd.Flags().StringVarP(&editContext, "context", "c", "", "new context (e.g., personal, work)")
	projectEditCmd.Flags().StringVarP(&editDescription, "description", "d", "", "new description")
}

func runProjectEdit(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	// Find the target project
	var project *models.Project
	if len(args) > 0 {
		project = cfg.GetProject(args[0])
		if project == nil {
			return fmt.Errorf("project '%s': %w", args[0], tterrors.ErrProjectNotFound)
		}
	} else {
		project = cfg.GetCurrentProject()
		if project == nil {
			return fmt.Errorf("%w; specify a project or use 'tt project switch'", tterrors.ErrNoCurrentProject)
		}
	}

	// Check if any flags were provided
	nameChanged := cmd.Flags().Changed("name")
	contextChanged := cmd.Flags().Changed("context")
	descriptionChanged := cmd.Flags().Changed("description")

	if !nameChanged && !contextChanged && !descriptionChanged {
		return fmt.Errorf("no changes specified; use --name, --context, or --description")
	}

	// Validate and apply name change
	if nameChanged {
		newName := strings.TrimSpace(editName)
		if newName == "" {
			return fmt.Errorf("project name cannot be empty")
		}

		// Generate new slug and check for uniqueness
		newSlug := models.Slugify(newName)
		for _, p := range cfg.Projects {
			if p.ID != project.ID && p.Slug == newSlug {
				return fmt.Errorf("project with slug '%s' already exists", newSlug)
			}
		}

		project.Name = newName
		project.Slug = newSlug
	}

	// Apply context change
	if contextChanged {
		newContext := strings.TrimSpace(editContext)
		if newContext == "" {
			newContext = cfg.DefaultContext
		}
		project.Context = newContext
	}

	// Apply description change
	if descriptionChanged {
		project.Description = strings.TrimSpace(editDescription)
	}

	// Update timestamp
	project.UpdatedAt = time.Now()

	// Save config
	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Updated project '%s'\n", project.Name)
	if nameChanged {
		fmt.Printf("  Name: %s (slug: %s)\n", project.Name, project.Slug)
	}
	if contextChanged {
		fmt.Printf("  Context: %s\n", project.Context)
	}
	if descriptionChanged {
		if project.Description != "" {
			fmt.Printf("  Description: %s\n", project.Description)
		} else {
			fmt.Printf("  Description: (cleared)\n")
		}
	}

	return nil
}
