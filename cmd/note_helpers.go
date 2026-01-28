package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// findNote locates a note by exact filename or title prefix match.
func findNote(project *models.Project, query string) (*models.Note, error) {
	// Try exact filename first
	if strings.HasSuffix(query, ".md") {
		note, err := storage.LoadNote(project, query)
		if err == nil {
			return note, nil
		}
	}

	// Search by title prefix
	notes, err := storage.ListNotes(project)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}

	lowerQuery := strings.ToLower(query)
	for _, note := range notes {
		if strings.HasPrefix(strings.ToLower(note.Title), lowerQuery) {
			return note, nil
		}
		// Also match filename without extension
		nameWithoutExt := strings.TrimSuffix(note.Filename, ".md")
		if strings.Contains(strings.ToLower(nameWithoutExt), lowerQuery) {
			return note, nil
		}
	}

	return nil, fmt.Errorf("note not found: %s", query)
}
