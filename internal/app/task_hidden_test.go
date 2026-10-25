package app_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// TestSetHiddenWritesOnlyTheFlagAndTouchTime holds the orthogonality contract:
// hiding is one transaction over every target, and each changed target differs
// from its prior self in Hidden and UpdatedAt alone. The state, its clock, the
// pin, the review flag, the check-in, and the deadline are compared whole.
func TestSetHiddenWritesOnlyTheFlagAndTouchTime(t *testing.T) {
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	pinnedAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	working := newTask("working", 1, "Working")
	working.WorkflowState = models.StateWaiting
	working.DueDate = &due
	working.NextCheckIn = &due
	working.CheckInRepeatDays = 7
	working.Pinned, working.PinnedAt, working.Review = true, &pinnedAt, true
	finished := newTask("finished", 2, "Finished")
	finished.WorkflowState = models.StateDone
	elsewhere := newTask("elsewhere", 1, "Elsewhere")
	repo := newFakeRepo(newProject("proj-a", "Alpha", working, finished), newProject("proj-b", "Beta", elsewhere))
	svc := newTestService(repo)

	before := map[string]*models.Task{}
	for _, task := range []*models.Task{working, finished, elsewhere} {
		before[task.ID] = models.CloneTaskDeep(task)
	}
	result, err := svc.SetHidden([]app.TaskLocator{repo.loc("working"), repo.loc("finished"), repo.loc("elsewhere")}, true)
	if err != nil {
		t.Fatalf("SetHidden: %v", err)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 || result.TransactionID == "" {
		t.Fatalf("commits = %d (%v), want one transaction over both projects", len(repo.commits), result)
	}
	wantNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	for _, task := range []*models.Task{working, finished, elsewhere} {
		if !task.Hidden || !task.UpdatedAt.Equal(wantNow) {
			t.Fatalf("%s hidden=%t updated=%v, want hidden at %v", task.ID, task.Hidden, task.UpdatedAt, wantNow)
		}
		restored := models.CloneTaskDeep(task)
		restored.Hidden, restored.UpdatedAt = false, before[task.ID].UpdatedAt
		if !reflect.DeepEqual(restored, before[task.ID]) {
			t.Fatalf("%s changed beyond Hidden and UpdatedAt:\nbefore %+v\nafter  %+v", task.ID, before[task.ID], restored)
		}
	}

	// An already-correct member is a no-op, so its touch time stays put.
	stale := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	working.UpdatedAt = stale
	if _, err := svc.SetHidden([]app.TaskLocator{repo.loc("working")}, true); err != nil {
		t.Fatalf("SetHidden again: %v", err)
	}
	if !working.UpdatedAt.Equal(stale) {
		t.Fatalf("re-hiding moved UpdatedAt to %v", working.UpdatedAt)
	}
	if _, err := svc.SetHidden([]app.TaskLocator{repo.loc("working"), repo.loc("finished")}, false); err != nil {
		t.Fatalf("unhide: %v", err)
	}
	if working.Hidden || finished.Hidden || !elsewhere.Hidden {
		t.Fatalf("unhide reached the wrong tasks: %t/%t/%t", working.Hidden, finished.Hidden, elsewhere.Hidden)
	}
}

func TestSetHiddenStoresTheFlagOnTheTargetAlone(t *testing.T) {
	parent := newTask("parent", 1, "Parent")
	child := childOf("child", 2, "Child", parent)
	project := newProject("proj-a", "Alpha", parent, child)
	repo := newFakeRepo(project)
	if _, err := newTestService(repo).SetHidden([]app.TaskLocator{repo.loc("parent")}, true); err != nil {
		t.Fatalf("SetHidden: %v", err)
	}
	if child.Hidden {
		t.Fatal("hiding a parent stored the flag on its child")
	}
	if !models.TaskEffectivelyHidden(project, child) {
		t.Fatal("the child of a hidden parent is not effectively hidden")
	}
}

func TestToggleHiddenHidesUntilAllHiddenThenUnhides(t *testing.T) {
	hidden := newTask("hidden", 1, "Hidden")
	hidden.Hidden = true
	visible := newTask("visible", 2, "Visible")
	repo := newFakeRepo(newProject("proj-a", "Alpha", hidden, visible))
	svc := newTestService(repo)
	toggle := func() {
		t.Helper()
		if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
			Operation: app.TaskOperationToggleHidden,
			Targets:   []app.TaskLocator{repo.loc("hidden"), repo.loc("visible")},
		}); err != nil {
			t.Fatalf("toggle: %v", err)
		}
	}
	toggle()
	if !hidden.Hidden || !visible.Hidden {
		t.Fatalf("mixed selection = %t/%t, want both hidden", hidden.Hidden, visible.Hidden)
	}
	toggle()
	if hidden.Hidden || visible.Hidden {
		t.Fatalf("all-hidden selection = %t/%t, want both unhidden", hidden.Hidden, visible.Hidden)
	}
	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetHidden, Targets: []app.TaskLocator{repo.loc("visible")}, Hidden: true,
	}); err != nil || !visible.Hidden || hidden.Hidden {
		t.Fatalf("set operation err=%v hidden=%t/%t", err, hidden.Hidden, visible.Hidden)
	}
}

func TestSetHiddenRefusesUnresolvableTargetsBeforeMutation(t *testing.T) {
	task := newTask("t1", 1, "first")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	svc := newTestService(repo)
	if _, err := svc.SetHidden(nil, true); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty targets err = %v", err)
	}
	if _, err := svc.SetHidden([]app.TaskLocator{repo.loc("t1"), {ProjectID: "proj-a", TaskID: "missing"}}, true); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("missing target err = %v", err)
	}
	if task.Hidden || len(repo.commits) != 0 {
		t.Fatalf("refused batch mutated: hidden=%t commits=%d", task.Hidden, len(repo.commits))
	}
}

// TestSetHiddenSurvivesReloadAndIsOneUndoEntry drives the flag through durable
// storage: one hide is one ledger entry, undo restores the visible task, and
// redo hides it again.
func TestSetHiddenSurvivesReloadAndIsOneUndoEntry(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)
	project := realTestProject(t, dataDir, "alpha", "Quiet")
	task := project.Tasks[0]
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	service := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	if _, err := service.SetHidden([]app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}}, true); err != nil {
		t.Fatalf("SetHidden: %v", err)
	}
	if !reloadedTaskLineEditTask(t, project, task.ID).Hidden {
		t.Fatal("the hidden flag did not survive a reload")
	}
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if reloadedTaskLineEditTask(t, project, task.ID).Hidden {
		t.Fatal("undo left the task hidden")
	}
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if !reloadedTaskLineEditTask(t, project, task.ID).Hidden {
		t.Fatal("redo did not hide the task again")
	}
}
