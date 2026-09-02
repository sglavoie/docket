package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestSetPriorityValidatesRange(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetPriority(repo.loc("t1"), 99); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetPriority(repo.loc("t1"), -1); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetPriority(repo.loc("t1"), 1); err != nil {
		t.Fatalf("SetPriority(1): %v", err)
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
	// Zero is the unset value, not an out-of-range priority, so it clears.
	if _, err := svc.SetPriority(repo.loc("t1"), 0); err != nil {
		t.Fatalf("SetPriority(0): %v", err)
	}
	if task.Priority != 0 {
		t.Errorf("priority = %d, want 0", task.Priority)
	}
}

func TestSetDueDateSetsAndClears(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	if _, err := svc.SetDueDate(repo.loc("t1"), &due); err != nil {
		t.Fatalf("SetDueDate: %v", err)
	}
	if task.DueDate == nil || task.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
		t.Fatalf("due=%v, want date %s", task.DueDate, due.Format("2006-01-02"))
	}
	if _, err := svc.SetDueDate(repo.loc("t1"), nil); err != nil {
		t.Fatalf("clear SetDueDate: %v", err)
	}
	if task.DueDate != nil {
		t.Errorf("due=%v, want cleared", task.DueDate)
	}
}

func TestToggleTagAddsAndRemoves(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "#backend"); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	if !task.HasTag("backend") {
		t.Errorf("tags = %v, want backend", task.Tags)
	}
	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "backend"); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	if task.HasTag("backend") {
		t.Errorf("tags = %v, want backend removed", task.Tags)
	}
}

func TestToggleTagAllowsIndependentSemanticTags(t *testing.T) {
	task := newTask("t1", 1, "task")
	task.Tags = []string{"idea", "keep"}
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "bug"); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	if !slices.Equal(task.Tags, []string{"idea", "keep", "bug"}) || len(repo.commits) != 1 {
		t.Fatalf("independent add tags=%v commits=%d", task.Tags, len(repo.commits))
	}
}

func TestSetPinnedTogglesPin(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetPinned(repo.loc("t1"), true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if !task.Pinned {
		t.Error("task not pinned")
	}
	if _, err := svc.SetPinned(repo.loc("t1"), false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if task.Pinned {
		t.Error("task still pinned")
	}
}

func TestDeferAndClear(t *testing.T) {
	task := newTask("t1", 1, "task")
	repo := newFakeRepo(newProject("proj-1", "Alpha", task))
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Defer(repo.loc("t1"), &until); err != nil {
		t.Fatalf("Defer: %v", err)
	}
	if task.DeferredUntil == nil || !task.DeferredUntil.Equal(until) {
		t.Fatalf("DeferredUntil = %v, want %v", task.DeferredUntil, until)
	}
	if task.DeferCount != 1 {
		t.Fatalf("DeferCount = %d, want 1", task.DeferCount)
	}
	if _, err := svc.Defer(repo.loc("t1"), nil); err != nil {
		t.Fatalf("clear Defer: %v", err)
	}
	if task.DeferredUntil != nil {
		t.Errorf("DeferredUntil = %v, want cleared", task.DeferredUntil)
	}
	if task.DeferCount != 1 {
		t.Errorf("clear changed DeferCount to %d, want 1", task.DeferCount)
	}
}

func TestCaptureWithDeferralStartsTheCount(t *testing.T) {
	project := newProject("proj-1", "Alpha")
	repo := newFakeRepo(project)
	until := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	if _, err := newTestService(repo).Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Later",
		DeferSet:  true,
		Defer:     &until,
	}); err != nil {
		t.Fatalf("Capture deferred task: %v", err)
	}
	if len(project.Tasks) != 1 {
		t.Fatalf("captured tasks = %d, want 1", len(project.Tasks))
	}
	if project.Tasks[0].DeferCount != 1 {
		t.Fatalf("captured defer count = %d, want 1", project.Tasks[0].DeferCount)
	}
}

// TestDeferRejectsCompletedAndBacklogsDoing proves terminal work is rejected
// while active work is normalized to Backlog before being hidden.
func TestDeferRejectsCompletedAndBacklogsDoing(t *testing.T) {
	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	done := newTask("t1", 1, "task")
	done.Complete()
	inProgress := newTask("t2", 2, "task")
	inProgress.Start()
	repo := newFakeRepo(newProject("proj-1", "Alpha", done, inProgress))
	svc := newTestService(repo)

	if _, err := svc.Defer(repo.loc("t1"), &until); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("deferral done: err = %v, want ErrInvalidInput", err)
	}
	if done.DeferredUntil != nil {
		t.Error("completed task should not be deferred")
	}
	if _, err := svc.Defer(repo.loc("t2"), &until); err != nil {
		t.Fatalf("deferral Doing: %v", err)
	}
	if inProgress.State() != models.StateBacklog || inProgress.DeferredUntil == nil {
		t.Fatalf("deferred Doing task = state %s date %v, want Backlog with date", inProgress.State(), inProgress.DeferredUntil)
	}
}

func TestMutationTaskNotFound(t *testing.T) {
	repo := newFakeRepo(newProject("proj-1", "Alpha"))
	svc := newTestService(repo)

	if _, err := svc.SetPriority(app.TaskLocator{ProjectID: "proj-1", TaskID: "ghost"}, 2); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestOptimisticConcurrencyConflictSurfacesStaleRevision(t *testing.T) {
	task := newTask("t1", 1, "task")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	// Simulate a concurrent writer advancing the project's revision after the
	// use case captured its snapshot revision but before it commits.
	repo.preCommit = func() {
		repo.revisions[project.ID] = "diverged-revision"
	}

	_, err := svc.SetPriority(repo.loc("t1"), 2)
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("err = %v, want storage.ErrStaleRevision", err)
	}
}

func TestCommitCarriesCapturedExpectedRevision(t *testing.T) {
	task := newTask("t1", 1, "task")
	project := newProject("proj-1", "Alpha", task)
	repo := newFakeRepo(project)
	captured := repo.revisions[project.ID]
	capturedBase := repo.documents[project.ID]
	svc := newTestService(repo)

	if _, err := svc.BulkToggleTag([]app.TaskLocator{repo.loc("t1")}, "focus"); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatalf("want exactly one changeset in one commit, got %v", repo.commits)
	}
	if cs.ExpectedRevision != captured {
		t.Errorf("ExpectedRevision = %q, want %q", cs.ExpectedRevision, captured)
	}
	if cs.Base == nil {
		t.Fatal("ChangeSet.Base is nil; want exact loaded ProjectDocument")
	}
	if cs.Base != capturedBase {
		t.Fatal("ChangeSet.Base is not the document retained by the snapshot")
	}
	if len(cs.Base.Tasks[0].Tags) != 0 {
		t.Fatalf("snapshot base was mutated with local project: tags=%v", cs.Base.Tasks[0].Tags)
	}
	if cs.UndoAction != "Add Tag" {
		t.Errorf("UndoAction = %q, want Add Tag", cs.UndoAction)
	}
}
