package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TestDependencyIsolationLeavesLifecycleUntouched is the positive half of the
// isolation contract that internal/app/dependency_isolation_test.go states
// negatively. That fence proves no identifier can express hard blocking; this
// proves the live product does not express it some other way either.
//
// Every assertion here is about something a hard dependency would have changed
// and a soft reference must not: whether a task may complete, whether its target
// completing rewrites it, and whether the graph shape is validated at all.
func TestDependencyIsolationLeavesLifecycleUntouched(t *testing.T) {
	t.Run("an open dependency never refuses completion", func(t *testing.T) {
		repo, svc, source, sibling, _ := dependencyFixture()
		if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
			t.Fatalf("AddTaskDependencies: %v", err)
		}
		if got := findTask(t, repo, "proj-a", sibling.ID).State(); got.IsTerminal() {
			t.Fatalf("fixture target state = %q, want an open target", got)
		}

		if _, err := svc.Transition(app.StateRequest{
			Locator: repo.loc(source.ID),
			Target:  models.StateDone,
		}); err != nil {
			t.Fatalf("completing a task with an open dependency was refused: %v", err)
		}
		completed := findTask(t, repo, "proj-a", source.ID)
		if completed.State() != models.StateDone {
			t.Fatalf("state = %q, want done", completed.State())
		}
		assertDependencies(t, completed, "proj-a:sibling")
	})

	t.Run("a target going terminal preserves the reference", func(t *testing.T) {
		repo, svc, source, sibling, _ := dependencyFixture()
		if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
			t.Fatalf("AddTaskDependencies: %v", err)
		}
		if _, err := svc.Transition(app.StateRequest{
			Locator: repo.loc(sibling.ID),
			Target:  models.StateCanceled,
		}); err != nil {
			t.Fatalf("Transition target: %v", err)
		}

		// Nothing prunes: a reference to closed work is still what the user wrote,
		// and a pruning pass would silently edit a set the user never touched.
		assertDependencies(t, findTask(t, repo, "proj-a", source.ID), "proj-a:sibling")
		if got := findTask(t, repo, "proj-a", source.ID).State(); got.IsTerminal() {
			t.Fatalf("subject state = %q, want the target's transition to leave it alone", got)
		}
	})

	t.Run("a reciprocal reference is legal", func(t *testing.T) {
		repo, svc, source, sibling, _ := dependencyFixture()
		if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
			t.Fatalf("AddTaskDependencies forward: %v", err)
		}
		// A DAG validator would refuse this edge. There is none, because nothing
		// walks the graph: the only question an addition asks is about the one
		// task it names.
		if _, err := svc.AddTaskDependencies(repo.loc(sibling.ID), []app.TaskLocator{repo.loc(source.ID)}); err != nil {
			t.Fatalf("a reciprocal reference was refused: %v", err)
		}
		assertDependencies(t, findTask(t, repo, "proj-a", source.ID), "proj-a:sibling")
		assertDependencies(t, findTask(t, repo, "proj-a", sibling.ID), "proj-a:source")

		// And both ends still complete, in either order.
		if _, err := svc.Transition(app.StateRequest{Locator: repo.loc(source.ID), Target: models.StateDone}); err != nil {
			t.Fatalf("completing one end of a cycle: %v", err)
		}
		if _, err := svc.Transition(app.StateRequest{Locator: repo.loc(sibling.ID), Target: models.StateDone}); err != nil {
			t.Fatalf("completing the other end of a cycle: %v", err)
		}
	})

	t.Run("dependents are computed and never stored", func(t *testing.T) {
		repo, svc, source, sibling, _ := dependencyFixture()
		if _, err := svc.AddTaskDependencies(repo.loc(source.ID), []app.TaskLocator{repo.loc(sibling.ID)}); err != nil {
			t.Fatalf("AddTaskDependencies: %v", err)
		}
		// The referenced task's own document gains nothing: an incoming reference
		// exists only in the other task's outgoing list, which is what keeps the
		// relation single-writer and undo-able as one change.
		assertDependencies(t, findTask(t, repo, "proj-a", sibling.ID))
	})
}
