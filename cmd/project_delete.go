package cmd

import (
	"encoding/json"
	"fmt"
	"os"

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

	project, workspace, err := LoadRuntimeProject(args[0])
	if err != nil {
		return err
	}

	if project.Sandbox {
		return fmt.Errorf("cannot delete the sandbox project; use 'tt sandbox reset' to clear its contents instead")
	}
	if workspace.Runtime.IsSystemInboxProject(project.ID) {
		return fmt.Errorf("cannot delete the system Inbox project")
	}

	// Warn about cross-project references that will be cleaned up
	projects := workspace.Runtime.AllProjects()
	if refCount, projectCount := countCrossProjectRefs(projects, project.Slug, project.ID); refCount > 0 {
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
	configRefs := marshalProjectReferences(workspace.Runtime, project.ID, project.Slug)

	// Remove the project and every owning-domain reference on a defensive domain
	// copy; the synced artifacts ride the same delete transaction.
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}
	domains.RemoveProjectReferences(project.ID, project.Slug)

	// Plan the cross-project reference repair now so each remaining project's
	// tasks.md is rewritten INSIDE the atomic delete transaction rather than as a
	// separate best-effort save afterwards. The rewrite loader retains both ID-
	// and slug-qualified refs until this identity-aware cleanup filters them.
	crossRefWrites, cleanedRefs := planCrossProjectRefCleanup(projects, project.Slug, project.ID)

	configArts, err := encodeSyncedDomainArtifacts(workspace.Runtime, domains)
	if err != nil {
		return err
	}

	// Route the delete through the application layer: the project directory move to a
	// new versioned trash entry, the tombstone write, the config rewrite, and the
	// cross-project reference repairs all commit as ONE atomic journaled transaction.
	// A failure leaves the project untouched (still under projects/, still in config,
	// references intact), so no manual rollback is needed. Existing trash entries that
	// share this slug are preserved.
	svc := newWorkspaceAppService(workspace)
	if _, err := svc.DeleteProject(app.DeleteProjectRequest{
		Project:        project,
		DataDir:        workspace.Runtime.DataDir(),
		Reason:         "deleted via CLI",
		ConfigRefs:     configRefs,
		Config:         configArts,
		CrossRefWrites: crossRefWrites,
		Confirm:        true,
	}); err != nil {
		return translateAppError(err)
	}
	if err := workspace.Runtime.CommitMachine(domains.Machine); err != nil {
		return fmt.Errorf("project deleted but machine-local references could not be saved: %w", err)
	}

	if cleanedRefs > 0 && !out.Quiet {
		fmt.Printf("Cleaned %s from other projects.\n",
			Pluralize(cleanedRefs, "cross-project reference", "cross-project references"))
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

// planCrossProjectRefCleanup rewrites every remaining project's tasks.md to drop
// cross-project dependency and relation references to the deleted project, and
// returns those rewrites as artifact writes (for the atomic delete transaction)
// plus the total number of references cleaned. It must be called BEFORE the
// deleted project's directory is trashed so the resulting revision-checked
// rewrites can join the same transaction. Projects that fail to load are skipped,
// matching the pre-migration best-effort behavior.
func planCrossProjectRefCleanup(projects []*models.Project, deletedProjectSlug, deletedProjectID string) ([]storage.ArtifactChange, int) {
	var writes []storage.ArtifactChange
	totalCleaned := 0

	for _, p := range projects {
		if p.ID == deletedProjectID {
			continue // the deleted project is already out of the catalog; skip defensively
		}
		expectedRevision, err := storage.LoadProjectForRewrite(p)
		if err != nil {
			continue
		}

		cleaned := 0
		for _, task := range p.Tasks {
			origDeps := len(task.DependsOn)
			task.DependsOn = filterRefs(task.DependsOn, deletedProjectSlug, deletedProjectID)
			cleaned += origDeps - len(task.DependsOn)

			origRels := len(task.RelatedTo)
			task.RelatedTo = filterRefs(task.RelatedTo, deletedProjectSlug, deletedProjectID)
			cleaned += origRels - len(task.RelatedTo)
		}

		if cleaned > 0 {
			content := storage.WriteTasksMarkdownWithFullProjectMetadata(
				p.Name, p.Tasks, p.GetSections(), p.SectionColors, p.Tags, p.PinnedTags)
			writes = append(writes, storage.ArtifactChange{
				Path:             storage.TasksFilePath(p.Path),
				Data:             []byte(content),
				ExpectedRevision: expectedRevision,
			})
			totalCleaned += cleaned
		}
	}

	return writes, totalCleaned
}

// countCrossProjectRefs counts references to the deleted project by resolved
// project identity, accepting both current stable-ID qualifiers and legacy slugs.
func countCrossProjectRefs(projects []*models.Project, deletedProjectSlug, deletedProjectID string) (int, int) {
	totalRefs := 0
	projectsWithRefs := 0
	for _, p := range projects {
		if _, err := storage.LoadProjectForRewrite(p); err != nil {
			continue
		}
		refs := 0
		for _, task := range p.Tasks {
			for _, ref := range task.DependsOn {
				parsed := models.ParseTaskRef(ref)
				if refTargetsProject(parsed, deletedProjectSlug, deletedProjectID) {
					refs++
				}
			}
			for _, ref := range task.RelatedTo {
				parsed := models.ParseTaskRef(ref)
				if refTargetsProject(parsed, deletedProjectSlug, deletedProjectID) {
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
func marshalProjectReferences(runtime *config.RuntimeState, id, slug string) json.RawMessage {
	refs := runtime.ExtractProjectReferences(id, slug)
	raw, err := json.Marshal(refs)
	if err != nil {
		return nil
	}
	return raw
}

// filterRefs returns a new slice with cross-project refs to the given project
// removed, regardless of whether the persisted qualifier is its ID or slug.
func filterRefs(refs []string, deletedProjectSlug, deletedProjectID string) []string {
	if len(refs) == 0 {
		return refs
	}
	filtered := make([]string, 0, len(refs))
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if refTargetsProject(parsed, deletedProjectSlug, deletedProjectID) {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
}

func refTargetsProject(ref models.TaskRef, projectSlug, projectID string) bool {
	return ref.IsCrossProject() && (ref.ProjectID == projectID || ref.ProjectID == projectSlug)
}
