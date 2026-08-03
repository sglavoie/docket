package app

import (
	"github.com/sglavoie/tt/internal/models"
)

// NoteCollectionResult is one location's outcome from a workspace-wide note
// query. Err is scoped to Location: other successfully loaded collections
// remain available to callers.
type NoteCollectionResult struct {
	Location NoteLocation
	Notes    []*models.Note
	Err      error
}

// ProjectTags returns a project's derived tag vocabulary from the snapshot.
func (s *Service) ProjectTags(projectID string) ([]string, error) {
	_, project, _, err := s.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	return project.AllTags(), nil
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

// NotesAtLocations loads all requested note collections against one
// revision-consistent workspace snapshot. Snapshot failures abort the complete
// query; location resolution and note-port failures are retained on their
// corresponding result so callers can still use every successful collection.
func (s *Service) NotesAtLocations(locations []NoteLocation) ([]NoteCollectionResult, error) {
	snap, err := s.repo.Snapshot()
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
