package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// The project lifecycle is the one place a project is created, renamed,
// retagged, parked, archived, restored, or deleted, so the desktop's project
// hub and the phone commit the same artifacts in the same transaction. Each
// function prepares on an isolated copy, commits through the application
// service's checked transaction and undo ledger, and only then refreshes the
// runtime state; a refusal writes nothing.

// ProjectChange is one committed project mutation: the project as it now
// stands and the transaction the workspace ledger recorded.
type ProjectChange struct {
	Project       *models.Project
	TransactionID string
}

// ProjectImpact is what a delete would take with the project, read from its
// stored files rather than from any presentation, so a confirmation never
// understates it.
type ProjectImpact struct {
	Tasks       int
	Notes       int
	Attachments int
}

// ClonePlanning copies planning state so a membership edit on the copy leaves
// the original untouched.
func ClonePlanning(planning *config.PlanningState) *config.PlanningState {
	cp := *planning
	cp.Next = append([]config.ProjectTaskLocator(nil), planning.GetNext()...)
	cp.Today = append([]config.ProjectTaskLocator(nil), planning.Today...)
	return &cp
}

func projectInput(field, reason string) error {
	return &app.InvalidInputError{Field: field, Reason: reason}
}

// catalogArtifacts renders the catalog alone, stamped with the revision the
// runtime state observed.
func catalogArtifacts(state *config.RuntimeState, catalog *config.WorkspaceCatalog) ([]storage.ArtifactChange, error) {
	artifact, err := state.CheckedCatalogArtifact(catalog)
	if err != nil {
		return nil, err
	}
	return []storage.ArtifactChange{artifact.StorageChange()}, nil
}

// domainArtifacts renders the catalog and planning together.
func domainArtifacts(state *config.RuntimeState, catalog *config.WorkspaceCatalog, planning *config.PlanningState) ([]storage.ArtifactChange, error) {
	domains, err := state.CheckedSyncedArtifacts(&config.Domains{Catalog: catalog, Planning: planning})
	if err != nil {
		return nil, err
	}
	changes := make([]storage.ArtifactChange, 0, len(domains))
	for _, artifact := range domains {
		changes = append(changes, artifact.StorageChange())
	}
	return changes, nil
}

// CreateProject creates an empty project and its catalog record as one
// transaction. id is the new project's identity.
func CreateProject(state *config.RuntimeState, service *app.Service, id, name string) (ProjectChange, error) {
	creation, err := app.PrepareProjectCreation(state, []app.NewProject{{ID: id, Name: name}}, service.Now())
	if err != nil {
		return ProjectChange{}, err
	}
	project := creation.Projects[0]
	tasks, err := app.EmptyTasksArtifact(project)
	if err != nil {
		return ProjectChange{}, err
	}
	transactionID, err := service.CommitProjectMutationResult(storage.OpsCommitRequest{
		Artifacts:   []storage.ArtifactChange{creation.Catalog, tasks},
		ProjectIDs:  []string{project.ID},
		UndoAction:  "Create Project",
		UndoContext: project.Name,
	})
	if err != nil {
		return ProjectChange{}, err
	}
	if err := state.RefreshAfterCommit(); err != nil {
		return ProjectChange{}, err
	}
	return ProjectChange{Project: project, TransactionID: transactionID}, nil
}

// ProjectEdit is one save of a project's own fields. The name, the
// description, and the complete tag set are all written; a caller keeping one
// passes its stored value.
type ProjectEdit struct {
	Name        string
	Description string
	Tags        []string
	Now         time.Time
}

// EditProject renames, describes, and retags a project as one transaction:
// the directory move a new slug needs, the tasks.md rewrite, and the catalog
// record land together or not at all. The system Unfiled project keeps its
// name and is never parked.
func EditProject(state *config.RuntimeState, service *app.Service, project *models.Project, edit ProjectEdit) (ProjectChange, error) {
	name := strings.TrimSpace(edit.Name)
	tags, err := models.NormalizeProjectTags(edit.Tags)
	if err != nil {
		return ProjectChange{}, projectInput("project tags", err.Error())
	}
	protected := state.IsSystemUnfiledProject(project.ID)
	if protected && name != project.Name {
		return ProjectChange{}, fmt.Errorf("%w: the system Unfiled name and slug are immutable", app.ErrProtectedSystemUnfiled)
	}
	if protected && models.ProjectTagsParked(tags) {
		return ProjectChange{}, fmt.Errorf("%w: cannot park the system Unfiled project", app.ErrProtectedSystemUnfiled)
	}
	newSlug := project.Slug
	if name != project.Name {
		if newSlug, err = app.ProjectSlug(state, name, project.ID); err != nil {
			return ProjectChange{}, err
		}
	}
	oldSlug := project.Slug
	updated := *project
	updated.Name = name
	updated.Slug = newSlug
	updated.Description = strings.TrimSpace(edit.Description)
	updated.Tags = tags
	updated.UpdatedAt = edit.Now

	var tasksMarkdown []byte
	var tasksExpectedRevision *string
	// The rewrite renders the whole freshly loaded project, not its name and
	// task list: tasks.md also holds the project's comments, and a rename that
	// re-rendered only the tasks would drop them.
	if expected, loadErr := storage.LoadProjectForRewrite(&updated); loadErr == nil {
		content, renderErr := storage.WriteProjectMarkdown(&updated)
		if renderErr != nil {
			return ProjectChange{}, fmt.Errorf("render renamed project: %w", renderErr)
		}
		tasksMarkdown = []byte(content)
		tasksExpectedRevision = expected
	}
	catalog := app.CloneCatalog(state.Catalog())
	if upsertErr := catalog.UpsertProject(&updated); upsertErr != nil {
		return ProjectChange{}, upsertErr
	}
	artifacts, err := catalogArtifacts(state, catalog)
	if err != nil {
		return ProjectChange{}, err
	}
	transactionID, err := service.RenameProjectResult(app.RenameProjectRequest{
		Project:                &updated,
		SystemUnfiledProjectID: state.Catalog().SystemUnfiledProjectID,
		DataDir:                state.DataDir(),
		OldName:                project.Name,
		OldSlug:                oldSlug,
		NewSlug:                newSlug,
		TasksMarkdown:          tasksMarkdown,
		TasksExpectedRevision:  tasksExpectedRevision,
		Config:                 artifacts,
	})
	if err != nil {
		return ProjectChange{}, err
	}
	if err := state.RefreshAfterCommit(); err != nil {
		return ProjectChange{}, err
	}
	return ProjectChange{Project: &updated, TransactionID: transactionID}, nil
}

// ProjectTagUndoAction names a tag edit by its one behavioral consequence.
func ProjectTagUndoAction(project *models.Project, tags []string) string {
	parking := models.ProjectTagsParked(tags) && !models.ProjectTagsParked(project.Tags)
	unparking := !models.ProjectTagsParked(tags) && models.ProjectTagsParked(project.Tags)
	switch {
	case parking:
		return "Park Project"
	case unparking:
		return "Unpark Project"
	default:
		return "Set Project Tags"
	}
}

// SetProjectTags replaces a project's tags through the catalog transaction.
// Parking is one of them: it hides the project's work from the task workspace
// and keeps its Today and Next references, which are filtered at read time.
func SetProjectTags(state *config.RuntimeState, service *app.Service, project *models.Project, tags []string) (ProjectChange, error) {
	normalized, err := models.NormalizeProjectTags(tags)
	if err != nil {
		return ProjectChange{}, projectInput("project tags", err.Error())
	}
	if models.ProjectTagsParked(normalized) && state.IsSystemUnfiledProject(project.ID) {
		return ProjectChange{}, fmt.Errorf("%w: cannot park the system Unfiled project", app.ErrProtectedSystemUnfiled)
	}
	updated := *project
	updated.Tags = normalized
	return commitCatalogProject(state, service, &updated, ProjectTagUndoAction(project, normalized))
}

// commitCatalogProject writes one project's catalog record alone.
func commitCatalogProject(state *config.RuntimeState, service *app.Service, updated *models.Project, action string) (ProjectChange, error) {
	catalog := app.CloneCatalog(state.Catalog())
	if err := catalog.UpsertProject(updated); err != nil {
		return ProjectChange{}, err
	}
	artifacts, err := catalogArtifacts(state, catalog)
	if err != nil {
		return ProjectChange{}, err
	}
	transactionID, err := service.CommitProjectArtifactsResult(storage.ArtifactCommitRequest{
		Artifacts:   artifacts,
		ProjectIDs:  []string{updated.ID},
		UndoAction:  action,
		UndoContext: updated.Name,
	})
	if err != nil {
		return ProjectChange{}, err
	}
	if err := state.RefreshAfterCommit(); err != nil {
		return ProjectChange{}, err
	}
	return ProjectChange{Project: updated, TransactionID: transactionID}, nil
}

// loadedProject returns a copy of project holding its stored tasks.
func loadedProject(project *models.Project) (*models.Project, error) {
	loaded := *project
	if loaded.Tasks == nil {
		if err := storage.LoadProject(&loaded); err != nil {
			return nil, err
		}
	}
	return &loaded, nil
}

// ArchiveProject archives a project and drops its tasks from Today and Next in
// the same transaction; its tags are kept.
func ArchiveProject(state *config.RuntimeState, service *app.Service, project *models.Project) (ProjectChange, error) {
	if state.IsSystemUnfiledProject(project.ID) {
		return ProjectChange{}, fmt.Errorf("%w: cannot archive the system Unfiled project", app.ErrProtectedSystemUnfiled)
	}
	updated, err := loadedProject(project)
	if err != nil {
		return ProjectChange{}, err
	}
	updated.ArchiveAt(service.Now())
	archived := make([]config.ProjectTaskLocator, 0, len(updated.Tasks))
	for _, task := range updated.Tasks {
		if task != nil && task.ID != "" {
			archived = append(archived, config.ProjectTaskLocator{ProjectID: updated.ID, TaskID: task.ID})
		}
	}
	catalog := app.CloneCatalog(state.Catalog())
	planning := ClonePlanning(state.Planning())
	if upsertErr := catalog.UpsertProject(updated); upsertErr != nil {
		return ProjectChange{}, upsertErr
	}
	if len(archived) > 0 {
		planning.RemoveDurableTaskLocators(archived)
	}
	artifacts, err := domainArtifacts(state, catalog, planning)
	if err != nil {
		return ProjectChange{}, err
	}
	transactionID, err := service.CommitProjectArtifactsResult(storage.ArtifactCommitRequest{
		Artifacts:   artifacts,
		ProjectIDs:  []string{updated.ID},
		UndoAction:  "Archive Project",
		UndoContext: updated.Name,
	})
	if err != nil {
		return ProjectChange{}, err
	}
	if err := state.RefreshAfterCommit(); err != nil {
		return ProjectChange{}, err
	}
	return ProjectChange{Project: updated, TransactionID: transactionID}, nil
}

// UnarchiveProject restores an archived project's catalog record; the plan
// archiving dropped stays dropped.
func UnarchiveProject(state *config.RuntimeState, service *app.Service, project *models.Project) (ProjectChange, error) {
	updated := *project
	updated.UnarchiveAt(service.Now())
	return commitCatalogProject(state, service, &updated, "Restore Project")
}

// MeasureProjectImpact counts what a delete would remove with the project. It
// reads the project's own directory, so an archived project, which no task
// snapshot holds, is measured as honestly as an active one.
func MeasureProjectImpact(project *models.Project) (ProjectImpact, error) {
	loaded, err := loadedProject(project)
	if err != nil {
		return ProjectImpact{}, err
	}
	impact := ProjectImpact{Tasks: len(loaded.Tasks)}
	for _, task := range loaded.Tasks {
		if task != nil {
			impact.Attachments += len(task.Attachments)
		}
	}
	entries, err := os.ReadDir(filepath.Join(project.Path, "notes"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ProjectImpact{}, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			impact.Notes++
		}
	}
	return impact, nil
}

// DeleteProject removes a project's directory and every catalog and planning
// entry naming it as one transaction, which the workspace undo ledger restores
// whole. The system Unfiled project is never deleted.
func DeleteProject(state *config.RuntimeState, service *app.Service, project *models.Project) (ProjectChange, error) {
	if state.IsSystemUnfiledProject(project.ID) {
		return ProjectChange{}, fmt.Errorf("%w: cannot delete the system Unfiled project", app.ErrProtectedSystemUnfiled)
	}
	updated, err := loadedProject(project)
	if err != nil {
		return ProjectChange{}, err
	}
	domains := &config.Domains{Catalog: app.CloneCatalog(state.Catalog()), Planning: ClonePlanning(state.Planning())}
	if _, removeErr := domains.RemoveProject(updated.ID, updated.Slug); removeErr != nil {
		return ProjectChange{}, removeErr
	}
	artifacts, err := domainArtifacts(state, domains.Catalog, domains.Planning)
	if err != nil {
		return ProjectChange{}, err
	}
	transactionID, err := service.DeleteProjectResult(app.DeleteProjectRequest{
		Project:                updated,
		SystemUnfiledProjectID: state.Catalog().SystemUnfiledProjectID,
		DataDir:                state.DataDir(),
		Config:                 artifacts,
	})
	if err != nil {
		return ProjectChange{}, err
	}
	if err := state.RefreshAfterCommit(); err != nil {
		return ProjectChange{}, err
	}
	return ProjectChange{Project: updated, TransactionID: transactionID}, nil
}

// project resolves a cataloged project, archived ones included, from a fresh
// read of the workspace.
func (r *Runtime) project(id string) (*models.Project, error) {
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	project := r.state.GetProject(id)
	if project == nil || project.ID != id {
		return nil, fmt.Errorf("%w: project %s", app.ErrProjectNotFound, id)
	}
	return project, nil
}

// CreateProject creates an empty project under a fresh identity.
func (r *Runtime) CreateProject(name string) (ProjectChange, error) {
	if err := r.Refresh(); err != nil {
		return ProjectChange{}, err
	}
	return CreateProject(r.state, r.service, r.ids.NewID(), name)
}

// EditProject saves a project's name, description, and tags.
func (r *Runtime) EditProject(id string, edit ProjectEdit) (ProjectChange, error) {
	project, err := r.project(id)
	if err != nil {
		return ProjectChange{}, err
	}
	if edit.Now.IsZero() {
		edit.Now = r.clock.Now()
	}
	return EditProject(r.state, r.service, project, edit)
}

// SetProjectTags replaces a project's tags, parking included.
func (r *Runtime) SetProjectTags(id string, tags []string) (ProjectChange, error) {
	project, err := r.project(id)
	if err != nil {
		return ProjectChange{}, err
	}
	return SetProjectTags(r.state, r.service, project, tags)
}

// ArchiveProject archives a project.
func (r *Runtime) ArchiveProject(id string) (ProjectChange, error) {
	project, err := r.project(id)
	if err != nil {
		return ProjectChange{}, err
	}
	if project.Archived {
		return ProjectChange{}, projectInput("project", fmt.Sprintf("%s is already archived", project.Name))
	}
	return ArchiveProject(r.state, r.service, project)
}

// UnarchiveProject restores an archived project.
func (r *Runtime) UnarchiveProject(id string) (ProjectChange, error) {
	project, err := r.project(id)
	if err != nil {
		return ProjectChange{}, err
	}
	if !project.Archived {
		return ProjectChange{}, projectInput("project", fmt.Sprintf("%s is not archived", project.Name))
	}
	return UnarchiveProject(r.state, r.service, project)
}

// ProjectImpact measures what deleting a project would remove.
func (r *Runtime) ProjectImpact(id string) (*models.Project, ProjectImpact, error) {
	project, err := r.project(id)
	if err != nil {
		return nil, ProjectImpact{}, err
	}
	impact, err := MeasureProjectImpact(project)
	return project, impact, err
}

// DeleteProject deletes a project and everything it owns.
func (r *Runtime) DeleteProject(id string) (ProjectChange, error) {
	project, err := r.project(id)
	if err != nil {
		return ProjectChange{}, err
	}
	return DeleteProject(r.state, r.service, project)
}
