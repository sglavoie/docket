package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func resetNoteEditVars() {
	noteEditEditor = ""
}

func applyNoteEdit(dir string, original *models.Note, modified string) (*models.Note, error) {
	updated, err := parseNoteEdit(original, modified)
	if err != nil {
		return nil, err
	}
	if err := storage.SaveNoteTo(dir, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func TestRunNoteEdit(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetNoteEditVars()
		err := runNoteEdit(noteEditCmd, []string{})
		if err == nil {
			t.Error("expected error for missing note ID")
		}
	})
}

// Regression: when the user's edit corrupts or wipes the YAML frontmatter,
// storage.ParseNoteContent does not return a Go error — it returns a Note
// with ParseError set and Title derived from the filename. Previously
// runNoteEdit unconditionally saved that sanitized Note, silently
// overwriting the original file with filename-derived metadata and
// discarding whatever the user actually typed. applyNoteEdit must instead
// surface the parse failure so the user can fix their edit before the
// original note is clobbered.
func TestApplyNoteEditRejectsCorruptedFrontmatter(t *testing.T) {
	dir := t.TempDir()

	original := models.NewNote("Original Title")
	original.Body = "important content"
	if err := storage.SaveNoteTo(dir, original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	// Body without any frontmatter delimiters.
	corrupted := "this is the new body, no frontmatter at all"
	_, err := applyNoteEdit(dir, original, corrupted)
	if err == nil {
		t.Fatal("expected error for edit that wipes frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "invalid frontmatter") {
		t.Errorf("expected error mentioning 'invalid frontmatter', got: %v", err)
	}

	// Verify the original on-disk file was NOT modified.
	path := filepath.Join(dir, original.Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if !strings.Contains(string(data), "Original Title") {
		t.Errorf("expected original title preserved on disk, got:\n%s", data)
	}
	if !strings.Contains(string(data), "important content") {
		t.Errorf("expected original body preserved on disk, got:\n%s", data)
	}
}

// Regression: an edit that leaves valid frontmatter but with an empty title
// should be rejected. Without this guard, the saved note loses its
// human-readable title (yielding a Note whose Title field is the bare
// filename), which is almost certainly not what the user intended and is
// silently destructive.
func TestApplyNoteEditRejectsEmptyTitle(t *testing.T) {
	dir := t.TempDir()

	original := models.NewNote("Real Title")
	if err := storage.SaveNoteTo(dir, original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	emptyTitle := "---\ntitle: \"\"\ntags: []\ncreated: 2025-01-01T00:00:00Z\nmodified: 2025-01-01T00:00:00Z\n---\n\nbody\n"
	_, err := applyNoteEdit(dir, original, emptyTitle)
	if err == nil {
		t.Fatal("expected error for edit with empty title, got nil")
	}
	if !strings.Contains(err.Error(), "empty title") {
		t.Errorf("expected error mentioning 'empty title', got: %v", err)
	}
}

// Verify the happy path still works: a well-formed edit is persisted and
// returned without error.
func TestApplyNoteEditSucceedsOnValidEdit(t *testing.T) {
	dir := t.TempDir()

	original := models.NewNote("First")
	if err := storage.SaveNoteTo(dir, original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	valid := "---\ntitle: First Updated\ntags: [done]\ncreated: 2025-01-01T00:00:00Z\nmodified: 2025-01-01T00:00:00Z\n---\n\nupdated body\n"
	updated, err := applyNoteEdit(dir, original, valid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Title != "First Updated" {
		t.Errorf("expected title %q, got %q", "First Updated", updated.Title)
	}
}

func TestNoteEditorCancellationLeavesCanonicalNoteUnchanged(t *testing.T) {
	deps, project := setupNoteLinkTest(t)
	defer deps.Cleanup()
	noteGlobal = false

	note := models.NewNote("Cancel Edit")
	note.Body = "canonical body"
	writeNoteToProject(t, project, note)
	path := filepath.Join(storage.NotesDir(project), note.Filename)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := resolveNoteContext()
	if err != nil {
		t.Fatal(err)
	}
	if notes, listErr := notesNoteContext(ctx); listErr != nil || len(notes) != 1 {
		t.Fatalf("typed note lookup before editor = %v, %v (written path %s, resolved project path %s, data dir %s)", notes, listErr, project.Path, ctx.Project.Path, ctx.Workspace.Runtime.DataDir())
	}

	originalEditor := editNoteInExternalEditor
	editNoteInExternalEditor = func(_ string, content string, _ string) (string, error) {
		return content, nil
	}
	t.Cleanup(func() { editNoteInExternalEditor = originalEditor })

	if runErr := runNoteEdit(noteEditCmd, []string{note.Filename}); runErr != nil {
		t.Fatalf("runNoteEdit: %v", runErr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("canceled editor changed canonical note:\n%s", after)
	}
}

func TestNoteEditorMalformedOutputLeavesCanonicalNoteUnchanged(t *testing.T) {
	deps, project := setupNoteLinkTest(t)
	defer deps.Cleanup()
	noteGlobal = false

	note := models.NewNote("Malformed Edit")
	note.Body = "canonical body"
	writeNoteToProject(t, project, note)
	path := filepath.Join(storage.NotesDir(project), note.Filename)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := resolveNoteContext()
	if err != nil {
		t.Fatal(err)
	}
	if notes, listErr := notesNoteContext(ctx); listErr != nil || len(notes) != 1 {
		t.Fatalf("typed note lookup before editor = %v, %v (written path %s, resolved project path %s, data dir %s)", notes, listErr, project.Path, ctx.Project.Path, ctx.Workspace.Runtime.DataDir())
	}

	originalEditor := editNoteInExternalEditor
	editNoteInExternalEditor = func(_ string, _ string, _ string) (string, error) {
		return "malformed output without frontmatter", nil
	}
	t.Cleanup(func() { editNoteInExternalEditor = originalEditor })

	err = runNoteEdit(noteEditCmd, []string{note.Filename})
	if err == nil || !strings.Contains(err.Error(), "invalid frontmatter") {
		t.Fatalf("runNoteEdit err = %v, want invalid frontmatter", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("malformed editor output changed canonical note:\n%s", after)
	}
}
