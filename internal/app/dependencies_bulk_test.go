package app_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestReciprocalDependenciesAreAccepted proves the layer records a cycle without
// traversing the graph: a two-task loop is a legal informational statement.
func TestReciprocalDependenciesAreAccepted(t *testing.T) {
	repo, svc, source, sibling, _ := dependencyFixture()

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if _, err := svc.AddTaskDependencies(repo.loc(sibling.ID), []app.TaskLocator{repo.loc(source.ID)}); err != nil {
		t.Fatalf("reciprocal: %v", err)
	}
	assertDependencies(t, source, "proj-a:sibling")
	assertDependencies(t, sibling, "proj-a:source")
}

func TestRemoveTaskDependenciesAcceptsClosedAndUnloadedTargets(t *testing.T) {
	repo, svc, source, sibling, _ := dependencyFixture()

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A reference whose project the snapshot never loads still has to be
	// removable, so it is written directly the way a stale document would carry it.
	source.Dependencies = append(source.Dependencies, models.TaskReference{ProjectID: "proj-ghost", TaskID: "gone"})
	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc(sibling.ID), Target: models.StateDone}); err != nil {
		t.Fatalf("close target: %v", err)
	}

	if _, err := svc.RemoveTaskDependencies(repo.loc(source.ID), []app.TaskLocator{
		repo.loc(sibling.ID),
		{ProjectID: "proj-ghost", TaskID: "gone"},
	}); err != nil {
		t.Fatalf("RemoveTaskDependencies: %v", err)
	}
	assertDependencies(t, source)
}

func TestClearTaskDependenciesDropsEverySoftReference(t *testing.T) {
	repo, svc, source, sibling, remote := dependencyFixture()

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID), repo.loc(remote.ID)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := svc.ClearTaskDependencies(repo.loc(source.ID)); err != nil {
		t.Fatalf("ClearTaskDependencies: %v", err)
	}
	assertDependencies(t, source)
}

// TestDependencyPatchRollsBackOnCommitFailure proves the whole patch is resolved
// and applied inside the existing transaction: a persistence failure leaves the
// stored set and the touch time exactly as they were.
func TestDependencyPatchRollsBackOnCommitFailure(t *testing.T) {
	repo, _, source, sibling, remote := dependencyFixture()
	// An advancing clock makes a rolled-back touch visible: a second stamp would
	// differ from the one the seed persisted.
	svc := newTestServiceWithClock(repo, &advancingClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), step: time.Minute})

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stamp := source.UpdatedAt
	repo.commitErr = errors.New("disk on fire")

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(remote.ID)}); err == nil {
		t.Fatal("AddTaskDependencies succeeded despite a commit failure")
	}
	assertDependencies(t, source, "proj-a:sibling")
	if !source.UpdatedAt.Equal(stamp) {
		t.Fatalf("UpdatedAt = %v, want %v", source.UpdatedAt, stamp)
	}

	if _, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       repo.loc(source.ID),
		Title:        source.Title,
		Dependencies: app.DependencyPatch{Clear: true},
	}); err == nil {
		t.Fatal("EditTaskLine succeeded despite a commit failure")
	}
	assertDependencies(t, source, "proj-a:sibling")
}

// TestDependenciesDoNotAffectLifecycle proves the references are inert: a task
// carrying an open dependency still completes, and closing a target neither
// rewrites nor reopens the tasks pointing at it.
func TestDependenciesDoNotAffectLifecycle(t *testing.T) {
	repo, svc, source, sibling, remote := dependencyFixture()

	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID), repo.loc(remote.ID)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc(source.ID), Target: models.StateDone}); err != nil {
		t.Fatalf("completing a task with open dependencies was refused: %v", err)
	}
	if source.State() != models.StateDone {
		t.Fatalf("state = %q, want done", source.State())
	}
	assertDependencies(t, source, "proj-a:sibling", "proj-b:remote")

	if _, err := svc.Transition(app.StateRequest{Locator: repo.loc(remote.ID), Target: models.StateCanceled}); err != nil {
		t.Fatalf("cancel target: %v", err)
	}
	assertDependencies(t, source, "proj-a:sibling", "proj-b:remote")
	if source.State() != models.StateDone {
		t.Fatalf("closing a target changed the source state to %q", source.State())
	}
	assertDependencies(t, remote)
}

// TestSoftDependenciesIntroduceNoWorkflowAPI locks the shape of the restored
// concept: references are data, so no readiness predicate, blocked state, or
// graph API may appear alongside them.
func TestSoftDependenciesIntroduceNoWorkflowAPI(t *testing.T) {
	for _, method := range []string{
		"IsDependencyBlocked", "HasIncompleteDependencies", "GetIncompleteDependencies",
		"GetBlockedByTasks", "GetBlockingTasks", "HasCircularDependency",
		"WouldCreateDependencyCycle", "DependsOn", "HasDependency",
	} {
		if _, ok := reflect.TypeOf(&models.Task{}).MethodByName(method); ok {
			t.Errorf("live task exposes %s", method)
		}
		if _, ok := reflect.TypeOf(&app.Service{}).MethodByName(method); ok {
			t.Errorf("application service exposes %s", method)
		}
	}
	if _, ok := reflect.TypeOf(models.Task{}).FieldByName("Dependents"); ok {
		t.Error("task stores Dependents; it must be computed from loaded projects")
	}
}

// TestBulkSetDependenciesPatchesEachTargetIndependently is the tri-state bulk
// contract at the application boundary: an addition lands on every target, a
// removal is taken only where it was stored, and a reference the patch never
// names survives on whichever targets happen to carry it. That last one is why
// the bulk path is a patch rather than a replacement — N tasks carry N different
// stored sets, and only a patch can leave them disagreeing.
func TestBulkSetDependenciesPatchesEachTargetIndependently(t *testing.T) {
	source := newTask("source", 1, "Source")
	sibling := newTask("sibling", 2, "Sibling")
	remote := newTask("remote", 1, "Remote")
	extra := newTask("extra", 2, "Extra")
	other := newTask("other", 3, "Other")
	repo := newFakeRepo(
		newProject("proj-a", "Alpha", source, sibling),
		newProject("proj-b", "Beta", remote, extra, other),
	)
	svc := newTestService(repo)
	if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(remote.ID), repo.loc(other.ID)}); err != nil {
		t.Fatalf("seed the disagreement: %v", err)
	}

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetDependencies,
		Targets:   []app.TaskLocator{repo.loc(source.ID), repo.loc(sibling.ID)},
		Dependencies: app.DependencyPatch{
			Add:    []app.TaskLocator{repo.loc(extra.ID)},
			Remove: []app.TaskLocator{repo.loc(remote.ID)},
		},
	}); err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}

	// `other` is named by neither half, so it stays where it was and nowhere else;
	// `sibling` never stored the removal, so taking it is the same no-op the
	// single-task path gives it.
	assertDependencies(t, findTask(t, repo, "proj-a", source.ID), "proj-b:other", "proj-b:extra")
	assertDependencies(t, findTask(t, repo, "proj-a", sibling.ID), "proj-b:extra")
}

// TestBulkSetDependenciesRefusesATargetInsideTheSelection names the refusal. One
// selected task depending on another is a relationship inside the selection, and
// the gesture cannot say which direction was meant, so neither is guessed and
// nothing commits.
func TestBulkSetDependenciesRefusesATargetInsideTheSelection(t *testing.T) {
	repo, svc, source, sibling, remote := dependencyFixture()

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:    app.TaskOperationSetDependencies,
		Targets:      []app.TaskLocator{repo.loc(source.ID), repo.loc(sibling.ID)},
		Dependencies: app.DependencyPatch{Add: []app.TaskLocator{repo.loc(remote.ID), repo.loc(sibling.ID)}},
	})
	if err == nil {
		t.Fatal("a bulk addition naming a selected task committed")
	}
	if got := err.Error(); !strings.Contains(got, "#2 Sibling") {
		t.Errorf("refusal = %q, want the selected task named", got)
	}
	assertDependencies(t, findTask(t, repo, "proj-a", source.ID))
	assertDependencies(t, findTask(t, repo, "proj-a", sibling.ID))
}

// TestBulkSetDependenciesLeavesUntouchedTargetsAlone holds the touch time. A
// target the patch changed nothing on still commits, matching the other
// idempotent bulk mutations, but UpdatedAt answers "when did this task last
// change" and must not move for a request that changed nothing.
func TestBulkSetDependenciesLeavesUntouchedTargetsAlone(t *testing.T) {
	repo, svc, source, sibling, remote := dependencyFixture()
	if _, err := svc.AddTaskDependencies(repo.loc(sibling.ID), []app.TaskLocator{repo.loc(remote.ID)}); err != nil {
		t.Fatalf("seed the stored reference: %v", err)
	}
	untouched := findTask(t, repo, "proj-a", sibling.ID).UpdatedAt

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:    app.TaskOperationSetDependencies,
		Targets:      []app.TaskLocator{repo.loc(source.ID), repo.loc(sibling.ID)},
		Dependencies: app.DependencyPatch{Add: []app.TaskLocator{repo.loc(remote.ID)}},
	}); err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}
	assertDependencies(t, findTask(t, repo, "proj-a", source.ID), "proj-b:remote")
	assertDependencies(t, findTask(t, repo, "proj-a", sibling.ID), "proj-b:remote")
	if got := findTask(t, repo, "proj-a", sibling.ID).UpdatedAt; !got.Equal(untouched) {
		t.Errorf("an unchanged target moved its touch time to %v, want %v", got, untouched)
	}
}

// findTask reads one task back out of the seeded fixture by its stable ID.
func findTask(t *testing.T, repo *fakeRepo, projectID, taskID string) *models.Task {
	t.Helper()
	for _, project := range repo.projects {
		if project.ID != projectID {
			continue
		}
		for _, task := range project.Tasks {
			if task.ID == taskID {
				return task
			}
		}
	}
	t.Fatalf("no task %s in project %s", taskID, projectID)
	return nil
}
