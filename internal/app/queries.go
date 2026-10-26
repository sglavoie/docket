package app

import (
	"github.com/sglavoie/docket/internal/models"
)

// NoteCollectionResult is one location's outcome from a workspace-wide note
// query. Err is scoped to Location: other successfully loaded collections
// remain available to callers.
type NoteCollectionResult struct {
	Location NoteLocation
	Notes    []*models.Note
	Err      error
}

// Notes returns a project's notes through the workspace-aware note port.
func (s *Service) Notes(projectID string) ([]*models.Note, error) {
	return s.NotesAt(ProjectNoteLocation(projectID))
}

// NotesAt returns project or global notes for an explicit path-free location.
// Locations are resolved against one workspace snapshot before the port reads.
func (s *Service) NotesAt(location NoteLocation) ([]*models.Note, error) {
	snap, err := s.noteLocationSnapshot()
	if err != nil {
		return nil, err
	}
	project, err := resolveNoteLocation(snap, location)
	if err != nil {
		return nil, err
	}
	return s.notes.Notes(location, project)
}

// NotesAtLocations loads all requested note collections against one
// revision-consistent workspace snapshot. Snapshot failures abort the complete
// query; location resolution and note-port failures are retained on their
// corresponding result so callers can still use every successful collection.
func (s *Service) NotesAtLocations(locations []NoteLocation) ([]NoteCollectionResult, error) {
	snap, err := s.noteLocationSnapshot()
	if err != nil {
		return nil, err
	}
	results := make([]NoteCollectionResult, len(locations))
	for i, location := range locations {
		results[i].Location = location
		project, resolveErr := resolveNoteLocation(snap, location)
		if resolveErr != nil {
			results[i].Err = resolveErr
			continue
		}
		results[i].Notes, results[i].Err = s.notes.Notes(location, project)
	}
	return results, nil
}

// NoteLocationSource is an optional repository capability: the cataloged,
// unarchived projects a note location may name, read without their tasks.
// Reading notes resolves a project to its directory and nothing more, so a
// repository that can enumerate the catalog alone spares a note query the
// parse of every tasks.md a full Snapshot performs.
type NoteLocationSource interface {
	NoteLocationProjects() ([]*models.Project, error)
}

// noteLocationSnapshot is the read model a note query resolves locations
// against: the task-free catalog when the repository offers one, else a full
// snapshot. Both admit the same projects, so resolution answers alike.
func (s *Service) noteLocationSnapshot() (Snapshot, error) {
	source, ok := s.repo.(NoteLocationSource)
	if !ok {
		return s.repo.Snapshot()
	}
	projects, err := source.NoteLocationProjects()
	if err != nil {
		return nil, err
	}
	return NewPartialSnapshot(projects, nil, nil, nil), nil
}
