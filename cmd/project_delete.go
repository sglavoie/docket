package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectDeleteForce bool

var projectDeleteCmd = &cobra.Command{
	Use:   "delete [name|id]",
	Short: "Delete a project",
	Long: `Delete a project by moving it to the trash folder.

The project directory is moved to ~/.tt-data/.trash/ and can be restored later.
Use 'tt project restore <slug>' to restore a deleted project.

Examples:
  tt project delete "Old Project"
  tt p delete my-project`,
	RunE: runProjectDelete,
}

func init() {
	projectCmd.AddCommand(projectDeleteCmd)
	projectDeleteCmd.Flags().BoolVarP(&projectDeleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	project, cfg, configManager, err := LoadConfigAndProject(args[0])
	if err != nil {
		return err
	}

	if project.Sandbox {
		return fmt.Errorf("cannot delete the sandbox project; use 'tt sandbox reset' to clear its contents instead")
	}

	if !ConfirmStrict(fmt.Sprintf("Delete project '%s'? It will be moved to trash.", project.Name), projectDeleteForce) {
		fmt.Println("Canceled")
		return nil
	}

	// Purge any existing trash entry with same slug
	if err := storage.PurgeFromTrash(project.Slug, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to purge old trash entry: %w", err)
	}

	// Backup config before making changes
	if err := configManager.BackupConfig(); err != nil {
		return fmt.Errorf("failed to create config backup: %w", err)
	}

	// Move project directory to trash
	if err := storage.MoveProjectToTrash(project, cfg.DataDir); err != nil {
		return fmt.Errorf("failed to move project to trash: %w", err)
	}

	// Remove project from config
	cfg.RemoveProject(project.ID)

	// Clean cross-project references in remaining projects
	if cleaned, err := cleanCrossProjectRefs(cfg.Projects, project.ID); err != nil {
		fmt.Printf("Warning: failed to clean some cross-project references: %v\n", err)
	} else if cleaned > 0 {
		// Summary is printed inside cleanCrossProjectRefs per project
		_ = cleaned
	}

	// Save config
	if err := configManager.Save(cfg); err != nil {
		// Try to restore from trash on failure
		if restoreErr := storage.RestoreProjectFromTrash(project.Slug, cfg.DataDir); restoreErr != nil {
			return fmt.Errorf("config save failed and restore from trash also failed: %w (restore error: %w)", err, restoreErr)
		}
		return tterrors.NewSaveConfigError(err)
	}

	fmt.Printf("Deleted project '%s'\n", project.Name)
	fmt.Printf("To restore: tt project restore %s\n", project.Slug)
	return nil
}

// cleanCrossProjectRefs removes cross-project dependency and relation references
// to the deleted project from all remaining projects. It loads each project's tasks,
// filters out references pointing to deletedProjectID, and saves any modified projects.
// Returns the total number of references cleaned across all projects.
func cleanCrossProjectRefs(projects []*models.Project, deletedProjectID string) (int, error) {
	totalCleaned := 0

	for _, p := range projects {
		if err := storage.LoadProject(p); err != nil {
			return totalCleaned, fmt.Errorf("loading project '%s': %w", p.Name, err)
		}

		cleaned := 0
		for _, task := range p.Tasks {
			origDeps := len(task.DependsOn)
			task.DependsOn = filterRefs(task.DependsOn, deletedProjectID)
			cleaned += origDeps - len(task.DependsOn)

			origRels := len(task.RelatedTo)
			task.RelatedTo = filterRefs(task.RelatedTo, deletedProjectID)
			cleaned += origRels - len(task.RelatedTo)
		}

		if cleaned > 0 {
			if err := storage.SaveProject(p); err != nil {
				return totalCleaned, fmt.Errorf("saving project '%s': %w", p.Name, err)
			}
			fmt.Printf("Cleaned %d cross-project references from '%s'\n", cleaned, p.Name)
			totalCleaned += cleaned
		}
	}

	return totalCleaned, nil
}

// filterRefs returns a new slice with cross-project refs to the given project ID removed.
func filterRefs(refs []string, deletedProjectID string) []string {
	if len(refs) == 0 {
		return refs
	}
	filtered := make([]string, 0, len(refs))
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() && parsed.ProjectID == deletedProjectID {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
}
