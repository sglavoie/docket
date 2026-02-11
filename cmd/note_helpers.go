package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// NoteContext holds resolved note directory and optional project context.
type NoteContext struct {
	Dir     string          // Notes directory path
	Project *models.Project // nil for global notes
	Config  *config.Config  // loaded config (available for both global and project notes)
}

// resolveNoteContext returns the notes directory and optional project based on the --global flag.
func resolveNoteContext() (*NoteContext, error) {
	if noteGlobal {
		dir, cfg, err := globalNotesDir()
		if err != nil {
			return nil, err
		}
		return &NoteContext{Dir: dir, Config: cfg}, nil
	}

	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return nil, err
	}
	return &NoteContext{Dir: storage.NotesDir(project), Project: project, Config: cfg}, nil
}

// findNote locates a note by exact filename or title prefix match.
func findNote(project *models.Project, query string) (*models.Note, error) {
	return findNoteIn(storage.NotesDir(project), query)
}

// findNoteIn locates a note in a directory by exact filename or title prefix match.
func findNoteIn(dir, query string) (*models.Note, error) {
	// Try exact filename first
	if strings.HasSuffix(query, ".md") {
		note, err := storage.LoadNoteFrom(dir, query)
		if err == nil {
			return note, nil
		}
	}

	notes, err := storage.ListNotesFrom(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}

	lowerQuery := strings.ToLower(query)
	for _, note := range notes {
		if strings.HasPrefix(strings.ToLower(note.Title), lowerQuery) {
			return note, nil
		}
		nameWithoutExt := strings.TrimSuffix(note.Filename, ".md")
		if strings.Contains(strings.ToLower(nameWithoutExt), lowerQuery) {
			return note, nil
		}
	}

	return nil, fmt.Errorf("note not found: %s", query)
}
