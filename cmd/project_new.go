package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var (
	projectContext     string
	projectDescription string
)

var projectNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new project",
	Long: `Create a new project with the given name.

Examples:
  tt project new "My Project"
  tt project new "Work Tasks" --context work
  tt project new "Side Project" --description "A cool side project"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectNew,
}

func init() {
	projectCmd.AddCommand(projectNewCmd)

	projectNewCmd.Flags().StringVarP(&projectContext, "context", "c", "personal", "project context (personal or work)")
	projectNewCmd.Flags().StringVarP(&projectDescription, "description", "d", "", "project description")
}

func runProjectNew(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	// Get project name
	var name string
	if len(args) > 0 {
		name = strings.TrimSpace(args[0])
	} else {
		return tterrors.NewRequiredFieldError("name")
	}

	if name == "" {
		return tterrors.NewInvalidValueError("name", name, "cannot be empty")
	}

	// Check for duplicate slug
	slug := models.Slugify(name)
	for _, p := range cfg.Projects {
		if p.Slug == slug {
			return fmt.Errorf("%w: '%s'", tterrors.ErrProjectExists, slug)
		}
	}

	// Create project
	projectID := uuid.New().String()
	projectPath := cfg.GetProjectPath(slug)

	project := models.NewProject(projectID, name, projectPath)
	project.Description = projectDescription
	project.Context = projectContext

	// Ensure data directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("%w: data directory: %w", tterrors.ErrWriteFailed, err)
	}

	// Purge any existing trash entry with same slug to allow recreation
	if err := storage.PurgeFromTrash(slug, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to purge old trash entry: %w", err)
	}

	// Create project directory and initial tasks.md
	if err := storage.CreateProjectDir(project); err != nil {
		return tterrors.NewSaveProjectError(name, err)
	}

	// Add to config and set as current
	cfg.AddProject(project)
	cfg.CurrentProject = project.ID

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("%w: %w", tterrors.ErrConfigSaveFailed, err)
	}

	fmt.Printf("Created project '%s' (%s)\n", name, slug)
	fmt.Printf("Project path: %s\n", projectPath)

	return nil
}
