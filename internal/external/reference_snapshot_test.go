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

	task, err := models.NewTask("task-1", 42, models.TaskTypeStory, "Ship reference snapshots")
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
		Category:  models.DecisionCategoryArchitecture,
		CreatedAt: now,
	}}
	task.RelatedTo = []string{"related-task"}

	child, err := models.NewTask("child-1", 43, models.TaskTypeTask, "Wire TUI")
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
	assertContains(t, got, "## Related Tasks")
	assertContains(t, got, "related-task")
	assertContains(t, got, "## Linked Notes")
	assertContains(t, got, "Reference behavior (2026-06-10-reference-behavior.md)")
}

func TestFormatNoteReferenceMarkdownIncludesLinkedTaskSummaryAndWarning(t *testing.T) {
	created := time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC)
	modified := time.Date(2026, 6, 10, 10, 15, 0, 0, time.UTC)
	note := &models.Note{
		Title:       "Launch notes",
		Tags:        []string{"release", "reference"},
		CreatedAt:   created,
		ModifiedAt:  modified,
		LinkedTasks: []string{"task-1", "missing-task"},
		Filename:    "2026-06-10-launch-notes.md",
		Body:        "## Context\n\nUse Ctrl+O for references.\n",
	}
	task, err := models.NewTask("task-1", 7, models.TaskTypeStory, "Open references")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = models.StatusInProgress

	got := FormatNoteReferenceMarkdownWithTasks(note, []*models.Task{task})

	assertContains(t, got, "# Note: Launch notes")
	assertContains(t, got, ReferenceSnapshotWarning)
	assertContains(t, got, "| Filename | 2026-06-10-launch-notes.md |")
	assertContains(t, got, "| Tags | release, reference |")
	assertContains(t, got, "| Created | 2026-06-09 08:00 |")
	assertContains(t, got, "| Modified | 2026-06-10 10:15 |")
	assertContains(t, got, "## Linked Tasks")
	assertContains(t, got, "#7 Story: Open references (in_progress) [task-1]")
	assertContains(t, got, "missing-task")
	assertContains(t, got, "## Body")
	assertContains(t, got, "Use Ctrl+O for references.")
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
