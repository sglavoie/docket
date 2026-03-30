package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func newTestProject(t *testing.T) *models.Project {
	t.Helper()
	dir := t.TempDir()
	return &models.Project{
		ID:   "test-project",
		Name: "Test",
		Path: dir,
	}
}

func TestNewNoteAndSaveLoad(t *testing.T) {
	project := newTestProject(t)
	note := models.NewNote("My Test Note")
	note.Body = "Hello, world!"
	note.Tags = []string{"test", "example"}

	if err := SaveNote(project, note); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}

	loaded, err := LoadNote(project, note.Filename)
	if err != nil {
		t.Fatalf("LoadNote failed: %v", err)
	}

	if loaded.Title != "My Test Note" {
		t.Errorf("Title = %q, want %q", loaded.Title, "My Test Note")
	}
	if strings.TrimSpace(loaded.Body) != "Hello, world!" {
		t.Errorf("Body = %q, want %q", loaded.Body, "Hello, world!")
	}
	if len(loaded.Tags) != 2 || loaded.Tags[0] != "test" {
		t.Errorf("Tags = %v, want [test example]", loaded.Tags)
	}
	if loaded.Filename != note.Filename {
		t.Errorf("Filename = %q, want %q", loaded.Filename, note.Filename)
	}
}

func TestListNotes(t *testing.T) {
	project := newTestProject(t)

	n1 := models.NewNote("First")
	n1.ModifiedAt = time.Now().Add(-2 * time.Hour)
	n2 := models.NewNote("Second")
	n2.ModifiedAt = time.Now().Add(-1 * time.Hour)
	n3 := models.NewNote("Pinned")
	n3.Pinned = true
	n3.ModifiedAt = time.Now().Add(-3 * time.Hour)

	for _, n := range []*models.Note{n1, n2, n3} {
		if err := SaveNote(project, n); err != nil {
			t.Fatalf("SaveNote failed: %v", err)
		}
	}

	notes, err := ListNotes(project)
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}
	if len(notes) != 3 {
		t.Fatalf("len = %d, want 3", len(notes))
	}
	// Pinned first
	if notes[0].Title != "Pinned" {
		t.Errorf("first note should be pinned, got %q", notes[0].Title)
	}
	// Then by modified date desc
	if notes[1].Title != "Second" {
		t.Errorf("second note should be Second, got %q", notes[1].Title)
	}
}

func TestDeleteNote(t *testing.T) {
	project := newTestProject(t)
	note := models.NewNote("To Delete")
	if err := SaveNote(project, note); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}

	if err := DeleteNote(project, note.Filename); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	path := filepath.Join(NotesDir(project), note.Filename)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("note file should not exist after deletion")
	}
}

func TestNoteLinking(t *testing.T) {
	note := models.NewNote("Linked Note")

	note.AddLink("task-uuid-1")
	if !note.HasLink("task-uuid-1") {
		t.Error("HasLink should return true after AddLink")
	}

	// Duplicate add is idempotent
	note.AddLink("task-uuid-1")
	if len(note.LinkedTasks) != 1 {
		t.Errorf("LinkedTasks should have 1 entry, got %d", len(note.LinkedTasks))
	}

	note.AddLink("task-uuid-2")
	if len(note.LinkedTasks) != 2 {
		t.Errorf("LinkedTasks should have 2 entries, got %d", len(note.LinkedTasks))
	}

	note.RemoveLink("task-uuid-1")
	if note.HasLink("task-uuid-1") {
		t.Error("HasLink should return false after RemoveLink")
	}
	if len(note.LinkedTasks) != 1 {
		t.Errorf("LinkedTasks should have 1 entry after remove, got %d", len(note.LinkedTasks))
	}
}

func TestNotesExist(t *testing.T) {
	project := newTestProject(t)
	if NotesExist(project) {
		t.Error("NotesExist should return false for empty project")
	}

	note := models.NewNote("Test")
	if err := SaveNote(project, note); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}

	if !NotesExist(project) {
		t.Error("NotesExist should return true after saving a note")
	}
}

func TestListNotesEmptyDir(t *testing.T) {
	project := newTestProject(t)
	notes, err := ListNotes(project)
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}
	if notes != nil {
		t.Errorf("expected nil, got %v", notes)
	}
}

func TestSaveNoteFilenameCollision(t *testing.T) {
	project := newTestProject(t)

	// Create two notes with the same filename but different creation times
	note1 := models.NewNote("Collision Test")
	if err := SaveNote(project, note1); err != nil {
		t.Fatalf("SaveNote note1 failed: %v", err)
	}
	originalFilename := note1.Filename

	note2 := &models.Note{
		Title:       "Collision Test 2",
		Tags:        []string{},
		CreatedAt:   time.Now().Add(time.Second), // different creation time
		ModifiedAt:  time.Now().Add(time.Second),
		LinkedTasks: []string{},
		Filename:    originalFilename, // same filename as note1
		Body:        "different content",
	}

	if err := SaveNote(project, note2); err != nil {
		t.Fatalf("SaveNote note2 failed: %v", err)
	}

	// note2 should have gotten a deduplicated filename
	if note2.Filename == originalFilename {
		t.Error("note2 should have a different filename after collision detection")
	}
	if !strings.HasSuffix(note2.Filename, ".md") {
		t.Errorf("note2 filename should end with .md, got %q", note2.Filename)
	}

	// Both notes should exist on disk
	notes, err := ListNotes(project)
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}

	// Verify original note is unchanged
	loaded1, err := LoadNote(project, originalFilename)
	if err != nil {
		t.Fatalf("LoadNote original failed: %v", err)
	}
	if loaded1.Title != "Collision Test" {
		t.Errorf("original note title = %q, want %q", loaded1.Title, "Collision Test")
	}
}

func TestRenameNoteIn(t *testing.T) {
	dir := t.TempDir()

	// Create initial note
	note := models.NewNote("Original Title")
	if err := SaveNoteTo(dir, note); err != nil {
		t.Fatalf("SaveNoteTo failed: %v", err)
	}
	oldFilename := note.Filename

	// Rename to a new title
	renamed, err := RenameNoteIn(dir, oldFilename, "New Title")
	if err != nil {
		t.Fatalf("RenameNoteIn failed: %v", err)
	}
	if renamed.Title != "New Title" {
		t.Errorf("Title = %q, want %q", renamed.Title, "New Title")
	}

	// Old file should be gone
	oldPath := filepath.Join(dir, oldFilename)
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file should not exist after rename")
	}

	// New file should exist
	newPath := filepath.Join(dir, renamed.Filename)
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new file should exist: %v", err)
	}
}

func TestRenameNoteInDedup(t *testing.T) {
	dir := t.TempDir()

	// Create two notes that will collide on rename
	note1 := models.NewNote("Target Name")
	if err := SaveNoteTo(dir, note1); err != nil {
		t.Fatalf("SaveNoteTo note1 failed: %v", err)
	}

	note2 := models.NewNote("Something Else")
	if err := SaveNoteTo(dir, note2); err != nil {
		t.Fatalf("SaveNoteTo note2 failed: %v", err)
	}

	// Rename note2 to "Target Name" — should get a deduplicated filename
	renamed, err := RenameNoteIn(dir, note2.Filename, "Target Name")
	if err != nil {
		t.Fatalf("RenameNoteIn failed: %v", err)
	}
	if renamed.Filename == note1.Filename {
		t.Error("renamed note should have a different filename from existing note")
	}
	if !strings.HasSuffix(renamed.Filename, ".md") {
		t.Errorf("renamed filename should end with .md, got %q", renamed.Filename)
	}

	// Both notes should exist
	notes, err := ListNotesFrom(dir)
	if err != nil {
		t.Fatalf("ListNotesFrom failed: %v", err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestParseNoteRoundtrip(t *testing.T) {
	note := models.NewNote("Roundtrip Test")
	note.Tags = []string{"go", "test"}
	note.LinkedTasks = []string{"uuid-123"}
	note.Pinned = true
	note.Body = "Some body content.\n\nWith paragraphs.\n"

	content, err := renderNote(note)
	if err != nil {
		t.Fatalf("renderNote failed: %v", err)
	}
	if !strings.HasPrefix(content, "---\n") {
		t.Error("rendered note should start with ---")
	}

	parsed, err := parseNote("test.md", content)
	if err != nil {
		t.Fatalf("parseNote failed: %v", err)
	}

	if parsed.Title != note.Title {
		t.Errorf("Title = %q, want %q", parsed.Title, note.Title)
	}
	if len(parsed.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(parsed.Tags))
	}
	if !parsed.Pinned {
		t.Error("Pinned should be true")
	}
	if len(parsed.LinkedTasks) != 1 || parsed.LinkedTasks[0] != "uuid-123" {
		t.Errorf("LinkedTasks = %v, want [uuid-123]", parsed.LinkedTasks)
	}
}
