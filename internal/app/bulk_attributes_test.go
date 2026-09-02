package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestBulkSetPrioritySetsAllAndCommitsOnce(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.BulkSetPriority([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, 2)
	if err != nil {
		t.Fatalf("BulkSetPriority: %v", err)
	}
	if t1.Priority != 2 || t2.Priority != 2 {
		t.Errorf("priorities = %d/%d, want 2/2", t1.Priority, t2.Priority)
	}
	if len(res.Tasks) != 2 {
		t.Errorf("result tasks = %d, want 2", len(res.Tasks))
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
	if got := repo.commits[0][0].UndoAction; got != "Set Priority" {
		t.Errorf("UndoAction = %q, want Set Priority", got)
	}

	// Zero is the unset value, not an out-of-range priority, so it clears in bulk too.
	if _, err = svc.BulkSetPriority([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, 0); err != nil {
		t.Fatalf("BulkSetPriority(0): %v", err)
	}
	if t1.Priority != 0 || t2.Priority != 0 {
		t.Errorf("cleared priorities = %d/%d, want 0/0", t1.Priority, t2.Priority)
	}
}

func TestBulkSetPriorityRejectsInvalidPriority(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	_, err := svc.BulkSetPriority([]app.TaskLocator{repo.loc("t1")}, 99)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if _, err = svc.BulkSetPriority([]app.TaskLocator{repo.loc("t1")}, -1); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (invalid priority aborts before commit)", len(repo.commits))
	}
	if t1.Priority == 99 {
		t.Error("t1 priority mutated despite rejection")
	}
}

func TestBulkSetPriorityAcrossProjectsCommitsOneTransaction(t *testing.T) {
	t1 := newTask("t1", 1, "in a")
	t2 := newTask("t2", 1, "in b")
	projA := newProject("proj-a", "Alpha", t1)
	projB := newProject("proj-b", "Beta", t2)
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	res, err := svc.BulkSetPriority([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, 3)
	if err != nil {
		t.Fatalf("BulkSetPriority: %v", err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (both projects in one transaction)", len(repo.commits))
	}
	if got := len(repo.commits[0]); got != 2 {
		t.Errorf("changesets in the transaction = %d, want 2", got)
	}
	wantTouched := map[string]bool{"proj-a": true, "proj-b": true}
	for _, id := range res.TouchedProjectIDs {
		delete(wantTouched, id)
	}
	if len(wantTouched) != 0 {
		t.Errorf("TouchedProjectIDs = %v, want proj-a and proj-b", res.TouchedProjectIDs)
	}
}

func TestBulkSetDueDateSetsDue(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	if _, err := svc.BulkSetDueDate([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, due); err != nil {
		t.Fatalf("BulkSetDueDate: %v", err)
	}
	for _, task := range []*models.Task{t1, t2} {
		if task.DueDate == nil || task.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
			t.Errorf("task %s due = %v, want date %s", task.ID, task.DueDate, due.Format("2006-01-02"))
		}
	}
	// Each task holds its own copy of the due date, not a shared pointer.
	if t1.DueDate == t2.DueDate {
		t.Error("tasks share the same due-date pointer; each must hold its own copy")
	}
}

func TestBulkClearDueDateClearsAndIsNoOpWhenAbsent(t *testing.T) {
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	t1 := newTask("t1", 1, "has due")
	t1.DueDate = &due
	t2 := newTask("t2", 2, "no due")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.BulkClearDueDate([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")})
	if err != nil {
		t.Fatalf("BulkClearDueDate: %v", err)
	}
	for _, task := range []*models.Task{t1, t2} {
		if task.DueDate != nil {
			t.Errorf("task %s due = %v, want nil", task.ID, task.DueDate)
		}
	}
	if len(res.Tasks) != 2 {
		t.Errorf("result tasks = %d, want 2 (no-op clear still commits)", len(res.Tasks))
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestBulkDeferSetsDeferOnAll(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.BulkDefer([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, &until); err != nil {
		t.Fatalf("BulkDefer: %v", err)
	}
	for _, task := range []*models.Task{t1, t2} {
		if task.DeferredUntil == nil || !task.DeferredUntil.Equal(until) {
			t.Errorf("task %s deferred = %v, want %v", task.ID, task.DeferredUntil, until)
		}
		if task.DeferCount != 1 {
			t.Errorf("task %s defer count = %d, want 1", task.ID, task.DeferCount)
		}
	}
	if t1.DeferredUntil == t2.DeferredUntil {
		t.Error("tasks share the same deferral pointer; each must hold its own copy")
	}
}

func TestBulkDeferRejectsTerminalOrInProgress(t *testing.T) {
	t1 := newTask("t1", 1, "ready")
	t2 := newTask("t2", 2, "done")
	t2.ApplyWorkflowState(models.StateDone)
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.BulkDefer([]app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, &until)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (terminal task cannot be deferred)", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (batch aborts before commit)", len(repo.commits))
	}
	if t1.DeferredUntil != nil {
		t.Error("t1 deferred despite the batch aborting")
	}
}

func TestBulkDeferClearClearsDefer(t *testing.T) {
	deferred := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t1 := newTask("t1", 1, "deferred")
	t1.DeferredUntil = &deferred
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.BulkDefer([]app.TaskLocator{repo.loc("t1")}, nil); err != nil {
		t.Fatalf("BulkDefer clear: %v", err)
	}
	if t1.DeferredUntil != nil {
		t.Errorf("t1 deferred = %v, want nil after clear", t1.DeferredUntil)
	}
}

func TestBulkAttributeEmptyTaskIDsRejected(t *testing.T) {
	projA := newProject("proj-a", "Alpha")
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.BulkSetPriority(nil, 1); !errors.Is(err, app.ErrInvalidInput) {
		t.Errorf("BulkSetPriority(nil) err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.BulkClearDueDate(nil); !errors.Is(err, app.ErrInvalidInput) {
		t.Errorf("BulkClearDueDate(nil) err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0", len(repo.commits))
	}
}

func TestBulkAttributeUnresolvedTaskAborts(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	_, err := svc.BulkSetPriority([]app.TaskLocator{repo.loc("t1"), {ProjectID: "proj-a", TaskID: "missing"}}, 2)
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (unresolved task aborts the batch)", len(repo.commits))
	}
	if t1.Priority == 2 {
		t.Error("t1 mutated despite the batch aborting on an unresolved id")
	}
}
