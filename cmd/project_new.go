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
	projectStatusIcon  string
)

var projectNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new project",
	Long: `Create a new project with the given name.

Examples:
  tt project new "My Project"
  tt project new "Work Tasks" --context work
  tt project new "Side Project" --description "A cool side project"
  tt project new "Side Project" --status-icon wip`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectNew,
}

func init() {
	projectCmd.AddCommand(projectNewCmd)

	projectNewCmd.Flags().StringVarP(&projectContext, "context", "c", "personal", "project context (personal or work)")
	projectNewCmd.Flags().StringVarP(&projectDescription, "description", "d", "", "project description")
	projectNewCmd.Flags().StringVarP(&projectStatusIcon, "status-icon", "i", "", "project status icon (planning, wip, active, review, testing, stabilizing, maintenance, bugfix, paused, archived)")

	addOutputFlags(projectNewCmd)

	_ = projectNewCmd.RegisterFlagCompletionFunc("context", completeContexts())
	_ = projectNewCmd.RegisterFlagCompletionFunc("status-icon", completeEnum(statusIconCompletions))
}

func runProjectNew(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	// Validate name before loading config
	var name string
	if len(args) > 0 {
		name = strings.TrimSpace(args[0])
	} else {
		return tterrors.NewRequiredFieldError("name")
	}

	if name == "" {
		return tterrors.NewInvalidValueError("name", name, "cannot be empty")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}
	if len(name) > models.MaxProjectNameLength {
		return fmt.Errorf("project name too long (%d chars, max %d)", len(name), models.MaxProjectNameLength)
	}

	// Check for duplicate slug
	slug := models.Slugify(name)
	if slug == "" {
		return fmt.Errorf("project name %q produces an empty slug (must contain at least one ASCII letter or digit)", name)
	}

	// Reject reserved slugs
	reservedSlugs := map[string]bool{
		".tt": true, "projects": true,
		"new": true, "list": true, "delete": true, "switch": true, "archive": true,
	}
	if reservedSlugs[slug] {
		return fmt.Errorf("%w: '%s' is a reserved name", tterrors.ErrInvalidValue, slug)
	}

	for _, p := range workspace.Runtime.AllProjects() {
		if p.Slug == slug {
			return fmt.Errorf("%w: '%s'", tterrors.ErrProjectExists, slug)
		}
	}

	// Validate context (free-form string; common values: personal, work).
	// Trim whitespace before storing so that flag values like "  work  " do
	// not bypass isKnownContext lookups (which compare with EqualFold but no
	// trim) and pollute the config with padded strings.
	trimmedContext := strings.TrimSpace(projectContext)
	if trimmedContext == "" {
		return tterrors.NewInvalidValueError("context", projectContext, "cannot be empty")
	}
	customNames, err := workspace.Dependencies.CustomContextNames()
	if err != nil {
		return err
	}
	if !isKnownContext(trimmedContext, customNames) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: unknown context %q (known: %s)\n", trimmedContext, knownContextList(customNames))
	}

	// Create project
	projectID := uuid.New().String()
	projectPath := workspace.Runtime.ProjectPath(slug)

	project, err := models.NewProject(projectID, name, projectPath)
	if err != nil {
		return err
	}
	project.Description = projectDescription
	project.Context = trimmedContext

	// Validate and apply status icon
	if projectStatusIcon != "" {
		if models.ProjectStatusIconByKey(projectStatusIcon) == nil {
			return fmt.Errorf("unknown status icon '%s'; valid keys: planning, wip, active, review, testing, stabilizing, maintenance, bugfix, paused, archived", projectStatusIcon)
		}
		project.StatusIcon = projectStatusIcon
		project.StatusIconSource = "user"
	}

	// Ensure data directory exists
	if ensureErr := workspace.Runtime.EnsureDataDir(); ensureErr != nil {
		return fmt.Errorf("%w: data directory: %w", tterrors.ErrWriteFailed, ensureErr)
	}

	// The initial v2 tasks document and catalog entry commit together. A same-slug
	// project may still exist in versioned trash; it does not conflict with this
	// active catalog/project path.
	doc, err := storage.CanonicalProjectDocument(project, nil)
	if err != nil {
		return tterrors.NewSaveProjectError(name, err)
	}
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}
	domains.Catalog.UpsertProject(project)
	artifacts, err := encodeCatalogArtifact(workspace.Runtime, domains.Catalog)
	if err != nil {
		return err
	}
	artifacts = append(artifacts, storage.ArtifactChange{
		Path: storage.TasksFilePath(project.Path),
		Data: []byte(doc.WriteV2()),
		Mode: storage.FilePermissions,
	})
	if err := translateAppError(newWorkspaceAppService(workspace).CommitConfig(artifacts...)); err != nil {
		return err
	}
	machine := *domains.Machine
	machine.CurrentProject = project.ID
	if err := workspace.Runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("project created but current selection could not be saved: %w", err)
	}

	return out.OutputProject(project, func() {
		fmt.Println(successMsg("Created project '%s'", name))
		fmt.Printf("  Path: %s\n", projectPath)
	})
}
