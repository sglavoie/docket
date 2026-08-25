package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func saveExportNoteFixture(t *testing.T, dir string, note *models.Note) {
	t.Helper()
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		t.Fatal(err)
	}
	path, err := NoteFilePathIn(dir, note.Filename)
	if err != nil {
		t.Fatal(err)
	}
	content, err := RenderNoteContent(note)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), FilePermissions); err != nil {
		t.Fatal(err)
	}
}

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

func TestExportProjectMarkdown_EmptyDestRejected(t *testing.T) {
	tmp := t.TempDir()
	p := &models.Project{Name: "Test", Slug: "test", Path: tmp}

	_, err := ExportProjectMarkdown(p, "")
	if err == nil {
		t.Fatal("expected error for empty destination path, got nil")
	}
	if !strings.Contains(err.Error(), "destination") {
		t.Errorf("expected destination-required error, got: %v", err)
	}
}

func TestExportProjectMarkdown_NotesUseRetainedMinimalShape(t *testing.T) {
	tmp := t.TempDir()
	notesDir := filepath.Join(tmp, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newer := &models.Note{
		Title:     "Newer Normal",
		CreatedAt: base, ModifiedAt: base,
		Filename: models.GenerateNoteFilename("Newer Normal", base),
	}
	older := &models.Note{
		Title:     "Older Normal",
		CreatedAt: base.Add(-time.Hour), ModifiedAt: base.Add(-time.Hour),
		Filename: models.GenerateNoteFilename("Older Normal", base.Add(-time.Hour)),
	}
	oldest := &models.Note{
		Title:     "Oldest Normal",
		CreatedAt: base.Add(-2 * time.Hour), ModifiedAt: base.Add(-2 * time.Hour),
		Filename: models.GenerateNoteFilename("Oldest Normal", base.Add(-2*time.Hour)),
	}
	for _, note := range []*models.Note{newer, older, oldest} {
		saveExportNoteFixture(t, notesDir, note)
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

	newerPos := strings.Index(s, "## Newer Normal")
	olderPos := strings.Index(s, "## Older Normal")
	oldestPos := strings.Index(s, "## Oldest Normal")
	if oldestPos < 0 || newerPos < 0 || olderPos < 0 {
		t.Fatalf("not all notes found in output: %q", s)
	}
	if newerPos >= olderPos || olderPos >= oldestPos {
		t.Errorf("notes are not newest-first: newer=%d older=%d oldest=%d", newerPos, olderPos, oldestPos)
	}
	if strings.Contains(s, "tags:") {
		t.Fatalf("minimal note export restored retired tags metadata: %q", s)
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
		Title:     "My Note",
		CreatedAt: now, ModifiedAt: now,
		Filename: models.GenerateNoteFilename("My Note", now),
		Body:     "note body",
	}
	saveExportNoteFixture(t, notesDir, note)

	task := mustTask("t1", 1, "My Task")
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

func TestWriteProjectMarkdownCleanIncludesSchema6TaskRecordsAndAttachmentReferences(t *testing.T) {
	now := time.Date(2026, 7, 29, 15, 30, 0, 0, time.UTC)
	active := mustTask("active", 1, "Active task")
	active.Priority = 0
	active.DueDate = &now
	active.Checklist = []models.ChecklistItem{{Text: "First", Done: true}, {Text: "Second"}}
	active.Comments = []*models.Comment{{Text: "A retained comment", CreatedAt: now}}
	active.Attachments = []models.Attachment{
		{Filename: "spec.pdf", OriginalPath: "/source/spec.pdf", AddedAt: now},
		{Filename: "https://example.test/item", OriginalPath: "https://example.test/item", IsURL: true, AddedAt: now},
		{Filename: "internal/app/service.go", OriginalPath: "internal/app/service.go", IsCodeRef: true, LineNumber: 42, AddedAt: now},
	}

	terminal := mustTask("terminal", 2, "Terminal task")
	terminal.Priority = 0
	terminal.WorkflowState = models.StateDone
	terminal.CompletedAt = &now

	project := newProject("Export", active, terminal)
	got := WriteProjectMarkdownClean(project)
	for _, want := range []string{
		"## Task: (#1) Active task [due 2026-07-29 15:30]",
		"- [x] First",
		"- [ ] Second",
		"- A retained comment",
		"- File: spec.pdf",
		"- URL: https://example.test/item",
		"- Code: internal/app/service.go:42",
		"## Task: (#2) Terminal task [done]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("export missing %q:\n%s", want, got)
		}
	}
}
