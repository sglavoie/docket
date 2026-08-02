package config

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
)

func TestAdaptV6WorkspaceToRuntimePreservesValidatedSchema6Values(t *testing.T) {
	stamp := time.Date(2026, 8, 1, 10, 30, 0, 0, time.UTC)
	pinnedAt := stamp.Add(time.Minute)
	completedAt := stamp.Add(2 * time.Minute)
	addedAt := stamp.Add(3 * time.Minute)
	states := []string{"inbox", "backlog", "doing", "waiting", "paused", "someday", "done", "canceled"}
	tasks := make([]format.V6TaskRecord, 0, len(states))
	for index, state := range states {
		record := format.V6TaskRecord{
			ID:        "task-" + state,
			ShortID:   index + 1,
			Title:     "Task " + state,
			State:     state,
			Priority:  index % 4,
			CreatedAt: stamp,
			UpdatedAt: stamp.Add(time.Duration(index) * time.Minute),
		}
		if state == "done" || state == "canceled" {
			record.CompletedAt = &completedAt
		}
		tasks = append(tasks, record)
	}
	tasks[1].Description = "retained description"
	tasks[1].Tags = []string{"bug", "source:github"}
	tasks[1].DueDate = "2026-08-03"
	tasks[1].SnoozedUntil = "2026-08-02"
	tasks[1].Pinned = true
	tasks[1].PinnedAt = &pinnedAt
	tasks[1].Attachments = []format.V6TaskAttachment{
		{Kind: "file", Filename: "spec.md", OriginalPath: "/tmp/spec.md", AddedAt: &addedAt},
		{Kind: "url", Filename: "https://example.com"},
		{Kind: "code", Filename: "main.go", OriginalPath: "/tmp/main.go", LineNumber: 42},
	}
	tasks[1].Checklist = []format.V6ChecklistItem{{Text: "first", Done: true}}
	tasks[1].Comments = []format.V6TaskComment{{Text: "converted decision", CreatedAt: stamp}}
	tasks[2].ParentID = tasks[1].ID

	unfiledID := "project-unfiled"
	alphaID := "project-alpha"
	domains := V6DomainCandidates{
		Catalog: V6WorkspaceCatalogCandidate{
			SchemaVersion: V6ConfigSchemaVersion,
			WorkspaceID:   "11111111-1111-4111-8111-111111111111",
			Projects: []V6CatalogProjectCandidate{
				{ID: unfiledID, Slug: V6UnfiledProjectSlug, Name: V6UnfiledProjectName, Context: "personal", CreatedAt: stamp, UpdatedAt: stamp, ColorSlot: 1},
				{ID: alphaID, Slug: "alpha", Name: "Alpha", Description: "project description", Context: "work", CreatedAt: stamp, UpdatedAt: stamp, Pinned: true, PinnedAt: &pinnedAt, ColorSlot: 2},
			},
			SystemUnfiledProjectID: unfiledID,
		},
		Planning: V6PlanningCandidate{
			Today: []ProjectTaskLocator{{ProjectID: alphaID, TaskID: tasks[1].ID}},
			Next:  []ProjectTaskLocator{{ProjectID: alphaID, TaskID: tasks[2].ID}},
		},
		Machine: V6MachineCandidate{Theme: "light"},
	}
	projects := []V6ProjectDocumentCandidate{
		{ProjectID: unfiledID, SourcePath: "/source/unfiled/tasks.md", Path: "/target/unfiled/tasks.md", SourceExists: true, Document: storage.V6ProjectDocument{ProjectName: V6UnfiledProjectName}},
		{ProjectID: alphaID, SourcePath: "/source/alpha/tasks.md", Path: "/target/alpha/tasks.md", SourceExists: true, Document: storage.V6ProjectDocument{ProjectName: "Alpha", Tasks: tasks}},
	}
	location := time.FixedZone("adapter-test", -6*60*60)
	dataDir := t.TempDir()

	adapted, err := AdaptV6WorkspaceToRuntime(dataDir, domains, projects, location)
	if err != nil {
		t.Fatalf("AdaptV6WorkspaceToRuntime: %v", err)
	}
	if adapted.Runtime == nil || adapted.SystemUnfiledProjectID != unfiledID {
		t.Fatalf("adapted runtime/protected ID = %+v", adapted)
	}
	if adapted.Runtime.Catalog().SchemaVersion != V6ConfigSchemaVersion ||
		adapted.Runtime.Catalog().SystemInboxProjectID != unfiledID {
		t.Fatalf("runtime catalog = %+v", adapted.Runtime.Catalog())
	}
	if adapted.Runtime.Machine().Theme != "light" {
		t.Fatalf("runtime machine = %+v, want theme-only", adapted.Runtime.Machine())
	}
	if !reflect.DeepEqual(adapted.Runtime.Planning().Today, domains.Planning.Today) ||
		!reflect.DeepEqual(adapted.Runtime.Planning().NextOrder, domains.Planning.Next) {
		t.Fatalf("runtime planning = %+v, want %+v", adapted.Runtime.Planning(), domains.Planning)
	}

	if len(adapted.Projects) != 2 {
		t.Fatalf("runtime project count = %d, want 2", len(adapted.Projects))
	}
	alpha := adapted.Projects[1]
	if alpha.Path != filepath.Join(dataDir, "projects", "alpha") || alpha.NextShortID != 9 {
		t.Fatalf("runtime alpha path/next short ID = %q/%d", alpha.Path, alpha.NextShortID)
	}
	wantStates := []models.WorkflowState{
		models.StateInbox,
		models.StateBacklog,
		models.StateDoing,
		models.StateWaiting,
		models.StatePaused,
		models.StateSomeday,
		models.StateDone,
		models.StateCanceled,
	}
	for index, want := range wantStates {
		if got := alpha.Tasks[index].WorkflowState; got != want {
			t.Errorf("task %d state = %q, want %q", index, got, want)
		}
		if alpha.Tasks[index].Priority != tasks[index].Priority {
			t.Errorf("task %d priority = %d, want %d", index, alpha.Tasks[index].Priority, tasks[index].Priority)
		}
		if alpha.Tasks[index].Project != alpha {
			t.Errorf("task %d project pointer not linked", index)
		}
	}
	backlog := alpha.Tasks[1]
	if backlog.Description != "retained description" || !reflect.DeepEqual(backlog.Tags, tasks[1].Tags) {
		t.Fatalf("runtime backlog content = %+v", backlog)
	}
	if backlog.DueDate == nil || backlog.SnoozedUntil == nil || backlog.DueDate.Location() != location || backlog.SnoozedUntil.Location() != location {
		t.Fatalf("runtime calendar dates = %v/%v, want adapter location", backlog.DueDate, backlog.SnoozedUntil)
	}
	if len(backlog.Attachments) != 3 || !backlog.Attachments[1].IsURL || !backlog.Attachments[2].IsCodeRef || backlog.Attachments[2].LineNumber != 42 {
		t.Fatalf("runtime attachments = %+v", backlog.Attachments)
	}
	if len(backlog.Checklist) != 1 || !backlog.Checklist[0].Done || len(backlog.Comments) != 1 {
		t.Fatalf("runtime checklist/comments = %+v/%+v", backlog.Checklist, backlog.Comments)
	}
	if len(backlog.Decisions) != 0 {
		t.Fatalf("schema-6 task gained retired values: %+v", backlog)
	}
	child := alpha.Tasks[2]
	if child.Parent != backlog || child.Level != 1 || len(backlog.Children) != 1 || backlog.Children[0] != child {
		t.Fatalf("runtime hierarchy not linked: parent=%p level=%d children=%v", child.Parent, child.Level, backlog.Children)
	}

	// The adapter result must not alias mutable candidate collections.
	domains.Catalog.Projects[1].Name = "Mutated source"
	domains.Planning.Today[0].TaskID = "mutated"
	projects[1].Document.Tasks[1].Tags[0] = "mutated"
	if alpha.Name != "Alpha" || adapted.Runtime.Planning().Today[0].TaskID != tasks[1].ID || backlog.Tags[0] != "bug" {
		t.Fatalf("runtime adapter aliases mutable candidate data")
	}
}

func TestAdaptV6WorkspaceToRuntimeRejectsInvalidCandidates(t *testing.T) {
	if _, err := AdaptV6WorkspaceToRuntime("", V6DomainCandidates{}, nil, time.UTC); err == nil {
		t.Fatal("blank data directory unexpectedly adapted")
	}
	if _, err := AdaptV6WorkspaceToRuntime(t.TempDir(), V6DomainCandidates{}, nil, time.UTC); err == nil {
		t.Fatal("invalid schema-6 domains unexpectedly adapted")
	}
}
