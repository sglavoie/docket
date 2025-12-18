package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sglavoie/dev-helpers/go/tt/internal/config"
	"github.com/sglavoie/dev-helpers/go/tt/internal/models"
	"github.com/sglavoie/dev-helpers/go/tt/internal/storage"
	"github.com/spf13/cobra"
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
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get project name
	var name string
	if len(args) > 0 {
		name = strings.TrimSpace(args[0])
	} else {
		return fmt.Errorf("project name is required")
	}

	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	// Check for duplicate slug
	slug := models.Slugify(name)
	for _, p := range cfg.Projects {
		if p.Slug == slug {
			return fmt.Errorf("project with slug '%s' already exists", slug)
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
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create project directory and initial tasks.md
	if err := storage.CreateProjectDir(project); err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	// Add to config and set as current
	cfg.AddProject(project)
	cfg.CurrentProject = project.ID

	if err := configManager.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Created project '%s' (%s)\n", name, slug)
	fmt.Printf("Project path: %s\n", projectPath)

	return nil
}
