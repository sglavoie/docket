package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func mustTask(id string, shortID int, taskType models.TaskType, title string) *models.Task {
	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		panic(err)
	}
	return task
}

func newProject(name string, tasks ...*models.Task) *models.Project {
	p, err := models.NewProject("proj-id", name, "")
	if err != nil {
		panic(err)
	}
	p.Slug = "test-project"
	p.Tasks = tasks
	return p
}

func TestWriteProjectMarkdownClean_EmptyProject(t *testing.T) {
	p := newProject("Empty")
	out := WriteProjectMarkdownClean(p)

	if !strings.HasPrefix(out, "# Project: Empty\n") {
		t.Errorf("expected project header, got: %q", out)
	}
	// No tasks — should only have the header line
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line for empty project, got %d: %q", len(lines), out)
	}
}

func TestWriteProjectMarkdownClean_SingleTask(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Do the thing")
	task.Priority = 0 // override default PriorityDefault=3 so no badge appears
	p := newProject("My Project", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Task: (#1) Do the thing") {
		t.Errorf("expected task heading, got: %q", out)
	}
	// todo status + zero priority → no badge
	if strings.Contains(out, "[") {
		t.Errorf("expected no badge for todo/zero-priority task, got: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NestedHierarchy(t *testing.T) {
	epic := mustTask("e1", 1, models.TaskTypeEpic, "Big Epic")
	story := mustTask("s1", 2, models.TaskTypeStory, "A Story")
	story.ParentID = &epic.ID
	task := mustTask("t1", 3, models.TaskTypeTask, "Leaf Task")
	task.ParentID = &story.ID

	p := newProject("Nested", epic, story, task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Epic: (#1) Big Epic") {
		t.Errorf("missing epic heading: %q", out)
	}
	if !strings.Contains(out, "### Story: (#2) A Story") {
		t.Errorf("missing story heading: %q", out)
	}
	if !strings.Contains(out, "#### Task: (#3) Leaf Task") {
		t.Errorf("missing task heading: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Orphan(t *testing.T) {
	parentID := "missing-parent"
	orphan := mustTask("o1", 5, models.TaskTypeTask, "Orphan Task")
	orphan.ParentID = &parentID

	p := newProject("WithOrphan", orphan)
	out := WriteProjectMarkdownClean(p)

	// Orphan should still appear at root level (## heading)
	if !strings.Contains(out, "## Task: (#5) Orphan Task") {
		t.Errorf("expected orphan to appear at root level: %q", out)
	}
}

func TestWriteProjectMarkdownClean_DeepNestingClamped(t *testing.T) {
	// Build a chain 8 levels deep — headings must clamp at ######
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	tasks := make([]*models.Task, len(ids))
	for i, id := range ids {
		tasks[i] = mustTask(id, i+1, models.TaskTypeTask, "Level "+id)
		if i > 0 {
			parentID := ids[i-1]
			tasks[i].ParentID = &parentID
		}
	}

	p := newProject("Deep", tasks...)
	out := WriteProjectMarkdownClean(p)

	// Levels 7 and 8 (indices 6, 7) would be ####### and ######## without clamping
	if strings.Contains(out, "#######") {
		t.Errorf("headings were not clamped at ######: %q", out)
	}
	// The clamped tasks still appear
	if !strings.Contains(out, "###### Task: (#7)") {
		t.Errorf("expected level-7+ task at ###### heading: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Tags(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Tagged")
	task.Tags = []string{"backend", "auth"}
	p := newProject("Tags", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "tags: backend, auth") {
		t.Errorf("expected tags line: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NoTagsWhenEmpty(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "No Tags")
	task.Tags = []string{}
	p := newProject("NoTags", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "tags:") {
		t.Errorf("expected no tags line for empty tags: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Priority(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "High Priority")
	task.Priority = 1
	p := newProject("Prio", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "!1") {
		t.Errorf("expected priority in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_ZeroPriorityNoBadge(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Zero Prio")
	task.Priority = 0
	p := newProject("ZeroPrio", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "!0") {
		t.Errorf("zero priority should not appear in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_DueDate(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Has Due")
	due := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	task.DueDate = &due
	task.Priority = 0
	p := newProject("Due", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "due 2026-04-20") {
		t.Errorf("expected due date in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_SnoozedUntil(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Snoozed")
	snooze := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	task.SnoozedUntil = &snooze
	task.Priority = 0
	p := newProject("Snooze", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "snoozed until 2026-05-01") {
		t.Errorf("expected snooze date in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NonTodoStatus(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "In Progress")
	task.Status = models.StatusInProgress
	task.Priority = 0
	p := newProject("Status", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "in_progress") {
		t.Errorf("expected non-todo status in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_TodoStatusNoBadge(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Todo")
	task.Status = models.StatusTodo
	task.Priority = 0
	p := newProject("TodoStatus", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "[") {
		t.Errorf("todo status should not produce a badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_EmptyDescription(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "No Desc")
	task.Description = ""
	p := newProject("NoDesc", task)
	out := WriteProjectMarkdownClean(p)

	// No blank line before children / end when description is empty
	lines := strings.Split(out, "\n")
	taskHeadingFound := false
	for i, line := range lines {
		if strings.Contains(line, "## Task: (#1) No Desc") {
			taskHeadingFound = true
			// The line immediately after the heading should not be a description line
			if i+1 < len(lines) && lines[i+1] == "" {
				// This blank line is acceptable only if there's no description content after it
				if i+2 < len(lines) && lines[i+2] != "" && !strings.HasPrefix(lines[i+2], "#") {
					t.Errorf("unexpected description content after empty description task: %v", lines[i+2])
				}
			}
			break
		}
	}
	if !taskHeadingFound {
		t.Errorf("task heading not found in output: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NoHTMLComments(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Clean")
	task.Status = models.StatusInProgress
	task.Priority = 2
	task.Tags = []string{"x"}
	p := newProject("Clean", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "<!--") || strings.Contains(out, "-->") {
		t.Errorf("clean export must not contain HTML comments: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Description(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "With Desc")
	task.Description = "This is the description.\nWith multiple lines."
	task.Priority = 0
	p := newProject("Desc", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "This is the description.\nWith multiple lines.") {
		t.Errorf("expected verbatim description in output: %q", out)
	}
}

// ---- Group A: writeNotesSectionClean (pure helper) ----

func TestWriteNotesSectionClean(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		notes       []*models.Note
		wantExact   string // when non-empty, output must equal this exactly
		contains    string // substring that must be present
		notContains string // substring that must be absent
	}{
		{
			name:      "empty slice returns empty string",
			notes:     []*models.Note{},
			wantExact: "",
		},
		{
			name: "single note with tags and body",
			notes: []*models.Note{
				{Title: "Project ideas", Tags: []string{"brainstorm", "roadmap"}, Body: "body text", ModifiedAt: now},
			},
			contains: "\n---\n\n# Notes\n\n## Project ideas\ntags: brainstorm, roadmap\n\nbody text\n",
		},
		{
			name: "single note no tags",
			notes: []*models.Note{
				{Title: "No Tags", Tags: []string{}, Body: "content", ModifiedAt: now},
			},
			contains:    "## No Tags",
			notContains: "tags:",
		},
		{
			name: "single note empty body",
			notes: []*models.Note{
				{Title: "Empty Body", Tags: []string{}, Body: "", ModifiedAt: now},
			},
			contains: "## Empty Body\n",
		},
		{
			name: "parse error note heading suffix",
			notes: []*models.Note{
				{Title: "Broken", ParseError: "invalid frontmatter: missing delimiter", Body: "recovered", ModifiedAt: now},
			},
			contains: "## Broken [parse error]\n",
		},
		{
			name: "multiple notes ordering preserved with blank line between",
			notes: []*models.Note{
				{Title: "First", Tags: []string{}, Body: "body one", ModifiedAt: now},
				{Title: "Second", Tags: []string{}, Body: "body two", ModifiedAt: now.Add(-time.Hour)},
			},
			contains: "## First",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := writeNotesSectionClean(tc.notes)

			if tc.wantExact != "" || len(tc.notes) == 0 {
				if got != tc.wantExact {
					t.Errorf("expected exact %q, got %q", tc.wantExact, got)
				}
				return
			}
			if tc.contains != "" && !strings.Contains(got, tc.contains) {
				t.Errorf("expected output to contain %q, got: %q", tc.contains, got)
			}
			if tc.notContains != "" && strings.Contains(got, tc.notContains) {
				t.Errorf("expected output NOT to contain %q, got: %q", tc.notContains, got)
			}
		})
	}
}

func TestWriteNotesSectionClean_MultipleNotesOrdering(t *testing.T) {
	now := time.Now()
	notes := []*models.Note{
		{Title: "Alpha", Tags: []string{}, Body: "a", ModifiedAt: now},
		{Title: "Beta", Tags: []string{}, Body: "b", ModifiedAt: now.Add(-time.Hour)},
	}
	got := writeNotesSectionClean(notes)

	alphaPos := strings.Index(got, "## Alpha")
	betaPos := strings.Index(got, "## Beta")
	if alphaPos < 0 || betaPos < 0 {
		t.Fatalf("expected both notes in output: %q", got)
	}
	if alphaPos > betaPos {
		t.Errorf("expected Alpha before Beta (caller ordering preserved): alpha=%d beta=%d", alphaPos, betaPos)
	}
	// blank line between notes: "\n## Beta" appears after Alpha's body
	if !strings.Contains(got, "a\n\n## Beta") {
		t.Errorf("expected blank line between notes, got: %q", got)
	}
}

// ---- Group B: ExportProjectMarkdown (end-to-end) ----

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
	for _, n := range []*models.Note{newer, older, pinned} {
		if err := SaveNoteTo(notesDir, n); err != nil {
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
	// Write raw .md without frontmatter delimiters to trigger parse error.
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
