package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestFindNoteInRejectsEmptyQuery ensures findNoteIn does not silently match the
// first note when called with an empty query string. strings.HasPrefix(x, "")
// and strings.Contains(x, "") both return true, so without an explicit guard a
// command like `tt note delete ""` would target the first note found.
func TestFindNoteInRejectsEmptyQuery(t *testing.T) {
	dir := t.TempDir()

	// Create at least one note so the directory is non-empty
	note := models.NewNote("Important Note")
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, note.Filename), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := findNoteIn(dir, "")
	if err == nil {
		t.Fatalf("expected error for empty query, got note %+v", got)
	}
	if got != nil {
		t.Errorf("expected nil note for empty query, got %+v", got)
	}
}
