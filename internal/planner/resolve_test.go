package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func makeProject(id, slug string, tasks ...*models.Task) *models.Project {
	return &models.Project{
		ID:    id,
		Slug:  slug,
		Tasks: tasks,
	}
}

func TestResolveAllDependencies_PlainID(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "myproject", task)

	idx := ResolveAllDependencies([]*models.Project{p})

	if idx["t1"] == nil {
		t.Error("plain task ID should be in index")
	}
}

func TestResolveAllDependencies_QualifiedByProjectID(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "myproject", task)

	idx := ResolveAllDependencies([]*models.Project{p})

	if idx["proj1:t1"] == nil {
		t.Error("project-ID-qualified ref should be in index")
	}
}

func TestResolveAllDependencies_QualifiedBySlug(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "myproject", task)

	idx := ResolveAllDependencies([]*models.Project{p})

	if idx["myproject:t1"] == nil {
		t.Error("slug-qualified ref should be in index")
	}
}

func TestResolveAllDependencies_MissingRef(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "myproject", task)

	idx := ResolveAllDependencies([]*models.Project{p})

	if idx["nonexistent"] != nil {
		t.Error("missing ref should not be in index")
	}
}

func TestResolveAllDependencies_CrossProject(t *testing.T) {
	taskA := newTask("ta")
	taskB := newTask("tb")
	pA := makeProject("projA", "alpha", taskA)
	pB := makeProject("projB", "beta", taskB)

	idx := ResolveAllDependencies([]*models.Project{pA, pB})

	if idx["projA:ta"] == nil || idx["projB:tb"] == nil {
		t.Error("cross-project tasks should be indexed by project ID")
	}
	if idx["alpha:ta"] == nil || idx["beta:tb"] == nil {
		t.Error("cross-project tasks should be indexed by project slug")
	}
}

func TestResolveAllDependencies_OmitsAmbiguousPlainID(t *testing.T) {
	sharedA := newTask("shared")
	sharedB := newTask("shared")
	pA := makeProject("projA", "alpha", sharedA)
	pB := makeProject("projB", "beta", sharedB)

	idx := ResolveAllDependencies([]*models.Project{pA, pB})
	if got := LookupTask("shared", idx); got != nil {
		t.Fatalf("ambiguous plain ID resolved to %p", got)
	}
	if idx["projA:shared"] != sharedA || idx["projB:shared"] != sharedB {
		t.Fatal("qualified duplicate task lookups were not preserved")
	}
}

func TestCollectAllIncompleteTasks_ExcludesComplete(t *testing.T) {
	todo := newTask("todo")
	done := withStatus(newTask("done"), models.StateDone)
	p := makeProject("proj1", "p1", todo, done)

	result := CollectAllIncompleteTasks([]*models.Project{p})

	if len(result) != 1 || result[0].ID != "todo" {
		t.Errorf("expected only incomplete tasks, got %d tasks", len(result))
	}
}

func TestCollectAllIncompleteTasks_MultiProject(t *testing.T) {
	t1 := newTask("t1")
	t2 := newTask("t2")
	p1 := makeProject("p1", "proj1", t1)
	p2 := makeProject("p2", "proj2", t2)

	result := CollectAllIncompleteTasks([]*models.Project{p1, p2})

	if len(result) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(result))
	}
}

func TestCollectAllIncompleteTasks_AttachesProjectForPlanning(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "alpha", task)

	result := CollectAllIncompleteTasks([]*models.Project{p})

	if len(result) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result))
	}
	if result[0].Project != p {
		t.Fatal("expected collected task to carry its project pointer")
	}
}

func TestLookupTask_PlainRef(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "p", task)
	idx := ResolveAllDependencies([]*models.Project{p})

	found := LookupTask("t1", idx)
	if found == nil || found.ID != "t1" {
		t.Error("LookupTask should find plain ref")
	}
}

func TestLookupTask_MissingRef(t *testing.T) {
	task := newTask("t1")
	p := makeProject("proj1", "p", task)
	idx := ResolveAllDependencies([]*models.Project{p})

	found := LookupTask("nonexistent", idx)
	if found != nil {
		t.Error("LookupTask should return nil for missing ref")
	}
}

// TestLookupTask_QualifiedRefNoBleedThrough guards against a bug where a
// qualified cross-project ref like "projX:foo" that did not resolve via its
// full key would silently fall back to a plain-ID lookup of "foo" and pick up
// an unrelated task in a different project that happens to share that ID.
func TestLookupTask_QualifiedRefNoBleedThrough(t *testing.T) {
	// projA owns task "foo"; projX is referenced by the user but does not
	// exist in the index.
	taskFoo := newTask("foo")
	pA := makeProject("projA", "alpha", taskFoo)
	idx := ResolveAllDependencies([]*models.Project{pA})

	// "projX:foo" must not resolve to projA's "foo".
	if got := LookupTask("projX:foo", idx); got != nil {
		t.Errorf("expected nil for unresolved qualified ref, got task with ID %q", got.ID)
	}
	// Sanity: the unqualified "foo" still resolves.
	if got := LookupTask("foo", idx); got == nil {
		t.Error("expected unqualified ref to still resolve")
	}
	// Sanity: a correctly qualified ref still resolves.
	if got := LookupTask("projA:foo", idx); got == nil {
		t.Error("expected correctly qualified ref to resolve")
	}
}
