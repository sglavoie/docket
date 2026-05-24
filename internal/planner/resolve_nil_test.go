package planner

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestResolveAllDependencies_NilProjectSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ResolveAllDependencies panicked on nil project: %v", r)
		}
	}()
	idx := ResolveAllDependencies([]*models.Project{nil})
	if len(idx) != 0 {
		t.Errorf("expected empty index for nil project, got %d entries", len(idx))
	}
}

func TestResolveAllDependencies_NilTaskSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ResolveAllDependencies panicked on nil task: %v", r)
		}
	}()
	good := newTask("t1")
	p := &models.Project{ID: "p1", Slug: "p1", Tasks: []*models.Task{nil, good, nil}}
	idx := ResolveAllDependencies([]*models.Project{p})
	if idx["t1"] != good {
		t.Errorf("expected good task to be indexed despite surrounding nil entries")
	}
}

func TestCollectAllIncompleteTasks_NilProjectSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CollectAllIncompleteTasks panicked: %v", r)
		}
	}()
	good := newTask("t1")
	p := &models.Project{ID: "p1", Slug: "p1", Tasks: []*models.Task{nil, good, nil}}
	result := CollectAllIncompleteTasks([]*models.Project{nil, p, nil})
	if len(result) != 1 || result[0] != good {
		t.Errorf("expected exactly the good incomplete task, got %d results", len(result))
	}
}
