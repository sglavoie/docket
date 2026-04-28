package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestExportProjectMarkdown_NoNotesDir(t *testing.T) {
	tmp := t.TempDir()
	p := &models.Project{Name: "Test", Slug: "test", Path: tmp}
	dest := filepath.Join(tmp, "out.md")

	path, err := ExportProjectMarkdown(p, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.Contains(string(got), "---") || strings.Contains(string(got), "# Notes") {
		t.Errorf("expected no notes section when notes dir absent, got: %q", string(got))
	}
}

func TestExportProjectMarkdown_EmptyNotesDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "notes"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := &models.Project{Name: "Test", Slug: "test", Path: tmp}
	dest := filepath.Join(tmp, "out.md")

	path, err := ExportProjectMarkdown(p, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if strings.Contains(string(got), "---") || strings.Contains(string(got), "# Notes") {
		t.Errorf("expected no notes section for empty dir, got: %q", string(got))
	}
}

func TestExportProjectMarkdown_PinnedOrdering(t *testing.T) {
	tmp := t.TempDir()
	notesDir := filepath.Join(tmp, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newer := &models.Note{
		Title: "Newer Normal", Tags: []string{},
		CreatedAt: base, ModifiedAt: base,
		Filename: models.GenerateNoteFilename("Newer Normal", base),
	}
	older := &models.Note{
		Title: "Older Normal", Tags: []string{},
		CreatedAt: base.Add(-time.Hour), ModifiedAt: base.Add(-time.Hour),
		Filename: models.GenerateNoteFilename("Older Normal", base.Add(-time.Hour)),
	}
	pinned := &models.Note{
		Title: "Pinned Note", Tags: []string{},
		Pinned:    true,
		CreatedAt: base.Add(-2 * time.Hour), ModifiedAt: base.Add(-2 * time.Hour),
		Filename: models.GenerateNoteFilename("Pinned Note", base.Add(-2*time.Hour)),
	}
	for _, note := range []*models.Note{newer, older, pinned} {
		if err := SaveNoteTo(notesDir, note); err != nil {
			t.Fatalf("save note: %v", err)
		}
	}

	p := &models.Project{Name: "Test", Slug: "test", Path: tmp}
	dest := filepath.Join(tmp, "out.md")
	path, err := ExportProjectMarkdown(p, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	s := string(got)

	pinnedPos := strings.Index(s, "## Pinned Note")
	newerPos := strings.Index(s, "## Newer Normal")
	olderPos := strings.Index(s, "## Older Normal")
	if pinnedPos < 0 || newerPos < 0 || olderPos < 0 {
		t.Fatalf("not all notes found in output: %q", s)
	}
	if pinnedPos > newerPos || pinnedPos > olderPos {
		t.Errorf("pinned note should appear first: pinned=%d newer=%d older=%d", pinnedPos, newerPos, olderPos)
	}
	if newerPos > olderPos {
		t.Errorf("newer note should appear before older: newer=%d older=%d", newerPos, olderPos)
	}
}

func TestExportProjectMarkdown_ParseErrorNote(t *testing.T) {
	tmp := t.TempDir()
	notesDir := filepath.Join(tmp, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	brokenContent := "this is not frontmatter\n\nbody content"
	if err := os.WriteFile(filepath.Join(notesDir, "broken.md"), []byte(brokenContent), 0o600); err != nil {
		t.Fatalf("write broken note: %v", err)
	}

	p := &models.Project{Name: "Test", Slug: "test", Path: tmp}
	dest := filepath.Join(tmp, "out.md")
	path, err := ExportProjectMarkdown(p, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, "[parse error]") {
		t.Errorf("expected [parse error] in output, got: %q", s)
	}
	if !strings.Contains(s, "## broken") {
		t.Errorf("expected ## broken heading (title from filename), got: %q", s)
	}
}

func TestExportProjectMarkdown_TasksAndNotesCombined(t *testing.T) {
	tmp := t.TempDir()
	notesDir := filepath.Join(tmp, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	now := time.Now()
	note := &models.Note{
		Title: "My Note", Tags: []string{},
		CreatedAt: now, ModifiedAt: now,
		Filename: models.GenerateNoteFilename("My Note", now),
		Body:     "note body",
	}
	if err := SaveNoteTo(notesDir, note); err != nil {
		t.Fatalf("save note: %v", err)
	}

	task := mustTask("t1", 1, models.TaskTypeTask, "My Task")
	task.Priority = 0
	p := newProject("Combined", task)
	p.Path = tmp
	if err := SaveProject(p); err != nil {
		t.Fatalf("save project: %v", err)
	}

	dest := filepath.Join(tmp, "out.md")
	path, err := ExportProjectMarkdown(p, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	s := string(got)

	taskPos := strings.Index(s, "## Task: (#1) My Task")
	sepPos := strings.Index(s, "---")
	notesPos := strings.Index(s, "# Notes")
	if taskPos < 0 || sepPos < 0 || notesPos < 0 {
		t.Fatalf("expected task, separator, and notes in output: %q", s)
	}
	if taskPos >= sepPos || sepPos >= notesPos {
		t.Errorf("expected task < --- < # Notes ordering: task=%d sep=%d notes=%d", taskPos, sepPos, notesPos)
	}
}