package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestTopologicalLinearChain verifies A -> B -> C order.
func TestTopologicalLinearChain(t *testing.T) {
	a := newTask("a")
	b := withDep(newTask("b"), "a")
	c := withDep(newTask("c"), "b")

	order, err := TopologicalSort([]*models.Task{c, b, a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] || pos["b"] >= pos["c"] {
		t.Errorf("order violation: a=%d b=%d c=%d", pos["a"], pos["b"], pos["c"])
	}
}

// TestTopologicalDiamond verifies a diamond dependency: A, B->A, C->A, D->B,D->C.
func TestTopologicalDiamond(t *testing.T) {
	a := newTask("a")
	b := withDep(newTask("b"), "a")
	c := withDep(newTask("c"), "a")
	d := withDep(newTask("d"), "b", "c")

	order, err := TopologicalSort([]*models.Task{d, c, b, a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] {
		t.Errorf("a must come before b")
	}
	if pos["a"] >= pos["c"] {
		t.Errorf("a must come before c")
	}
	if pos["b"] >= pos["d"] {
		t.Errorf("b must come before d")
	}
	if pos["c"] >= pos["d"] {
		t.Errorf("c must come before d")
	}
}

// TestTopologicalNoDeps verifies tasks without dependencies are all returned.
func TestTopologicalNoDeps(t *testing.T) {
	tasks := []*models.Task{newTask("x"), newTask("y"), newTask("z")}
	order, err := TopologicalSort(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
}

// TestTopologicalSingleNode verifies a single task returns correctly.
func TestTopologicalSingleNode(t *testing.T) {
	order, err := TopologicalSort([]*models.Task{newTask("solo")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0].ID != "solo" {
		t.Errorf("expected [solo], got %v", taskIDs(order))
	}
}

// TestTopologicalMixedCompleteIncomplete verifies complete tasks are excluded.
func TestTopologicalMixedCompleteIncomplete(t *testing.T) {
	done := withStatus(newTask("done"), models.StatusDone)
	todo := withDep(newTask("todo"), "done") // depends on a complete task

	order, err := TopologicalSort([]*models.Task{done, todo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0].ID != "todo" {
		t.Errorf("expected only [todo], got %v", taskIDs(order))
	}
}

// TestTopologicalDisconnectedComponents verifies multiple independent trees.
func TestTopologicalDisconnectedComponents(t *testing.T) {
	// Component 1: p -> q
	p := newTask("p")
	q := withDep(newTask("q"), "p")
	// Component 2: r -> s
	r := newTask("r")
	s := withDep(newTask("s"), "r")

	order, err := TopologicalSort([]*models.Task{s, q, r, p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["p"] >= pos["q"] {
		t.Errorf("p must come before q")
	}
	if pos["r"] >= pos["s"] {
		t.Errorf("r must come before s")
	}
}

// TestTopologicalCrossProjectDependency verifies cross-project refs are
// treated as satisfied (not in the slice) and don't block ordering.
func TestTopologicalCrossProjectDependency(t *testing.T) {
	// Task B depends on a task in project "proj-a" (not in this slice).
	b := withDep(newTask("b"), "proj-a:external-task")
	c := withDep(newTask("c"), "b")

	order, err := TopologicalSort([]*models.Task{c, b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["b"] >= pos["c"] {
		t.Errorf("b must come before c")
	}
}

func TestTopologicalQualifiedRefResolvesIncludedProjectTask(t *testing.T) {
	pA := makeProject("proj-a", "alpha")
	pB := makeProject("proj-b", "beta")
	a := newTask("a")
	a.Project = pA
	b := withDep(newTask("b"), "proj-a:a")
	b.Project = pB

	order, err := TopologicalSort([]*models.Task{b, a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] {
		t.Errorf("qualified proj-a:a should force a before b, got %v", taskIDs(order))
	}
}

func TestTopologicalQualifiedRefResolvesIncludedProjectTaskBySlug(t *testing.T) {
	pA := makeProject("proj-a", "alpha")
	pB := makeProject("proj-b", "beta")
	a := newTask("a")
	a.Project = pA
	b := withDep(newTask("b"), "alpha:a")
	b.Project = pB

	order, err := TopologicalSort([]*models.Task{b, a})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] {
		t.Errorf("qualified alpha:a should force a before b, got %v", taskIDs(order))
	}
}

func TestTopologicalQualifiedRefDoesNotBleedToPlainID(t *testing.T) {
	local := newTask("z")
	dependent := withDep(newTask("b"), "missing-project:z")

	order, err := TopologicalSort([]*models.Task{local, dependent})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := taskPositions(order)
	if pos["b"] > pos["z"] {
		t.Errorf("qualified missing-project:z should not force local z before b, got %v", taskIDs(order))
	}
}

// TestTopologicalCrossProjectChainResolved verifies that when cross-project
// tasks ARE included in the slice (via CollectAllIncompleteTasks), ordering
// is correct across the unified flat slice.
func TestTopologicalCrossProjectChainResolved(t *testing.T) {
	// Task in project B depends on task in project A.
	taskA := newTask("task-a")
	taskB := withDep(newTask("task-b"), "task-a") // same-project lookup in unified slice

	order, err := TopologicalSort([]*models.Task{taskB, taskA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := taskPositions(order)
	if pos["task-a"] >= pos["task-b"] {
		t.Errorf("task-a must come before task-b")
	}
}

// TestTopologicalSortSkipsNilAndEmptyID guards against nil entries or tasks
// with empty IDs (e.g., loosely-assembled cross-project slices) — previously
// these caused a nil-deref or polluted taskMap with the empty key.
func TestTopologicalSortSkipsNilAndEmptyID(t *testing.T) {
	a := newTask("a")
	b := withDep(newTask("b"), "a")
	noID := newTask("")
	noID.ID = "" // explicit; newTask sets it but defensive
	order, err := TopologicalSort([]*models.Task{nil, a, noID, b, nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 valid tasks, got %d", len(order))
	}
	pos := taskPositions(order)
	if pos["a"] >= pos["b"] {
		t.Errorf("expected a before b")
	}
}
