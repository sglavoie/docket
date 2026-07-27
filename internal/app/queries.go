package app

import (
	"github.com/sglavoie/tt/internal/models"
)

// ProjectSummary is the presentation-neutral view of a project's durable metadata,
// derived from the WorkspaceSnapshot. It carries the stable identity, the synced
// catalog fields, and the derived per-project tag vocabulary so an
// adapter can render a project list without reaching into the domain model.
type ProjectSummary struct {
	ID          string
	Slug        string
	Name        string
	Description string
	Context     string
	Archived    bool
	Pinned      bool
	Sandbox     bool
	Internal    bool
	TaskCount   int
	Tags        []string
	Revision    string
}

// ListProjects returns a summary of every project in the workspace snapshot, in
// snapshot order. It is a read-only query: it loads a snapshot and never commits.
func (s *Service) ListProjects() ([]ProjectSummary, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return nil, err
	}
	projects := snap.Projects()
	out := make([]ProjectSummary, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectSummary(p, snap.Revision(p.ID)))
	}
	return out, nil
}

// Project returns the summary for a single project resolved from the snapshot.
func (s *Service) Project(projectID string) (ProjectSummary, error) {
	_, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectSummary{}, err
	}
	return projectSummary(project, revision), nil
}

// ProjectTags returns a project's derived tag vocabulary from the snapshot.
func (s *Service) ProjectTags(projectID string) ([]string, error) {
	_, project, _, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	return copyStrings(project.Tags), nil
}

// Notes returns a project's notes through the workspace-aware note port.
func (s *Service) Notes(projectID string) ([]*models.Note, error) {
	return s.NotesAt(ProjectNoteLocation(projectID))
}

// NotesAt returns project or global notes for an explicit path-free location.
// Locations are resolved against one workspace snapshot before the port reads.
func (s *Service) NotesAt(location NoteLocation) ([]*models.Note, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return nil, err
	}
	project, err := resolveNoteLocation(snap, location)
	if err != nil {
		return nil, err
	}
	return s.notes.Notes(location, project)
}

// Blockers returns a project's blockers through the artifact reader.
func (s *Service) Blockers(projectID string) ([]*models.Blocker, error) {
	_, project, _, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	return s.blockers.Blockers(project)
}

// ActiveBlockerCount returns the number of unresolved blockers in a project.
func (s *Service) ActiveBlockerCount(projectID string) (int, error) {
	blockers, err := s.Blockers(projectID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, b := range blockers {
		if !b.IsResolved() {
			count++
		}
	}
	return count, nil
}

// projectSummary builds a ProjectSummary from a project and its revision, copying
// the derived slices so a caller cannot mutate the snapshot's backing arrays.
func projectSummary(p *models.Project, revision string) ProjectSummary {
	return ProjectSummary{
		ID:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Context:     p.Context,
		Archived:    p.Archived,
		Pinned:      p.Pinned,
		Sandbox:     p.Sandbox,
		Internal:    p.Internal,
		TaskCount:   len(p.Tasks),
		Tags:        copyStrings(p.Tags),
		Revision:    revision,
	}
}

// copyStrings returns a defensive copy of a string slice, or nil for an empty
// input, so query results never alias the snapshot's backing arrays.
func copyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
