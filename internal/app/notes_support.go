package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func resolveNoteLocation(snap Snapshot, location NoteLocation) (*models.Project, error) {
	switch location.Scope {
	case NoteScopeGlobal:
		if location.ProjectID != "" {
			return nil, invalidInput("note location", "global scope cannot carry a project id")
		}
		return nil, nil
	case NoteScopeProject:
		if strings.TrimSpace(location.ProjectID) == "" {
			return nil, invalidInput("note location", "project id is required")
		}
		project := snap.ProjectByID(location.ProjectID)
		if project == nil {
			return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, location.ProjectID)
		}
		return project, nil
	default:
		return nil, invalidInput("note location", "scope must be project or global")
	}
}

func requireObservedNote(note *models.Note) error {
	if strings.TrimSpace(note.Filename) == "" {
		return invalidInput("note", "filename is required")
	}
	if strings.TrimSpace(note.RawRevision) == "" {
		return invalidInput("note revision", "an observed source revision is required")
	}
	return nil
}

func cloneNote(note *models.Note) *models.Note {
	if note == nil {
		return nil
	}
	copyNote := *note
	return &copyNote
}

func collisionFilename(base string, index int) string {
	if index <= 1 {
		return base
	}
	extension := filepath.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	return fmt.Sprintf("%s-%d%s", stem, index, extension)
}

func bumpCollision(operation *preparedNoteOperation) bool {
	if operation.collisionIndex >= maxNoteCollisionAttempts {
		return false
	}
	operation.collisionIndex++
	return true
}

func advanceCollision(operations []preparedNoteOperation, artifacts []preparedNoteArtifact, conflictPath string) bool {
	conflictPath = filepath.Clean(conflictPath)
	for i := range artifacts {
		artifact := artifacts[i]
		if artifact.destination && filepath.Clean(artifact.change.Path) == conflictPath {
			return bumpCollision(&operations[artifact.operation])
		}
	}
	return false
}

func artifactChanges(artifacts []preparedNoteArtifact) []storage.ArtifactChange {
	changes := make([]storage.ArtifactChange, len(artifacts))
	for i := range artifacts {
		changes[i] = artifacts[i].change
	}
	return changes
}

func noteProjectIDs(operations []preparedNoteOperation) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, operation := range operations {
		for _, location := range []NoteLocation{operation.source, operation.destination} {
			if location.Scope == NoteScopeProject && !seen[location.ProjectID] {
				seen[location.ProjectID] = true
				ids = append(ids, location.ProjectID)
			}
		}
	}
	return ids
}

func noteProjects(snap Snapshot, projectIDs []string) []*models.Project {
	projects := make([]*models.Project, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		if project := snap.ProjectByID(projectID); project != nil {
			projects = append(projects, project)
		}
	}
	return projects
}

func noteUndoMetadata(req NoteBatchRequest, results []NoteResult) (string, string) {
	action := strings.TrimSpace(req.UndoAction)
	if action == "" {
		if len(results) == 1 {
			action = strings.ToUpper(string(results[0].Kind[:1])) + string(results[0].Kind[1:]) + " Note"
		} else {
			action = "Update Notes"
		}
	}
	context := strings.TrimSpace(req.UndoContext)
	if context == "" {
		switch len(results) {
		case 1:
			context = results[0].Title
		default:
			context = fmt.Sprintf("%d notes", len(results))
		}
	}
	return action, context
}

func stringPointer(value string) *string { return &value }
