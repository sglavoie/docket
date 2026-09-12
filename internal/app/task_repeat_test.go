package app_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// repeatSourceTask builds a task carrying one value in every field a repetition
// might plausibly copy, so a draft that quietly retained an occurrence field has
// something to retain rather than a zero value that would pass by accident.
func repeatSourceTask() *models.Task {
	task := newTask("task-1", 7, "Ship the quarterly report")
	task.Description = "Pull the numbers, then circulate for sign-off."
	task.Priority = 2
	task.Tags = []string{"finance", "recurring"}
	task.Context = "office"
	task.ApplyWorkflowState(models.StateDone)
	task.Checklist = []models.ChecklistItem{
		{Text: "Pull the numbers", Done: true},
		{Text: "Draft the summary", Done: true},
		{Text: "Circulate", Done: false},
	}
	due := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	task.DueDate = &due
	deferred := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	task.DeferredUntil = &deferred
	pinned := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	task.Pinned = true
	task.PinnedAt = &pinned
	task.Review = true
	task.ResumeNote = "Waiting on the finance export"
	stamp := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	task.Comments = []*models.Comment{{Text: "Numbers landed", CreatedAt: stamp}}
	task.Attachments = []models.Attachment{{Filename: "https://example.test/report", IsURL: true, AddedAt: stamp}}
	task.Dependencies = []models.TaskReference{{ProjectID: "proj-1", TaskID: "task-2"}}
	parent := "task-2"
	task.ParentID = &parent
	task.ChecklistOrigin = &models.ChecklistOrigin{ProjectID: "proj-1", TaskID: parent, Index: 2}
	return task
}

// TestPrepareTaskRepeatDraftRetainsOnlyTheShapeOfTheWork asserts both halves of
// the copy/reset matrix on one source: every retained field arrives, and the
// draft type has no way at all to express the cleared ones. The absences are
// checked structurally — the struct's whole field set is enumerated below — so a
// field added to TaskRepeatDraft without a decision fails this test.
func TestPrepareTaskRepeatDraftRetainsOnlyTheShapeOfTheWork(t *testing.T) {
	source := repeatSourceTask()
	project := newProject("proj-1", "Alpha", newTask("task-2", 2, "Parent task"), source)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	draft, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-1", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("PrepareTaskRepeatDraft: %v", err)
	}

	want := app.TaskRepeatDraft{
		Destination:    app.CaptureDestinationBacklog,
		Title:          "Ship the quarterly report",
		Description:    "Pull the numbers, then circulate for sign-off.",
		Priority:       2,
		Tags:           []string{"finance", "recurring"},
		Context:        "office",
		ChecklistItems: []string{"Pull the numbers", "Draft the summary", "Circulate"},
	}
	if draft.Destination != want.Destination {
		t.Errorf("Destination = %q, want %q", draft.Destination, want.Destination)
	}
	if draft.Title != want.Title {
		t.Errorf("Title = %q, want %q", draft.Title, want.Title)
	}
	if draft.Description != want.Description {
		t.Errorf("Description = %q, want %q", draft.Description, want.Description)
	}
	if draft.Priority != want.Priority {
		t.Errorf("Priority = %d, want %d", draft.Priority, want.Priority)
	}
	if !slices.Equal(draft.Tags, want.Tags) {
		t.Errorf("Tags = %v, want %v", draft.Tags, want.Tags)
	}
	if draft.Context != want.Context {
		t.Errorf("Context = %q, want %q", draft.Context, want.Context)
	}
	if !slices.Equal(draft.ChecklistItems, want.ChecklistItems) {
		t.Errorf("ChecklistItems = %v, want %v", draft.ChecklistItems, want.ChecklistItems)
	}
}

// TestTaskRepeatDraftCarriesNoOccurrenceField enumerates the draft type's whole
// field set. It is the cleared half of the matrix stated once and structurally:
// the fields below are all there is, so no due date, deferral, parent, planning
// membership, flag, resume note, comment, attachment, dependency, identity, or
// timestamp can reach a repetition however the seeding code is later rewritten.
func TestTaskRepeatDraftCarriesNoOccurrenceField(t *testing.T) {
	value := reflect.TypeOf(app.TaskRepeatDraft{})
	fields := make([]string, 0, value.NumField())
	for i := range value.NumField() {
		fields = append(fields, value.Field(i).Name)
	}
	want := []string{
		"Destination", "Title", "Description",
		"Priority", "Tags", "Context", "ChecklistItems",
	}
	if !slices.Equal(fields, want) {
		t.Fatalf("TaskRepeatDraft fields = %v, want exactly %v; a new field needs a "+
			"decision in the repeat copy/reset matrix, not a default", fields, want)
	}
}

// TestPrepareTaskRepeatDraftWritesNothing is the "opening writes nothing" half
// of the contract: no transaction, no artifact commit, and the source task
// untouched down to its touch time.
func TestPrepareTaskRepeatDraftWritesNothing(t *testing.T) {
	source := repeatSourceTask()
	project := newProject("proj-1", "Alpha", newTask("task-2", 2, "Parent task"), source)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	before := *source
	revision := repo.revisions[project.ID]

	if _, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-1", TaskID: "task-1"}); err != nil {
		t.Fatalf("PrepareTaskRepeatDraft: %v", err)
	}

	if len(repo.commits) != 0 {
		t.Errorf("commits = %v, want none: preparing a draft is a pure read", repo.commits)
	}
	if len(repo.artifactCommits) != 0 {
		t.Errorf("artifact commits = %d, want none", len(repo.artifactCommits))
	}
	if repo.revisions[project.ID] != revision {
		t.Error("preparing a draft advanced the project revision")
	}
	if !source.UpdatedAt.Equal(before.UpdatedAt) || source.Title != before.Title {
		t.Error("preparing a draft mutated the source task")
	}
}

// TestPrepareTaskRepeatDraftOwnsItsMemory pins the independence claim at the one
// place aliasing could creep in. A draft sharing the source's backing arrays
// would let an edit on either side reach the other, which is the whole reason
// the preparation copies rather than references.
func TestPrepareTaskRepeatDraftOwnsItsMemory(t *testing.T) {
	source := repeatSourceTask()
	project := newProject("proj-1", "Alpha", newTask("task-2", 2, "Parent task"), source)
	svc := newTestService(newFakeRepo(project))

	draft, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-1", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("PrepareTaskRepeatDraft: %v", err)
	}

	draft.Tags[0] = "mutated"
	draft.ChecklistItems[0] = "mutated"
	if source.Tags[0] != "finance" {
		t.Errorf("source tag = %q, want finance: the draft aliases the task's tags", source.Tags[0])
	}
	if source.Checklist[0].Text != "Pull the numbers" {
		t.Errorf("source checklist item = %q, want the stored text: the draft aliases the checklist",
			source.Checklist[0].Text)
	}

	source.Tags[1] = "changed after preparation"
	source.Checklist[2].Text = "changed after preparation"
	source.Title = "Renamed after preparation"
	if draft.Tags[1] != "recurring" || draft.ChecklistItems[2] != "Circulate" {
		t.Error("a source edit after preparation reached the draft")
	}
	if draft.Title != "Ship the quarterly report" {
		t.Errorf("draft title = %q, want the value captured at preparation", draft.Title)
	}
}

// TestPrepareTaskRepeatDraftKeepsLiteralSigilsAndUnicode is the parser-avoidance
// claim stated at the layer that produces the value: the draft is stored text,
// not a capture line, so sigils survive as characters and multi-byte titles come
// back rune for rune.
func TestPrepareTaskRepeatDraftKeepsLiteralSigilsAndUnicode(t *testing.T) {
	source := newTask("task-1", 1, "Fix bug #123 $today @tomorrow *pin — café ☕ 日本語")
	source.Description = "Check %billing and ~task-9 in the ;notes;"
	source.Checklist = []models.ChecklistItem{{Text: "Review !1 items ✓", Done: true}}
	// Tags and contexts are canonical kebab case at the storage boundary, so the
	// Unicode claim is made where free text actually lives: title, description,
	// and checklist item.
	source.Tags = []string{"quarterly"}
	project := newProject("proj-1", "Alpha", source)
	svc := newTestService(newFakeRepo(project))

	draft, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-1", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("PrepareTaskRepeatDraft: %v", err)
	}

	if draft.Title != source.Title {
		t.Errorf("Title = %q, want the literal %q", draft.Title, source.Title)
	}
	if draft.Description != source.Description {
		t.Errorf("Description = %q, want the literal %q", draft.Description, source.Description)
	}
	if !slices.Equal(draft.ChecklistItems, []string{"Review !1 items ✓"}) {
		t.Errorf("ChecklistItems = %v, want the literal item", draft.ChecklistItems)
	}
	if !slices.Equal(draft.Tags, []string{"quarterly"}) {
		t.Errorf("Tags = %v, want the literal tag", draft.Tags)
	}
}

// TestPrepareTaskRepeatDraftAcceptsEveryLifecycleState covers the open/terminal
// half of the admission rule in one sweep: finishing a task is the usual reason
// to want another like it, so no state may refuse.
func TestPrepareTaskRepeatDraftAcceptsEveryLifecycleState(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		t.Run(string(state), func(t *testing.T) {
			source := newTask("task-1", 1, "Repeatable work")
			source.ApplyWorkflowState(state)
			project := newProject("proj-1", "Alpha", source)
			svc := newTestService(newFakeRepo(project))

			draft, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-1", TaskID: "task-1"})
			if err != nil {
				t.Fatalf("PrepareTaskRepeatDraft from %s: %v", state, err)
			}
			if draft.Destination != app.CaptureDestinationBacklog {
				t.Errorf("Destination = %q, want backlog whatever the source state", draft.Destination)
			}
		})
	}
}

// TestPrepareTaskRepeatDraftRefusesArchivedAndMissing covers the two ways the
// preparation can fail. Parked is deliberately absent from the refusals: parking
// hides a project from the default population rather than sealing it, so a
// parked owner prepares like any other.
func TestPrepareTaskRepeatDraftRefusesArchivedAndMissing(t *testing.T) {
	archived := newProject("proj-archived", "Retired", newTask("task-1", 1, "Old work"))
	archived.Archived = true
	parked := newProject("proj-parked", "Parked", newTask("task-2", 1, "Parked work"))
	parked.Tags = []string{"parked"}
	svc := newTestService(newFakeRepo(archived, parked))

	if _, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-archived", TaskID: "task-1"}); err == nil {
		t.Error("an archived owner prepared a repeat draft")
	}
	if _, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-parked", TaskID: "task-2"}); err != nil {
		t.Errorf("a parked owner refused a repeat draft: %v", err)
	}
	if _, err := svc.PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: "proj-parked", TaskID: "missing"}); err == nil {
		t.Error("a missing task prepared a repeat draft")
	}
}
