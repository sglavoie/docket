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
	editName         string
	editContext      string
	editDescription  string
	editStatusIcon   string
	projectEditJSON  bool
	projectEditQuiet bool
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

	projectEditCmd.Flags().BoolVar(&projectEditJSON, "json", false, "output updated project as JSON")
	projectEditCmd.Flags().BoolVar(&projectEditQuiet, "quiet", false, "suppress output on success")

	_ = projectEditCmd.RegisterFlagCompletionFunc("context", completeContexts())
	_ = projectEditCmd.RegisterFlagCompletionFunc("status-icon", completeEnum(statusIconCompletions))
}

func runProjectEdit(cmd *cobra.Command, args []string) error {
	out := MutateOutput{JSON: projectEditJSON, Quiet: projectEditQuiet}
	if err := out.CheckMutualExclusion(); err != nil {
		return err
	}

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
		return tterrors.NewNoChangesError("--name, --context, --description, or --status-icon")
	}

	// Validate and apply name change
	if nameChanged {
		newName := strings.TrimSpace(editName)
		if newName == "" {
			return tterrors.NewRequiredFieldError("project name")
		}

		if len(newName) > models.MaxProjectNameLength {
			return fmt.Errorf("project name too long (%d chars, max %d)", len(newName), models.MaxProjectNameLength)
		}

		// Generate new slug and check for uniqueness
		newSlug := models.Slugify(newName)
		if newSlug == "" {
			return tterrors.NewInvalidValueError("project name", newName, "produces an empty slug")
		}

		// Reject reserved slugs (same check as project new)
		reservedSlugs := map[string]bool{
			".tt": true, "projects": true,
			"new": true, "list": true, "delete": true, "switch": true, "archive": true,
		}
		if reservedSlugs[newSlug] {
			return fmt.Errorf("%w: '%s' is a reserved name", tterrors.ErrInvalidValue, newSlug)
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
	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	// When the name changes, update the tasks.md heading so the project name
	// in the file stays in sync with the config. This matters for project
	// restore (which reads the name from tasks.md) and human readability.
	if nameChanged {
		if err := storage.LoadProject(project); err == nil {
			if err := storage.SaveProjectLocked(project); err != nil {
				return fmt.Errorf("failed to update tasks file after rename: %w", err)
			}
		}
		// If LoadProject fails (e.g., no tasks.md yet), skip silently —
		// the heading will be written correctly on the next task save.
	}

	return out.OutputProject(project, func() {
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
	})
}
