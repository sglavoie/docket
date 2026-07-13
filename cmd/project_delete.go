package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectDeleteForce bool

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <slug-or-id>",
	Short: "Delete a project",
	Long: `Delete a project by moving it to the trash folder.

The project directory is moved to the trash/ folder in your tt data directory and can be restored later.
Use 'tt project restore <slug>' to restore a deleted project.

Examples:
  tt project delete old-project
  tt p delete my-project`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectDelete,
}

func init() {
	projectCmd.AddCommand(projectDeleteCmd)
	projectDeleteCmd.Flags().BoolVarP(&projectDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(projectDeleteCmd)
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}
	if len(args) > 1 {
		return fmt.Errorf("expected exactly one project slug, got %d arguments", len(args))
	}

	project, cfg, configManager, err := LoadConfigAndProject(args[0])
	if err != nil {
		return err
	}

	if project.Sandbox {
		return fmt.Errorf("cannot delete the sandbox project; use 'tt sandbox reset' to clear its contents instead")
	}

	// Warn about cross-project references that will be cleaned up
	if refCount, projectCount := countCrossProjectRefs(cfg.Projects, project.Slug); refCount > 0 {
		fmt.Printf("Warning: %s in %s will be cleaned up.\n",
			Pluralize(refCount, "cross-project reference", "cross-project references"),
			Pluralize(projectCount, "project", "projects"))
	}

	if !ConfirmStrict(fmt.Sprintf("Delete project '%s'? It will be moved to trash.", project.Name), projectDeleteForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Snapshot the project's configuration references before RemoveProject
	// strips them, so a later restore can faithfully re-establish them.
	configRefs := marshalProjectReferences(cfg, project.ID, project.Slug)

	// Remove the project from the catalog (in-memory) and encode the resulting
	// config as the artifact that will ride the delete transaction.
	cfg.RemoveProject(project.ID)
	configArts, err := encodeConfigArtifacts(configManager, cfg)
	if err != nil {
		return err
	}

	// Route the delete through the application layer: the project directory move to a
	// new versioned trash entry, the tombstone write, and the config rewrite all
	// commit as ONE atomic journaled transaction. A failure leaves the project
	// untouched (still under projects/, still in config), so no manual rollback is
	// needed. Existing trash entries that share this slug are preserved.
	svc := newAppService(cfg)
	if _, err := svc.DeleteProject(app.DeleteProjectRequest{
		Project:    project,
		DataDir:    cfg.DataDir,
		Reason:     "deleted via CLI",
		ConfigRefs: configRefs,
		Config:     configArts,
		Confirm:    true,
	}); err != nil {
		return translateAppError(err)
	}

	// Clean cross-project references in the remaining projects now that the deleted
	// project's directory is gone. This is a best-effort repair separate from the
	// atomic delete: a failure here is a warning, not an error — the project is
	// already trashed and the stale references are harmless dangling links.
	if _, err := cleanCrossProjectRefs(cfg.Projects, project.Slug); err != nil {
		fmt.Printf("Warning: failed to clean some cross-project references: %v\n", err)
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputSingleProjectJSON(project)
	}
	fmt.Println(successMsg("Deleted project '%s'", project.Name))
	fmt.Printf("To restore: tt project restore %s\n", project.Slug)
	return nil
}

// cleanCrossProjectRefs removes cross-project dependency and relation references
// to the deleted project from all remaining projects. It loads each project's tasks,
// filters out references pointing to deletedProjectSlug, and saves any modified projects.
// Returns the total number of references cleaned across all projects.
//
// LoadProjectWithWarnings runs validateCrossProjectRefs at load time, which
// already strips refs to projects whose directory no longer exists under
// projects/ (the case immediately after MoveProjectToTrash). We count those
// load-time strips via warnings so the file is rewritten without the stale
// refs even when the in-memory diff would otherwise show zero changes.
func cleanCrossProjectRefs(projects []*models.Project, deletedProjectSlug string) (int, error) {
	totalCleaned := 0

	for _, p := range projects {
		loadResult, err := storage.LoadProjectWithWarnings(p)
		if err != nil {
			return totalCleaned, fmt.Errorf("loading project '%s': %w", p.Name, err)
		}

		cleaned := 0
		for _, w := range loadResult.Warnings {
			if (w.Field == "depends" || w.Field == "related") && strings.Contains(w.Message, fmt.Sprintf("%q", deletedProjectSlug)) {
				cleaned++
			}
		}

		for _, task := range p.Tasks {
			origDeps := len(task.DependsOn)
			task.DependsOn = filterRefs(task.DependsOn, deletedProjectSlug)
			cleaned += origDeps - len(task.DependsOn)

			origRels := len(task.RelatedTo)
			task.RelatedTo = filterRefs(task.RelatedTo, deletedProjectSlug)
			cleaned += origRels - len(task.RelatedTo)
		}

		if cleaned > 0 {
			if err := storage.SaveProjectLocked(p); err != nil {
				return totalCleaned, fmt.Errorf("saving project '%s': %w", p.Name, err)
			}
			fmt.Printf("Cleaned %d cross-project references from '%s'\n", cleaned, p.Name)
			totalCleaned += cleaned
		}
	}

	return totalCleaned, nil
}

// countCrossProjectRefs counts the total cross-project references to deletedProjectSlug across
// all given projects, and returns the ref count and the number of projects with refs.
func countCrossProjectRefs(projects []*models.Project, deletedProjectSlug string) (int, int) {
	totalRefs := 0
	projectsWithRefs := 0
	for _, p := range projects {
		if err := storage.LoadProject(p); err != nil {
			continue
		}
		refs := 0
		for _, task := range p.Tasks {
			for _, ref := range task.DependsOn {
				parsed := models.ParseTaskRef(ref)
				if parsed.IsCrossProject() && parsed.ProjectID == deletedProjectSlug {
					refs++
				}
			}
			for _, ref := range task.RelatedTo {
				parsed := models.ParseTaskRef(ref)
				if parsed.IsCrossProject() && parsed.ProjectID == deletedProjectSlug {
					refs++
				}
			}
		}
		if refs > 0 {
			totalRefs += refs
			projectsWithRefs++
		}
	}
	return totalRefs, projectsWithRefs
}

// marshalProjectReferences captures the given project's configuration
// references and encodes them for storage in a trash tombstone. A marshal
// failure is non-fatal: the project data is still trashed, only the later
// config-reference restoration would be lost, so we return nil rather than
// abort the delete.
func marshalProjectReferences(cfg *config.Config, id, slug string) json.RawMessage {
	refs := cfg.ExtractProjectReferences(id, slug)
	raw, err := json.Marshal(refs)
	if err != nil {
		return nil
	}
	return raw
}

// filterRefs returns a new slice with cross-project refs to the given project slug removed.
func filterRefs(refs []string, deletedProjectSlug string) []string {
	if len(refs) == 0 {
		return refs
	}
	filtered := make([]string, 0, len(refs))
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() && parsed.ProjectID == deletedProjectSlug {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
}
