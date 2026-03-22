package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

// Note: config import kept for listTrashedProjects function signature

var projectRestoreCmd = &cobra.Command{
	Use:   "restore [slug]",
	Short: "Restore a deleted project from trash",
	Long: `Restore a previously deleted project from the trash folder.

If no slug is provided, lists all projects in trash.
Use 'tt project delete <name>' to delete a project.

Examples:
  tt project restore              # List trashed projects
  tt project restore my-project   # Restore specific project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectRestore,
}

func init() {
	projectCmd.AddCommand(projectRestoreCmd)
	addOutputFlags(projectRestoreCmd)
}

func runProjectRestore(cmd *cobra.Command, args []string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// If no args, list trashed projects
	if len(args) == 0 {
		return listTrashedProjects(cfg)
	}

	slug := args[0]

	// Check if project exists in trash
	if !storage.TrashEntryExists(slug, cfg.DataDir) {
		return tterrors.NewProjectNotFoundInTrashError(slug)
	}

	// Check if active project with same slug exists
	if cfg.GetProject(slug) != nil {
		return fmt.Errorf("active project '%s' already exists; rename or delete it first", slug)
	}

	// Backup config before making changes
	if backupErr := configManager.BackupConfig(); backupErr != nil {
		return fmt.Errorf("failed to create config backup: %w", backupErr)
	}

	// Restore project directory from trash
	if restoreErr := storage.RestoreProjectFromTrash(slug, cfg.DataDir); restoreErr != nil {
		return fmt.Errorf("failed to restore project from trash: %w", restoreErr)
	}

	// Reconstruct project metadata
	projectPath := cfg.GetProjectPath(slug)
	project, err := reconstructProjectFromDir(slug, projectPath)
	if err != nil {
		// Move back to trash on failure (best effort)
		if trashErr := storage.MoveProjectToTrash(&models.Project{Slug: slug, Path: projectPath}, cfg.DataDir); trashErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to move project back to trash after reconstruction failure: %v\n", trashErr)
		}
		return fmt.Errorf("failed to reconstruct project metadata: %w", err)
	}

	// Add project to config
	cfg.AddProject(project)

	// Save config
	if err := saveConfig(configManager, cfg); err != nil {
		// Move back to trash on failure (best effort)
		if trashErr := storage.MoveProjectToTrash(project, cfg.DataDir); trashErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to move project back to trash after config save failure: %v\n", trashErr)
		}
		return err
	}

	return out.OutputProject(project, func() { fmt.Println(successMsg("Restored project '%s'", project.Name)) })
}

func listTrashedProjects(cfg *config.Config) error {
	projects, err := storage.ListTrashedProjects(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to list trashed projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects in trash.")
		return nil
	}

	fmt.Println("Projects in trash:")
	for _, slug := range projects {
		fmt.Printf("  %s\n", slug)
	}
	fmt.Println("\nTo restore: tt project restore <slug>")
	return nil
}

func reconstructProjectFromDir(slug, projectPath string) (*models.Project, error) {
	// Create a temporary project to load tasks
	tempProject := &models.Project{
		Path: projectPath,
	}

	// Try to load and parse tasks.md to get project name
	if err := storage.LoadProject(tempProject); err != nil {
		return nil, fmt.Errorf("failed to load project tasks: %w", err)
	}

	// Read the raw file to get the project name from the header
	projectName := slug // Default to slug if we can't find the name
	tasksPath := filepath.Join(projectPath, "tasks.md")
	content, err := storage.ParseProjectNameFromFile(tasksPath)
	if err == nil && content != "" {
		projectName = content
	}

	// Create the project with a new ID (since we don't store ID in tasks.md)
	project := &models.Project{
		ID:            uuid.New().String(),
		Slug:          slug,
		Name:          projectName,
		Path:          projectPath,
		Context:       "personal", // Default context
		CreatedAt:     time.Now(), // We don't preserve original creation time
		UpdatedAt:     time.Now(),
		Tasks:         tempProject.Tasks,
		NextShortID:   tempProject.NextShortID,
		Sections:      tempProject.Sections,
		SectionColors: tempProject.SectionColors,
	}

	return project, nil
}
