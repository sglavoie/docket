package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Regression: when a query matches one note title exactly and is also a prefix
// of other note titles, the exact match must win. Previously findNoteIn
// returned whichever note happened to be listed first, which made commands
// like `tt note delete meeting` ambiguous when both "meeting" and
// "meeting-followup" existed.
func TestFindNoteInExactMatchPreferred(t *testing.T) {
	dir := t.TempDir()

	makeNote := func(title string) *models.Note {
		n := models.NewNote(title)
		content, err := storage.RenderNoteContent(n)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, n.Filename), []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return n
	}

	// Order matters: write the longer one first so storage lists it earlier.
	makeNote("meeting-followup")
	exact := makeNote("meeting")

	got, err := findNoteIn(dir, "meeting")
	if err != nil {
		t.Fatalf("expected exact match to win, got error: %v", err)
	}
	if got == nil || got.Filename != exact.Filename {
		t.Fatalf("expected note %q, got %+v", exact.Filename, got)
	}
}

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
