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

	res, err := svc.BulkSetPriority([]string{"t1", "t2"}, 2)
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
}

func TestBulkSetPriorityRejectsInvalidPriority(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	_, err := svc.BulkSetPriority([]string{"t1"}, 99)
	if !errors.Is(err, app.ErrInvalidInput) {
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

	res, err := svc.BulkSetPriority([]string{"t1", "t2"}, 3)
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

func TestBulkSetDueDateSetsDueAndTime(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	due := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	if _, err := svc.BulkSetDueDate([]string{"t1", "t2"}, due, true); err != nil {
		t.Fatalf("BulkSetDueDate: %v", err)
	}
	for _, task := range []*models.Task{t1, t2} {
		if task.DueDate == nil || !task.DueDate.Equal(due) {
			t.Errorf("task %s due = %v, want %v", task.ID, task.DueDate, due)
		}
		if !task.HasTime {
			t.Errorf("task %s HasTime = false, want true", task.ID)
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
	t1.HasTime = true
	t2 := newTask("t2", 2, "no due")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.BulkClearDueDate([]string{"t1", "t2"})
	if err != nil {
		t.Fatalf("BulkClearDueDate: %v", err)
	}
	for _, task := range []*models.Task{t1, t2} {
		if task.DueDate != nil {
			t.Errorf("task %s due = %v, want nil", task.ID, task.DueDate)
		}
		if task.HasTime {
			t.Errorf("task %s HasTime = true, want false", task.ID)
		}
	}
	if len(res.Tasks) != 2 {
		t.Errorf("result tasks = %d, want 2 (no-op clear still commits)", len(res.Tasks))
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestBulkSnoozeSetsSnoozeOnAll(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.BulkSnooze([]string{"t1", "t2"}, &until); err != nil {
		t.Fatalf("BulkSnooze: %v", err)
	}
	for _, task := range []*models.Task{t1, t2} {
		if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(until) {
			t.Errorf("task %s snoozed = %v, want %v", task.ID, task.SnoozedUntil, until)
		}
	}
	if t1.SnoozedUntil == t2.SnoozedUntil {
		t.Error("tasks share the same snooze pointer; each must hold its own copy")
	}
}

func TestBulkSnoozeRejectsTerminalOrInProgress(t *testing.T) {
	t1 := newTask("t1", 1, "ready")
	t2 := newTask("t2", 2, "done")
	t2.ApplyWorkflowState(models.StateDone)
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	until := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.BulkSnooze([]string{"t1", "t2"}, &until)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (terminal task cannot be snoozed)", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (batch aborts before commit)", len(repo.commits))
	}
	if t1.SnoozedUntil != nil {
		t.Error("t1 snoozed despite the batch aborting")
	}
}

func TestBulkSnoozeClearClearsSnooze(t *testing.T) {
	snoozed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t1 := newTask("t1", 1, "snoozed")
	t1.SnoozedUntil = &snoozed
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.BulkSnooze([]string{"t1"}, nil); err != nil {
		t.Fatalf("BulkSnooze clear: %v", err)
	}
	if t1.SnoozedUntil != nil {
		t.Errorf("t1 snoozed = %v, want nil after clear", t1.SnoozedUntil)
	}
}

func TestBulkSetTagsReplacesTagSet(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t1.Tags = []string{"old", "stale"}
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.BulkSetTags([]string{"t1", "t2"}, []string{"#backend", "urgent", "backend"}); err != nil {
		t.Fatalf("BulkSetTags: %v", err)
	}
	// "#backend" normalizes to "backend"; the duplicate "backend" is dropped.
	want := []string{"backend", "urgent"}
	for _, task := range []*models.Task{t1, t2} {
		if len(task.Tags) != len(want) {
			t.Fatalf("task %s tags = %v, want %v", task.ID, task.Tags, want)
		}
		for i, tag := range want {
			if task.Tags[i] != tag {
				t.Errorf("task %s tag[%d] = %q, want %q", task.ID, i, task.Tags[i], tag)
			}
		}
	}
	// Each task holds its own tag slice, not a shared backing array.
	t1.Tags[0] = "mutated"
	if t2.Tags[0] == "mutated" {
		t.Error("tasks share the same tag backing slice")
	}
}

func TestBulkSetTagsEmptyClearsTags(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t1.Tags = []string{"keep", "me"}
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.BulkSetTags([]string{"t1"}, nil); err != nil {
		t.Fatalf("BulkSetTags empty: %v", err)
	}
	if len(t1.Tags) != 0 {
		t.Errorf("t1 tags = %v, want empty", t1.Tags)
	}
}

func TestBulkSetTagsRejectsInvalidTag(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	_, err := svc.BulkSetTags([]string{"t1"}, []string{"ok", "bad,tag"})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (comma is invalid in a tag)", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (invalid tag aborts before commit)", len(repo.commits))
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

	_, err := svc.BulkSetPriority([]string{"t1", "missing"}, 2)
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
