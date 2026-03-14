package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	editName        string
	editContext     string
	editDescription string
	editStatusIcon  string
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
  tt project edit --name "New Name" --context personal
  tt project edit --status-icon wip`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectEdit,
}

func init() {
	projectCmd.AddCommand(projectEditCmd)

	projectEditCmd.Flags().StringVarP(&editName, "name", "n", "", "new project name")
	projectEditCmd.Flags().StringVarP(&editContext, "context", "c", "", "new context (e.g., personal, work)")
	projectEditCmd.Flags().StringVarP(&editDescription, "description", "d", "", "new status note")
	projectEditCmd.Flags().StringVarP(&editStatusIcon, "status-icon", "i", "", "project status icon (e.g., wip, active, paused)")
}

func runProjectEdit(cmd *cobra.Command, args []string) error {
	project, cfg, configManager, err := LoadConfigAndProjectOrCurrent(args)
	if err != nil {
		return err
	}

	// Check if any flags were provided
	nameChanged := cmd.Flags().Changed("name")
	contextChanged := cmd.Flags().Changed("context")
	descriptionChanged := cmd.Flags().Changed("description")
	statusIconChanged := cmd.Flags().Changed("status-icon")

	if !nameChanged && !contextChanged && !descriptionChanged && !statusIconChanged {
		return fmt.Errorf("no changes specified; use --name, --context, --description, or --status-icon")
	}

	// Validate and apply name change
	if nameChanged {
		newName := strings.TrimSpace(editName)
		if newName == "" {
			return fmt.Errorf("project name cannot be empty")
		}

		// Generate new slug and check for uniqueness
		newSlug := models.Slugify(newName)
		if newSlug == "" {
			return fmt.Errorf("project name %q produces an empty slug (must contain at least one ASCII letter or digit)", newName)
		}
		for _, p := range cfg.Projects {
			if p.ID != project.ID && p.Slug == newSlug {
				return fmt.Errorf("project with slug '%s' already exists", newSlug)
			}
		}

		// Rename project directory if slug changed (must happen before updating project fields)
		oldSlug := project.Slug
		if oldSlug != newSlug {
			if err := storage.RenameProjectDir(project, oldSlug, newSlug, cfg.DataDir); err != nil {
				return fmt.Errorf("failed to rename project directory: %w", err)
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

	// Apply status icon change
	if statusIconChanged {
		iconKey := strings.TrimSpace(editStatusIcon)
		if iconKey != "" {
			if models.ProjectStatusIconByKey(iconKey) == nil {
				return fmt.Errorf("unknown status icon '%s'; valid keys: planning, wip, active, review, testing, stabilizing, maintenance, bugfix, paused, archived", iconKey)
			}
		}
		project.StatusIcon = iconKey
		if iconKey != "" {
			project.StatusIconSource = "user"
		} else {
			project.StatusIconSource = ""
		}
	}

	// Update timestamp
	project.UpdatedAt = time.Now()

	// Save config
	if err := configManager.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
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
	if statusIconChanged {
		if icon := models.ProjectStatusIconByKey(project.StatusIcon); icon != nil {
			fmt.Printf("  Status icon: %s %s\n", icon.Emoji, icon.Label)
		} else {
			fmt.Printf("  Status icon: (cleared)\n")
		}
	}

	return nil
}
