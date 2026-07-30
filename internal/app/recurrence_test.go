package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// recurringTask builds a leaf task already carrying the given recurrence pattern.
func recurringTask(t *testing.T, id string, shortID int, pattern string) *models.Task {
	t.Helper()
	task := newTask(id, shortID, "Recurring")
	p := models.ParseRecurrence(pattern)
	if p == nil {
		t.Fatalf("ParseRecurrence(%q) returned nil", pattern)
	}
	if err := task.SetRecurrence(p); err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	return task
}

func TestSetRecurrenceAppliesPatternAndCommits(t *testing.T) {
	task := newTask("t1", 1, "Task")
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.SetRecurrence(task.ID, models.ParseRecurrence("weekly"))
	if err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	if res.Pattern != "weekly" {
		t.Errorf("Pattern = %q, want weekly", res.Pattern)
	}
	if task.Recurrence == nil || task.Recurrence.String() != "weekly" {
		t.Errorf("task recurrence not set: %+v", task.Recurrence)
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatal("expected exactly one committed changeset")
	}
	if cs.UndoAction != "Set Recurrence" {
		t.Errorf("UndoAction = %q, want Set Recurrence", cs.UndoAction)
	}
}

func TestSetRecurrenceNilPatternRejected(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SetRecurrence(task.ID, nil); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SetRecurrence(nil) error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit on rejected input, got %d", len(repo.commits))
	}
}

func TestSetRecurrenceInvalidPatternRejected(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	// A zero-value pattern has Interval 0 and an empty unit, so Validate rejects it.
	if _, err := svc.SetRecurrence(task.ID, &models.RecurrencePattern{}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SetRecurrence(invalid) error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit on validation failure, got %d", len(repo.commits))
	}
	if task.Recurrence != nil {
		t.Errorf("task recurrence should be untouched, got %+v", task.Recurrence)
	}
}

func TestSetRecurrenceMissingTaskID(t *testing.T) {
	repo := newFakeRepo(newProject("p1", "Alpha", newTask("t1", 1, "Task")))
	svc := newTestService(repo)

	if _, err := svc.SetRecurrence("", models.ParseRecurrence("weekly")); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SetRecurrence(empty id) error = %v, want ErrInvalidInput", err)
	}
}

func TestClearRecurrenceRemovesPattern(t *testing.T) {
	task := recurringTask(t, "t1", 1, "weekly")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.ClearRecurrence(task.ID)
	if err != nil {
		t.Fatalf("ClearRecurrence: %v", err)
	}
	if res.Pattern != "" {
		t.Errorf("Pattern = %q, want empty", res.Pattern)
	}
	if task.Recurrence != nil {
		t.Errorf("recurrence should be nil after clear, got %+v", task.Recurrence)
	}
	if cs, _ := repo.lastCommit(); cs.UndoAction != "Clear Recurrence" {
		t.Errorf("UndoAction = %q, want Clear Recurrence", cs.UndoAction)
	}
}

func TestSkipRecurrenceAdvancesDueAndCommits(t *testing.T) {
	task := recurringTask(t, "t1", 1, "weekly")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.SkipRecurrence(task.ID)
	if err != nil {
		t.Fatalf("SkipRecurrence: %v", err)
	}
	if res.DueDate == nil {
		t.Fatal("expected a due date after skip")
	}
	if cs, _ := repo.lastCommit(); cs.UndoAction != "Skip Recurrence" {
		t.Errorf("UndoAction = %q, want Skip Recurrence", cs.UndoAction)
	}
}

func TestSkipRecurrenceNonRecurringRejected(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.SkipRecurrence(task.ID); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SkipRecurrence(non-recurring) error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit, got %d", len(repo.commits))
	}
}

func TestDeferRecurrenceNonPositiveRejected(t *testing.T) {
	task := recurringTask(t, "t1", 1, "weekly")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.DeferRecurrence(task.ID, 0); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("DeferRecurrence(0) error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit, got %d", len(repo.commits))
	}
}

func TestDeferRecurrenceNonRecurringRejected(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.DeferRecurrence(task.ID, 3); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("DeferRecurrence(non-recurring) error = %v, want ErrInvalidInput", err)
	}
}

func TestSetRecurrenceCommitErrorSurfaces(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	sentinel := errors.New("injected commit failure")
	repo.commitErr = sentinel
	svc := newTestService(repo)

	_, err := svc.SetRecurrence(task.ID, models.ParseRecurrence("weekly"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("SetRecurrence error = %v, want injected failure", err)
	}
	if task.Recurrence != nil {
		t.Fatalf("failed SetRecurrence left pattern %+v, want rollback to nil", task.Recurrence)
	}
}

func TestSkipRecurrenceRetriesOnStaleRevision(t *testing.T) {
	// A stale-revision conflict propagates the sentinel so the CLI retry wrapper can
	// re-drive the mutation; here we assert the app surfaces storage.ErrStaleRevision.
	task := recurringTask(t, "t1", 1, "weekly")
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	// Advance the stored revision out from under the snapshot so the commit's
	// ExpectedRevision no longer matches.
	repo.preCommit = func() { repo.revisions[project.ID] = "diverged-revision" }
	svc := newTestService(repo)

	if _, err := svc.SkipRecurrence(task.ID); !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("SkipRecurrence error = %v, want ErrStaleRevision", err)
	}
}
