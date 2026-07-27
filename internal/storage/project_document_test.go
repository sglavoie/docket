package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// newDocTask builds a minimal task for document round-trip tests.
func newDocTask(t *testing.T, id string, shortID int, taskType models.TaskType, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	base := time.Date(2025, 3, 1, 8, 0, 0, 0, time.Local)
	task.CreatedAt = base
	task.UpdatedAt = base
	return task
}

// TestProjectDocumentV2FixedPoint verifies write→parse→write reaches a fixed
// point for a schema-v2 document, the primary losslessness contract.
func TestProjectDocumentV2FixedPoint(t *testing.T) {
	epic := newDocTask(t, "epic-1", 1, models.TaskTypeEpic, "Epic one")
	epic.ApplyWorkflowState(models.StateInProgress)
	child := newDocTask(t, "task-1", 2, models.TaskTypeTask, "Child task")
	child.ParentID = &epic.ID
	child.ApplyWorkflowState(models.StateWaiting)

	doc := &ProjectDocument{
		ProjectName:   "Schema V2",
		Tags:          []string{"backend"},
		Tasks:         []*models.Task{epic, child},
		SchemaVersion: SchemaVersionV2,
	}

	seed := doc.WriteV2()
	if !strings.Contains(seed, "<!-- tt-schema: 2 -->") {
		t.Fatalf("v2 output missing schema marker:\n%s", seed)
	}

	parsed, err := ParseProjectDocument(seed)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if parsed.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("schema version = %d, want %d", parsed.SchemaVersion, SchemaVersionV2)
	}
	if len(parsed.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", parsed.Warnings)
	}

	written := parsed.WriteV2()
	if written != seed {
		t.Fatalf("v2 write not a fixed point:\n--- seed ---\n%s\n--- written ---\n%s", seed, written)
	}

	// Revision is stable across the round-trip because canonical content matches.
	if parsed.Revision != doc.ComputeRevision() {
		t.Fatalf("revision drift: parsed=%s want=%s", parsed.Revision, doc.ComputeRevision())
	}
}

// TestProjectDocumentV2SameMinuteUpdatedAtFixedPoint covers timestamps whose
// in-memory seconds differ but whose persisted minute is identical. The writer
// must decide whether to emit updated using persisted precision; otherwise the
// first write contains a redundant updated field that disappears after parse and
// changes the canonical revision.
func TestProjectDocumentV2SameMinuteUpdatedAtFixedPoint(t *testing.T) {
	task := newDocTask(t, "same-minute", 1, models.TaskTypeTask, "Same minute")
	task.CreatedAt = time.Date(2026, 7, 13, 10, 15, 5, 0, time.Local)
	task.UpdatedAt = time.Date(2026, 7, 13, 10, 15, 55, 0, time.Local)
	doc := &ProjectDocument{
		ProjectName:   "Stable timestamps",
		Tasks:         []*models.Task{task},
		SchemaVersion: SchemaVersionV2,
	}

	first := doc.WriteV2()
	if strings.Contains(first, "updated:") {
		t.Fatalf("same-minute timestamp emitted a non-canonical updated field:\n%s", first)
	}
	parsed, err := ParseProjectDocument(first)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if second := parsed.WriteV2(); second != first {
		t.Fatalf("same-minute write not a fixed point:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if parsed.Revision != doc.ComputeRevision() {
		t.Fatalf("same-minute revision drift: parsed=%s initial=%s", parsed.Revision, doc.ComputeRevision())
	}
}

// TestProjectDocumentV1Upgrade verifies a legacy v1 document parses, upgrades to
// v2, and preserves the derived state and hierarchy.
func TestProjectDocumentV1Upgrade(t *testing.T) {
	legacy := "# Project: Legacy\n\n" +
		"## Task: Waiting item\n" +
		"<!-- id: leg-1, status: todo, section: Waiting, priority: 2, created: 2025-03-01 08:00:00 -->\n"

	doc, err := ParseProjectDocument(legacy)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if doc.SchemaVersion != SchemaVersionLegacy {
		t.Fatalf("schema version = %d, want %d (legacy)", doc.SchemaVersion, SchemaVersionLegacy)
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(doc.Tasks))
	}
	if got := doc.Tasks[0].State(); got != models.StateWaiting {
		t.Fatalf("derived state = %q, want %q", got, models.StateWaiting)
	}

	// Upgrade: rendering as v2 emits the marker and the canonical state field.
	doc.SchemaVersion = SchemaVersionV2
	v2 := doc.WriteV2()
	if !strings.Contains(v2, "state: waiting") {
		t.Fatalf("upgraded v2 missing canonical state:\n%s", v2)
	}
	if strings.Contains(v2, "status:") || strings.Contains(v2, "section:") {
		t.Fatalf("upgraded v2 leaked legacy fields:\n%s", v2)
	}

	reparsed, err := ParseProjectDocument(v2)
	if err != nil {
		t.Fatalf("re-parse upgraded doc: %v", err)
	}
	if got := reparsed.Tasks[0].State(); got != models.StateWaiting {
		t.Fatalf("post-upgrade state = %q, want %q", got, models.StateWaiting)
	}
}

// TestProjectDocumentPreservesOpaqueBlocks verifies unknown header comments
// survive a v2 round-trip losslessly.
func TestProjectDocumentPreservesOpaqueBlocks(t *testing.T) {
	src := "# Project: Opaque\n" +
		"<!-- tt-schema: 2 -->\n" +
		"<!-- future-feature: keep-me -->\n" +
		"<!-- sections: Next -->\n\n" +
		"## Task: Only task\n" +
		"<!-- id: op-1, state: ready, priority: 1, created: 2025-03-01 08:00:00 -->\n"

	doc, err := ParseProjectDocument(src)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if len(doc.OpaqueBlocks) != 1 || doc.OpaqueBlocks[0].Content != "<!-- future-feature: keep-me -->" {
		t.Fatalf("opaque blocks = %#v, want single future-feature comment", doc.OpaqueBlocks)
	}

	out := doc.WriteV2()
	if !strings.Contains(out, "<!-- future-feature: keep-me -->") {
		t.Fatalf("v2 output dropped opaque block:\n%s", out)
	}

	// Fixed point including the opaque block.
	reparsed, err := ParseProjectDocument(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if reparsed.WriteV2() != out {
		t.Fatalf("opaque-block round-trip not a fixed point:\n%s\n---\n%s", out, reparsed.WriteV2())
	}
}

// TestProjectDocumentMalformedState verifies a bogus state value warns rather
// than crashing, and the task still loads.
func TestProjectDocumentMalformedState(t *testing.T) {
	src := "# Project: Malformed\n" +
		"<!-- tt-schema: 2 -->\n\n" +
		"## Task: Bad state\n" +
		"<!-- id: bad-1, state: not-a-real-state, priority: 1, created: 2025-03-01 08:00:00 -->\n"

	doc, err := ParseProjectDocument(src)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(doc.Tasks))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Field == "state" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a state warning, got %v", doc.Warnings)
	}
}

// TestProjectDocumentHierarchyRoundTrip verifies parent/child relationships and
// slice order survive a v2 round-trip.
func TestProjectDocumentHierarchyRoundTrip(t *testing.T) {
	epic := newDocTask(t, "epic-h", 1, models.TaskTypeEpic, "Parent epic")
	epic.ApplyWorkflowState(models.StateReady)
	story := newDocTask(t, "story-h", 2, models.TaskTypeStory, "Child story")
	story.ParentID = &epic.ID
	story.ApplyWorkflowState(models.StateNext)
	leaf := newDocTask(t, "task-h", 3, models.TaskTypeTask, "Grandchild task")
	leaf.ParentID = &story.ID
	leaf.ApplyWorkflowState(models.StateInProgress)

	doc := &ProjectDocument{
		ProjectName:   "Hierarchy",
		Tasks:         []*models.Task{epic, story, leaf},
		SchemaVersion: SchemaVersionV2,
	}

	reparsed, err := ParseProjectDocument(doc.WriteV2())
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if len(reparsed.Tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(reparsed.Tasks))
	}
	gotStory := findTaskByID(reparsed.Tasks, "story-h")
	gotLeaf := findTaskByID(reparsed.Tasks, "task-h")
	if gotStory == nil || gotStory.ParentID == nil || *gotStory.ParentID != "epic-h" {
		t.Fatalf("story parent = %v, want epic-h", gotStory)
	}
	if gotLeaf == nil || gotLeaf.ParentID == nil || *gotLeaf.ParentID != "story-h" {
		t.Fatalf("leaf parent = %v, want story-h", gotLeaf)
	}
}

// TestProjectDocumentArchiveState verifies a terminal (done) task with a
// completion timestamp round-trips through v2 with state and CompletedAt intact —
// the shape archived/completed tasks take.
func TestProjectDocumentArchiveState(t *testing.T) {
	done := newDocTask(t, "done-1", 1, models.TaskTypeTask, "Finished work")
	done.ApplyWorkflowState(models.StateDone)
	completed := time.Date(2025, 3, 2, 12, 0, 0, 0, time.Local)
	done.CompletedAt = &completed

	doc := &ProjectDocument{
		ProjectName:   "Archive",
		Tasks:         []*models.Task{done},
		SchemaVersion: SchemaVersionV2,
	}

	out := doc.WriteV2()
	if !strings.Contains(out, "state: done") {
		t.Fatalf("v2 output missing done state:\n%s", out)
	}
	if !strings.Contains(out, "completed: 2025-03-02") {
		t.Fatalf("v2 output missing completed timestamp:\n%s", out)
	}

	reparsed, err := ParseProjectDocument(out)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	got := findTaskByID(reparsed.Tasks, "done-1")
	if got == nil || got.State() != models.StateDone {
		t.Fatalf("reparsed state = %v, want done", got)
	}
	if got.CompletedAt == nil || !got.CompletedAt.Equal(completed) {
		t.Fatalf("reparsed CompletedAt = %v, want %v", got.CompletedAt, completed)
	}
}

// TestProjectDocumentManualEdit verifies a hand-authored v2 file (minimal
// whitespace, canonical `state`, no legacy fields) loads and reconstitutes the
// legacy backing fields the rest of the domain still reads.
func TestProjectDocumentManualEdit(t *testing.T) {
	manual := "# Project: Manual\n" +
		"<!-- tt-schema: 2 -->\n\n" +
		"## Task: Hand written\n" +
		"<!-- id: man-1, state: someday, priority: 3, created: 2025-03-01 08:00:00 -->\n"

	doc, err := ParseProjectDocument(manual)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	task := findTaskByID(doc.Tasks, "man-1")
	if task == nil {
		t.Fatalf("manual task not parsed")
	}
	if task.State() != models.StateSomeday {
		t.Fatalf("state = %q, want someday", task.State())
	}
	if got := task.State(); got != models.StateSomeday {
		t.Fatalf("derived section = %q, want %q", got, models.StateSomeday)
	}
}

// TestLoadSaveProjectDocumentV2 verifies the opt-in v2 filesystem round-trip
// through SaveProjectDocumentV2 / LoadProjectDocument.
func TestLoadSaveProjectDocumentV2(t *testing.T) {
	dir := t.TempDir()
	project, err := models.NewProject("v2-disk-id", "V2 Disk", dir)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "v2-disk"

	task := newDocTask(t, "disk-1", 1, models.TaskTypeTask, "Disk task")
	task.ApplyWorkflowState(models.StateNext)
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	if saveErr := SaveProjectDocumentV2(project, nil); saveErr != nil {
		t.Fatalf("SaveProjectDocumentV2: %v", saveErr)
	}

	doc, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	if doc.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("schema version = %d, want v2", doc.SchemaVersion)
	}
	if len(doc.Tasks) != 1 || doc.Tasks[0].State() != models.StateNext {
		t.Fatalf("loaded doc tasks = %#v, want one next task", doc.Tasks)
	}
	if doc.Revision == "" {
		t.Fatalf("expected non-empty revision")
	}
}
