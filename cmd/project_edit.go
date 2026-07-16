package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
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
	Use:   "edit [slug-or-id]",
	Short: "Edit project details",
	Long: `Edit a project's name, context, or description.

If no project is specified, edits the current project.
Projects can be specified by slug or ID.

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
	projectEditCmd.Flags().StringVarP(&editDescription, "description", "d", "", "new project description")
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

	project, workspace, err := LoadRuntimeProjectOrCurrent(args)
	if err != nil {
		return err
	}

	originalSlug := project.Slug

	// Check if any flags were provided
	nameChanged := cmd.Flags().Changed("name")
	contextChanged := cmd.Flags().Changed("context")
	descriptionChanged := cmd.Flags().Changed("description")
	statusIconChanged := cmd.Flags().Changed("status-icon")
	if nameChanged && workspace.Runtime.IsSystemInboxProject(project.ID) {
		return fmt.Errorf("cannot rename the system Inbox project")
	}

	if !nameChanged && !contextChanged && !descriptionChanged && !statusIconChanged {
		return tterrors.NewNoChangesError("--name, --context, --description, or --status-icon")
	}

	// Validate and apply name change
	if nameChanged {
		if err := applyProjectNameChange(project, editName, workspace.Runtime.AllProjects()); err != nil {
			return err
		}
	}

	// Apply context change
	if contextChanged {
		newContext := strings.TrimSpace(editContext)
		if newContext == "" {
			newContext = workspace.Runtime.DefaultContext()
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

	// A rename touches the project directory, the tasks.md heading, the catalog,
	// and every other project's cross-project references. Commit all of those as
	// one atomic transaction so a partial rename can never leave data and catalog
	// out of sync. A pure metadata edit (context/description/status-icon, or a name
	// change that leaves the slug unchanged still counts as a rename here) that does
	// not touch tasks.md only rewrites the config.
	if nameChanged {
		if err := commitProjectRename(workspace, project, originalSlug, out); err != nil {
			return err
		}
	} else if err := commitProjectCatalog(workspace, project); err != nil {
		return err
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
				fmt.Printf("  Status icon: %s %s\n", icon.DisplayIcon(), icon.Label)
			} else {
				fmt.Printf("  Status icon: (cleared)\n")
			}
		}
	})
}

// applyProjectNameChange validates the new name and applies it to the in-memory
// project. It deliberately performs NO filesystem effect: the directory move,
// tasks.md heading rewrite, catalog rewrite, and cross-reference repair all commit
// together in commitProjectRename so the rename is atomic. Validating here — before
// any commit — keeps a rejected name (empty, too long, reserved, or colliding) from
// touching disk at all.
func applyProjectNameChange(project *models.Project, rawName string, projects []*models.Project) error {
	newName := strings.TrimSpace(rawName)
	if newName == "" {
		return tterrors.NewRequiredFieldError("project name")
	}

	if len(newName) > models.MaxProjectNameLength {
		return fmt.Errorf("project name too long (%d chars, max %d)", len(newName), models.MaxProjectNameLength)
	}

	newSlug := models.Slugify(newName)
	if newSlug == "" {
		return tterrors.NewInvalidValueError("project name", newName, "produces an empty slug")
	}

	if err := models.ValidateProjectSlug(newSlug, false); err != nil {
		return fmt.Errorf("%w: %w", tterrors.ErrInvalidValue, err)
	}

	for _, p := range projects {
		if p.ID != project.ID && p.Slug == newSlug {
			return fmt.Errorf("project with slug '%s' already exists", newSlug)
		}
	}

	project.Name = newName
	project.Slug = newSlug
	return nil
}

// commitProjectRename renders the renamed project's tasks.md heading and every
// referencing project's repaired tasks.md, encodes the updated catalog, and drives
// all of them plus the directory move through one atomic app-layer transaction.
func commitProjectRename(workspace *CLIWorkspace, project *models.Project, originalSlug string, out MutateOutput) error {
	slugChanged := originalSlug != project.Slug

	// Render the renamed project's tasks.md from its current on-disk tasks, loaded
	// at the OLD path before the transaction moves the directory. If the project has
	// no persisted tasks.md, leave the write out — the heading is written correctly
	// on the next task save, matching the pre-migration behavior.
	var tasksMarkdown []byte
	var tasksExpectedRevision *string
	if expected, err := storage.LoadProjectForRewrite(project); err == nil {
		tasksMarkdown = []byte(storage.WriteTasksMarkdownWithFullProjectMetadata(
			project.Name, project.Tasks, project.GetSections(), project.SectionColors, project.Tags, project.PinnedTags))
		tasksExpectedRevision = expected
	}

	// Cross-project reference repair rides the same transaction when the slug
	// changes, so no other project can observe a dangling reference to the old slug.
	var crossRefWrites []storage.ArtifactChange
	var crossRefCount int
	if slugChanged {
		crossRefWrites, crossRefCount = planCrossProjectRefRewrites(workspace.Runtime.AllProjects(), originalSlug, project.Slug, project.ID)
	}

	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}
	domains.Catalog.UpsertProject(project)
	configArts, err := encodeCatalogArtifact(workspace.Runtime, domains.Catalog)
	if err != nil {
		return err
	}

	if _, err := newWorkspaceAppService(workspace).RenameProject(app.RenameProjectRequest{
		Project:               project,
		DataDir:               workspace.Runtime.DataDir(),
		OldSlug:               originalSlug,
		NewSlug:               project.Slug,
		TasksMarkdown:         tasksMarkdown,
		TasksExpectedRevision: tasksExpectedRevision,
		Config:                configArts,
		CrossRefWrites:        crossRefWrites,
	}); err != nil {
		return translateAppError(err)
	}

	if crossRefCount > 0 && !out.Quiet {
		fmt.Printf("Updated %d cross-project reference(s) from '%s' to '%s'\n", crossRefCount, originalSlug, project.Slug)
	}
	return nil
}

// commitProjectCatalog persists a catalog-owned project metadata change through
// the application transaction boundary using a revision-checked workspace.json
// artifact. No planning, settings, automation, or machine domain is rewritten.
func commitProjectCatalog(workspace *CLIWorkspace, project *models.Project) error {
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}
	domains.Catalog.UpsertProject(project)
	artifacts, err := encodeCatalogArtifact(workspace.Runtime, domains.Catalog)
	if err != nil {
		return err
	}
	return translateAppError(newWorkspaceAppService(workspace).CommitConfig(artifacts...))
}

// planCrossProjectRefRewrites rewrites cross-project references in other projects
// when a project's slug changes, returning the tasks.md artifact writes (for the
// atomic rename transaction) and the total number of refs updated. Projects that
// cannot be loaded are skipped, matching the pre-migration best-effort behavior.
func planCrossProjectRefRewrites(projects []*models.Project, originalSlug, newSlug, renamedProjectID string) ([]storage.ArtifactChange, int) {
	var writes []storage.ArtifactChange
	totalUpdated := 0

	for _, p := range projects {
		if p.ID == renamedProjectID {
			continue // skip the renamed project itself
		}
		expectedRevision, err := storage.LoadProjectForRewrite(p)
		if err != nil {
			continue // skip projects that can't be loaded
		}

		projectUpdated := 0
		for _, task := range p.Tasks {
			var n int
			task.DependsOn, n = rewriteRefs(task.DependsOn, originalSlug, newSlug)
			projectUpdated += n
			task.RelatedTo, n = rewriteRefs(task.RelatedTo, originalSlug, newSlug)
			projectUpdated += n
		}

		if projectUpdated > 0 {
			content := storage.WriteTasksMarkdownWithFullProjectMetadata(
				p.Name, p.Tasks, p.GetSections(), p.SectionColors, p.Tags, p.PinnedTags)
			writes = append(writes, storage.ArtifactChange{
				Path:             storage.TasksFilePath(p.Path),
				Data:             []byte(content),
				ExpectedRevision: expectedRevision,
			})
			totalUpdated += projectUpdated
		}
	}

	return writes, totalUpdated
}

// rewriteRefs replaces cross-project references from originalSlug to newSlug.
// Returns the updated slice and the number of refs rewritten.
func rewriteRefs(refs []string, originalSlug, newSlug string) ([]string, int) {
	if len(refs) == 0 {
		return refs, 0
	}
	updated := 0
	result := make([]string, len(refs))
	for i, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() && parsed.ProjectID == originalSlug {
			result[i] = newSlug + ":" + parsed.TaskID
			updated++
		} else {
			result[i] = ref
		}
	}
	return result, updated
}
