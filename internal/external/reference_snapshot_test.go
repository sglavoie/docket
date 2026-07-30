package external

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestFormatTaskReferenceMarkdownIncludesReferenceSections(t *testing.T) {
	now := time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC)

	task, err := models.NewGenericTask("task-1", 42, "Ship reference snapshots")
	if err != nil {
		t.Fatal(err)
	}
	task.Description = "Open a rich markdown reference."
	task.Attachments = []models.Attachment{{Filename: "design.md"}}
	task.Checklist = []models.ChecklistItem{
		{Text: "Format tasks", Done: true},
		{Text: "Format notes"},
	}
	task.Comments = []*models.Comment{{
		Text:      "Remember linked notes.",
		CreatedAt: now,
	}}
	task.Decisions = []*models.Decision{{
		Text:      "Use cached temp files.",
		Rationale: "Async editors need the file after launch.",
		CreatedAt: now,
	}}
	child, err := models.NewGenericTask("child-1", 43, "Wire TUI")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.AddChild(child); err != nil {
		t.Fatal(err)
	}

	note := &models.Note{
		Title:    "Reference behavior",
		Filename: "2026-06-10-reference-behavior.md",
	}

	got := FormatTaskReferenceMarkdownWithNotes(task, []*models.Note{note})

	assertContains(t, got, ReferenceSnapshotWarning)
	assertContains(t, got, "## Description")
	assertContains(t, got, "Open a rich markdown reference.")
	assertContains(t, got, "## Attachments")
	assertContains(t, got, "design.md")
	assertContains(t, got, "## Subtasks")
	assertContains(t, got, "Wire TUI")
	assertContains(t, got, "## Checklist")
	assertContains(t, got, "- [x] Format tasks")
	assertContains(t, got, "- [ ] Format notes")
	assertContains(t, got, "## Comments")
	assertContains(t, got, "Remember linked notes.")
	assertContains(t, got, "## Decisions")
	assertContains(t, got, "Use cached temp files.")
	assertContains(t, got, "Rationale: Async editors need the file after launch.")
	assertContains(t, got, "## Linked Notes")
	assertContains(t, got, "Reference behavior (2026-06-10-reference-behavior.md)")
}

func TestCreateReferenceSnapshotFileUsesCacheAndCleansStaleFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	origCacheDir := referenceSnapshotCacheDir
	origMaxAge := referenceSnapshotMaxAge
	origNow := referenceSnapshotNow
	referenceSnapshotCacheDir = func() (string, error) { return dir, nil }
	referenceSnapshotMaxAge = time.Hour
	referenceSnapshotNow = func() time.Time { return now }
	t.Cleanup(func() {
		referenceSnapshotCacheDir = origCacheDir
		referenceSnapshotMaxAge = origMaxAge
		referenceSnapshotNow = origNow
	})

	stalePath := filepath.Join(dir, "stale.md")
	if err := os.WriteFile(stalePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write stale snapshot: %v", err)
	}
	staleTime := now.Add(-2 * time.Hour)
	if err := os.Chtimes(stalePath, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes stale snapshot: %v", err)
	}

	freshPath := filepath.Join(dir, "fresh.md")
	if err := os.WriteFile(freshPath, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("write fresh snapshot: %v", err)
	}
	freshTime := now.Add(-30 * time.Minute)
	if err := os.Chtimes(freshPath, freshTime, freshTime); err != nil {
		t.Fatalf("chtimes fresh snapshot: %v", err)
	}

	path, err := CreateReferenceSnapshotFile("# Reference\n", "tt-task-reference-", ".md")
	if err != nil {
		t.Fatalf("CreateReferenceSnapshotFile returned error: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("snapshot dir = %q, want %q", filepath.Dir(path), dir)
	}
	if !strings.HasPrefix(filepath.Base(path), "tt-task-reference-") {
		t.Fatalf("snapshot basename %q should preserve prefix", filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(data) != "# Reference\n" {
		t.Fatalf("snapshot content = %q", string(data))
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale snapshot should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh snapshot should remain, stat err = %v", err)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}
