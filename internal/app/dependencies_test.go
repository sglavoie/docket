package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// completedTask builds a done leaf task so a cross-project dependency can resolve
// to a genuinely complete blocker.
func completedTask(id string, shortID int, title string) *models.Task {
	t := newTask(id, shortID, title)
	t.Complete()
	return t
}

func TestCompleteResolvesCompletedCrossProjectDependency(t *testing.T) {
	blocker := completedTask("b1", 1, "shared blocker")
	dependent := newTask("a1", 1, "waits on beta")
	dependent.DependsOn = []string{"proj-b:b1"}

	projA := newProject("proj-a", "Alpha", dependent)
	projB := newProject("proj-b", "Beta", blocker)
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	// The cross-project blocker is already done, so the workspace resolver must
	// allow completion — unlike the per-project helper, which treats every
	// cross-project reference as unresolved.
	res, err := svc.Complete("a1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.State != models.StateDone {
		t.Errorf("State = %q, want done", res.State)
	}
}

func TestCompleteBlockedByIncompleteCrossProjectDependency(t *testing.T) {
	blocker := newTask("b1", 1, "still open")
	dependent := newTask("a1", 1, "waits on beta")
	dependent.DependsOn = []string{"proj-b:b1"}

	projA := newProject("proj-a", "Alpha", dependent)
	projB := newProject("proj-b", "Beta", blocker)
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	_, err := svc.Complete("a1")
	if !errors.Is(err, app.ErrDependencyBlocked) {
		t.Fatalf("err = %v, want ErrDependencyBlocked", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("blocked completion committed %d times, want 0", len(repo.commits))
	}
}

func TestCompleteBlockedByUnresolvableCrossProjectDependency(t *testing.T) {
	dependent := newTask("a1", 1, "waits on missing project")
	dependent.DependsOn = []string{"proj-gone:x1"}

	projA := newProject("proj-a", "Alpha", dependent)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// A cross-project reference whose project is not loaded stays blocking
	// (fail-closed), so an unavailable dependency never silently unblocks.
	_, err := svc.Complete("a1")
	if !errors.Is(err, app.ErrDependencyBlocked) {
		t.Fatalf("err = %v, want ErrDependencyBlocked", err)
	}
}

func TestAddDependencySameProject(t *testing.T) {
	a1 := newTask("a1", 1, "task one")
	a2 := newTask("a2", 2, "task two")
	projA := newProject("proj-a", "Alpha", a1, a2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.AddDependency("a1", "a2"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if !a1.HasDependency("a2") {
		t.Errorf("DependsOn = %v, want a2", a1.DependsOn)
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatalf("want exactly one changeset in one commit, got %v", repo.commits)
	}
	if cs.UndoAction != "Add Dependency" {
		t.Errorf("UndoAction = %q, want Add Dependency", cs.UndoAction)
	}
}

func TestAddDependencyCrossProject(t *testing.T) {
	a1 := newTask("a1", 1, "alpha task")
	b1 := newTask("b1", 1, "beta task")
	projA := newProject("proj-a", "Alpha", a1)
	projB := newProject("proj-b", "Beta", b1)
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	if _, err := svc.AddDependency("a1", "proj-b:b1"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if !a1.HasDependency("proj-b:b1") {
		t.Errorf("DependsOn = %v, want proj-b:b1 preserved qualified", a1.DependsOn)
	}
}

func TestAddDependencyRejectsSelf(t *testing.T) {
	a1 := newTask("a1", 1, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", a1))
	svc := newTestService(repo)

	if _, err := svc.AddDependency("a1", "a1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestAddDependencyUnknownBlocker(t *testing.T) {
	a1 := newTask("a1", 1, "task")
	repo := newFakeRepo(newProject("proj-a", "Alpha", a1))
	svc := newTestService(repo)

	if _, err := svc.AddDependency("a1", "ghost"); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestAddDependencyRejectsSameProjectCycle(t *testing.T) {
	a1 := newTask("a1", 1, "one")
	a2 := newTask("a2", 2, "two")
	a1.DependsOn = []string{"a2"} // a1 already depends on a2
	projA := newProject("proj-a", "Alpha", a1, a2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// Adding a2 -> a1 would close the cycle a1 -> a2 -> a1.
	if _, err := svc.AddDependency("a2", "a1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (cycle)", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("rejected cyclic dependency committed %d times, want 0", len(repo.commits))
	}
}

func TestAddDependencyRejectsCrossProjectCycle(t *testing.T) {
	a1 := newTask("a1", 1, "alpha")
	b1 := newTask("b1", 1, "beta")
	a1.DependsOn = []string{"proj-b:b1"} // alpha already depends on beta
	projA := newProject("proj-a", "Alpha", a1)
	projB := newProject("proj-b", "Beta", b1)
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	// Adding beta -> alpha closes a cycle that spans both projects.
	if _, err := svc.AddDependency("b1", "proj-a:a1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (cross-project cycle)", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	a1 := newTask("a1", 1, "one")
	a2 := newTask("a2", 2, "two")
	a1.DependsOn = []string{"a2"}
	projA := newProject("proj-a", "Alpha", a1, a2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.RemoveDependency("a1", "a2"); err != nil {
		t.Fatalf("RemoveDependency: %v", err)
	}
	if a1.HasDependency("a2") {
		t.Errorf("DependsOn = %v, want a2 removed", a1.DependsOn)
	}
}

func TestClearDependencies(t *testing.T) {
	// Dependency blocking is a derived predicate, not a workflow state:
	// clearing every dependency makes the task derived-unblocked while the use
	// case preserves the task's workflow state (no reopen/transition).
	a1 := newTask("a1", 1, "one")
	a2 := newTask("a2", 2, "two")
	a3 := newTask("a3", 3, "three")
	a1.DependsOn = []string{"a2", "a3"}
	a1.WorkflowState = models.StateWaiting
	projA := newProject("proj-a", "Alpha", a1, a2, a3)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ClearDependencies("a1"); err != nil {
		t.Fatalf("ClearDependencies: %v", err)
	}
	if len(a1.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty", a1.DependsOn)
	}
	if projA.HasIncompleteDependencies(a1) {
		t.Errorf("expected task derived-unblocked after clearing dependencies")
	}
	if a1.State() != models.StateWaiting {
		t.Errorf("State = %s, want workflow state preserved (blocked)", a1.State())
	}
}

func TestSetDependencies(t *testing.T) {
	a1 := newTask("a1", 1, "one")
	a2 := newTask("a2", 2, "two")
	a3 := newTask("a3", 3, "three")
	a1.DependsOn = []string{"a2"}
	projA := newProject("proj-a", "Alpha", a1, a2, a3)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// Replace the whole set: drop a2, add a3.
	if _, err := svc.SetDependencies("a1", []string{"a3"}); err != nil {
		t.Fatalf("SetDependencies: %v", err)
	}
	if a1.HasDependency("a2") {
		t.Errorf("DependsOn = %v, want a2 dropped", a1.DependsOn)
	}
	if !a1.HasDependency("a3") {
		t.Errorf("DependsOn = %v, want a3 present", a1.DependsOn)
	}

	// An empty set clears every dependency.
	if _, err := svc.SetDependencies("a1", nil); err != nil {
		t.Fatalf("SetDependencies(clear): %v", err)
	}
	if len(a1.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty after clearing", a1.DependsOn)
	}
}

func TestAddAndRemoveRelated(t *testing.T) {
	a1 := newTask("a1", 1, "one")
	a2 := newTask("a2", 2, "two")
	projA := newProject("proj-a", "Alpha", a1, a2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.AddRelated("a1", "a2"); err != nil {
		t.Fatalf("AddRelated: %v", err)
	}
	if !a1.HasRelatedTo("a2") {
		t.Errorf("RelatedTo = %v, want a2", a1.RelatedTo)
	}
	if _, err := svc.RemoveRelated("a1", "a2"); err != nil {
		t.Fatalf("RemoveRelated: %v", err)
	}
	if a1.HasRelatedTo("a2") {
		t.Errorf("RelatedTo = %v, want a2 removed", a1.RelatedTo)
	}
}
