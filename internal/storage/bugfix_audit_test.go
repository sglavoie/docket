package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRoundTrip_CommentTextWithLeadingBackslashHeading guards against double
// unescape of comment text whose payload begins with a literal backslash
// followed by a heading-shaped line. The parser strips one backslash via
// unescapeDescriptionLine while accumulating description content, and the
// comment-block extractor then strips another via unescapeBlockText — the
// writer only emitted one, so the user-visible text loses a backslash on
// every full-pipeline round-trip.
func TestRoundTrip_CommentTextWithLeadingBackslashHeading(t *testing.T) {
	original := "\\## Task: not a heading"
	task := &models.Task{
		ID:       "task-uuid-1",
		ShortID:  1,
		Type:     models.TaskTypeTask,
		Title:    "Carrier",
		Status:   models.StatusTodo,
		Priority: models.PriorityDefault,
		Comments: []*models.Comment{
			{
				Text:      original,
				CreatedAt: time.Date(2025, 3, 4, 10, 0, 0, 0, time.Local),
			},
		},
	}

	doc := WriteTasksMarkdown("Project", []*models.Task{task})
	_, parsed, err := ParseTasksMarkdown(doc)
	if err != nil {
		t.Fatalf("ParseTasksMarkdown: %v", err)
	}
	if len(parsed) != 1 || len(parsed[0].Comments) != 1 {
		t.Fatalf("expected one task with one comment, got %d task(s)", len(parsed))
	}

	if got := parsed[0].Comments[0].Text; got != original {
		t.Errorf("comment text not preserved on full-pipeline round-trip\nwant: %q\ngot:  %q", original, got)
	}
}

// TestSaveNote_PreservesUnreadableExistingFile guards against silent overwrite
// of an existing-but-unreadable note. The earlier code only dedup'd when the
// existing note could be loaded AND its CreatedAt differed; an unreadable file
// would skip dedup and overwrite the existing content. The fix now treats an
// unreadable existing file as a collision and writes to a deduplicated path.
func TestSaveNote_PreservesUnreadableExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("filesystem permission semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks; test requires unprivileged user")
	}

	dir := t.TempDir()

	// Pre-create an existing note file with content we want to preserve.
	existingFilename := "preserve-me.md"
	existingPath := filepath.Join(dir, existingFilename)
	existingContent := []byte("---\ntitle: existing\ncreated: 2024-01-01T00:00:00Z\nmodified: 2024-01-01T00:00:00Z\n---\nimportant body\n")
	if err := os.WriteFile(existingPath, existingContent, 0644); err != nil {
		t.Fatalf("write existing note: %v", err)
	}

	// Make it unreadable so LoadNoteFrom errors out (but Stat still succeeds
	// because the parent directory itself is searchable).
	if err := os.Chmod(existingPath, 0); err != nil {
		t.Fatalf("chmod existing: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(existingPath, 0644)
	})

	// Try to save a new note with the same filename. Before the fix, this would
	// have silently overwritten the existing content.
	newNote := &models.Note{
		Title:       "Brand New Note",
		Tags:        []string{},
		LinkedTasks: []string{},
		Filename:    existingFilename,
		Body:        "new content that should NOT replace the existing file",
	}
	if err := SaveNoteTo(dir, newNote); err != nil {
		t.Fatalf("SaveNoteTo: %v", err)
	}

	// The new note must have received a deduplicated filename rather than
	// the original.
	if newNote.Filename == existingFilename {
		t.Errorf("expected new note to receive a deduplicated filename, got %q", newNote.Filename)
	}

	// Restore read permission and verify the original content survived intact.
	if err := os.Chmod(existingPath, 0644); err != nil {
		t.Fatalf("restore chmod: %v", err)
	}
	got, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing after save: %v", err)
	}
	if string(got) != string(existingContent) {
		t.Errorf("existing note was overwritten\nwant: %q\ngot:  %q", existingContent, got)
	}
}

// TestPurgeFromTrash_RejectsEmptySlug guards against a catastrophic data-loss
// bug where PurgeFromTrash("") would join with the trash root and os.RemoveAll
// the entire trash directory, wiping every recoverable project. The cmd layer
// added a similar guard in 81a37d10; this is the defense-in-depth at the
// storage entry point so that non-CLI callers (TUI, future SDK consumers)
// cannot bypass the check.
func TestPurgeFromTrash_RejectsEmptySlug(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, ".tt", "trash")
	if err := os.MkdirAll(filepath.Join(trashDir, "real-project"), 0o755); err != nil {
		t.Fatalf("setup trash: %v", err)
	}
	if err := PurgeFromTrash("", dir); err == nil {
		t.Error("expected error for empty slug, got nil")
	}
	if _, err := os.Stat(filepath.Join(trashDir, "real-project")); err != nil {
		t.Errorf("real-project was wiped by empty-slug PurgeFromTrash: %v", err)
	}
}

// TestRestoreProjectFromTrash_RejectsEmptySlug guards against the same class
// of catastrophic bug for the restore path: with slug="", trashPath collapses
// to the trash root and projectPath collapses to projects/, so the rename
// silently relocated every trashed entry into projects/, scrambling them.
func TestRestoreProjectFromTrash_RejectsEmptySlug(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, ".tt", "trash")
	projectsDir := filepath.Join(dir, "projects")
	if err := os.MkdirAll(filepath.Join(trashDir, "real-project"), 0o755); err != nil {
		t.Fatalf("setup trash: %v", err)
	}
	if err := RestoreProjectFromTrash("", dir); err == nil {
		t.Error("expected error for empty slug, got nil")
	}
	if _, err := os.Stat(filepath.Join(trashDir, "real-project")); err != nil {
		t.Errorf("real-project was relocated by empty-slug RestoreProjectFromTrash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectsDir, "real-project")); err == nil {
		t.Error("trash contents were moved to projects/ via empty-slug restore")
	}
}

// TestMoveProjectToTrash_RejectsEmptySlug guards against a malformed project
// (Slug=="") being passed in: with the empty slug, trashPath would resolve to
// the trash root itself and os.Rename(project.Path, trashRoot) could relocate
// arbitrary directories on top of the trash tree (or vice versa) depending on
// pre-existing layout. The pre-fix code happened to fail in some configurations
// because the trash root already existed, but that incidental safety must not
// be relied on.
func TestMoveProjectToTrash_RejectsEmptySlug(t *testing.T) {
	dir := t.TempDir()
	projectsDir := filepath.Join(dir, "projects", "real-project")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("setup projects: %v", err)
	}
	proj := &models.Project{Slug: "", Path: projectsDir}
	if err := MoveProjectToTrash(proj, dir); err == nil {
		t.Error("expected error for empty slug, got nil")
	}
	if _, err := os.Stat(projectsDir); err != nil {
		t.Errorf("project dir disappeared after empty-slug MoveProjectToTrash: %v", err)
	}
}

// TestRenameProjectDir_RejectsEmptySlug guards against an empty oldSlug or
// newSlug resolving to the projects/ root and renaming the entire projects/
// tree on top of (or out from under) a single project entry.
func TestRenameProjectDir_RejectsEmptySlug(t *testing.T) {
	dir := t.TempDir()
	projectsRoot := filepath.Join(dir, "projects")
	realPath := filepath.Join(projectsRoot, "real-project")
	if err := os.MkdirAll(realPath, 0o755); err != nil {
		t.Fatalf("setup projects: %v", err)
	}

	proj := &models.Project{Path: realPath}

	if err := RenameProjectDir(proj, "", "real-project", dir); err == nil {
		t.Error("expected error for empty oldSlug, got nil")
	}
	if _, err := os.Stat(realPath); err != nil {
		t.Errorf("real-project disappeared after empty-oldSlug RenameProjectDir: %v", err)
	}

	if err := RenameProjectDir(proj, "real-project", "", dir); err == nil {
		t.Error("expected error for empty newSlug, got nil")
	}
	if _, err := os.Stat(realPath); err != nil {
		t.Errorf("real-project disappeared after empty-newSlug RenameProjectDir: %v", err)
	}
}
