package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// A project is created in one place: PrepareProjectCreation validates the names
// against the runtime state and renders the catalog that records them. The
// project hub's create commits that alone; a capture that names a project the
// workspace does not hold yet commits it beside the tasks it files there, so
// both share one validation and one catalog rendering.

// NewProject names one project to create: the identity it takes and the name
// its slug derives from.
type NewProject struct {
	ID   string
	Name string
}

// ProjectCreation is a set of brand-new empty projects and the catalog
// artifact that records them, stamped with the catalog revision the runtime
// state observed, so a concurrent catalog write refuses the commit carrying it.
type ProjectCreation struct {
	Projects []*models.Project
	Catalog  storage.ArtifactChange
}

// CloneCatalog copies a catalog deeply enough that upserting or removing a
// project on the copy leaves the original untouched.
func CloneCatalog(catalog *config.WorkspaceCatalog) *config.WorkspaceCatalog {
	cp := *catalog
	cp.Projects = slices.Clone(catalog.Projects)
	return &cp
}

// ProjectSlug validates a project name and returns the slug it would take,
// refusing one another project already holds. selfID is the project being
// renamed, or empty for a new one.
func ProjectSlug(state *config.RuntimeState, name, selfID string) (string, error) {
	if name == "" {
		return "", invalidInput("project name", "name cannot be empty")
	}
	slug := models.Slugify(name)
	if slug == "" {
		return "", invalidInput("project name", "name must contain at least one ASCII letter or digit")
	}
	if existing := state.GetProject(slug); existing != nil && existing.ID != selfID {
		return "", invalidInput("project name", fmt.Sprintf("project with slug '%s' already exists", slug))
	}
	return slug, nil
}

// PrepareProjectCreation builds every requested project and the one catalog
// that adds them all. It writes nothing but the workspace data directory, and
// refuses a name another project holds or two requests that would share a
// slug.
func PrepareProjectCreation(state *config.RuntimeState, requests []NewProject, now time.Time) (ProjectCreation, error) {
	if len(requests) == 0 {
		return ProjectCreation{}, invalidInput("project", "no projects to create")
	}
	catalog := CloneCatalog(state.Catalog())
	projects := make([]*models.Project, 0, len(requests))
	seen := make(map[string]bool, len(requests))
	for _, request := range requests {
		name := strings.TrimSpace(request.Name)
		slug, err := ProjectSlug(state, name, "")
		if err != nil {
			return ProjectCreation{}, err
		}
		if seen[slug] {
			return ProjectCreation{}, invalidInput("project name", fmt.Sprintf("project slug '%s' is requested twice", slug))
		}
		seen[slug] = true
		project, err := models.NewProjectAt(request.ID, name, state.ProjectPath(slug), now)
		if err != nil {
			return ProjectCreation{}, invalidInput("project", err.Error())
		}
		if upsertErr := catalog.UpsertProject(project); upsertErr != nil {
			return ProjectCreation{}, upsertErr
		}
		projects = append(projects, project)
	}
	if err := state.EnsureDataDir(); err != nil {
		return ProjectCreation{}, err
	}
	artifact, err := state.CheckedCatalogArtifact(catalog)
	if err != nil {
		return ProjectCreation{}, err
	}
	return ProjectCreation{Projects: projects, Catalog: artifact.StorageChange()}, nil
}

// EmptyTasksArtifact renders a created project's first tasks.md, required to
// be absent, for a commit that writes no task into it.
func EmptyTasksArtifact(project *models.Project) (storage.ArtifactChange, error) {
	document, err := storage.CanonicalProjectDocument(project, nil)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	content, err := document.WriteCurrent()
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	requireAbsent := ""
	return storage.ArtifactChange{
		Path:             storage.TasksFilePath(project.Path),
		Data:             []byte(content),
		Mode:             storage.FilePermissions,
		ExpectedRevision: &requireAbsent,
	}, nil
}
