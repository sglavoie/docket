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

func TestCollectAllIncompleteTasks_ExcludesComplete(t *testing.T) {
	todo := newTask("todo")
	done := withStatus(newTask("done"), models.StatusDone)
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
