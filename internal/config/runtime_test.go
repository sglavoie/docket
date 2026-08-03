package config

import (
	"path/filepath"
	"testing"
)

// fakeDomainStore verifies that RuntimeDependencies reads NextOrder through the
// injected Planning dependency.
type fakeDomainStore struct {
	DomainStore
	planning  *PlanningState
	loadCalls int
}

func (f *fakeDomainStore) LoadPlanning() (*PlanningState, error) {
	f.loadCalls++
	return f.planning, nil
}

func TestNewRuntimeDependenciesWiresEveryDomainToStore(t *testing.T) {
	store := newIsolatedStore(t)
	deps := NewRuntimeDependencies(store)
	if deps == nil {
		t.Fatal("NewRuntimeDependencies returned nil for a non-nil store")
	}
	if deps.Catalog == nil || deps.Planning == nil {
		t.Fatalf("every domain must be wired: %+v", deps)
	}
	if NewRuntimeDependencies(nil) != nil {
		t.Fatal("NewRuntimeDependencies(nil) must be nil")
	}
}

func TestRuntimeDependenciesNextOrderReadsPlanningDependency(t *testing.T) {
	fake := &fakeDomainStore{planning: &PlanningState{
		Next: []ProjectTaskLocator{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
		},
	}}
	deps := NewRuntimeDependencies(fake)

	order, err := deps.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 Next-order items, got %d", len(order))
	}
	if fake.loadCalls == 0 {
		t.Fatal("NextOrder must load through the Planning dependency")
	}

	locator := ProjectTaskLocator{TaskID: "t2", ProjectID: "p1"}
	if in, err := deps.TaskInNext(locator); err != nil || !in {
		t.Fatalf("TaskInNext(%+v) = %v, %v", locator, in, err)
	}
	for _, absent := range []ProjectTaskLocator{
		{TaskID: "missing", ProjectID: "p1"},
		{TaskID: "t2", ProjectID: "p2"},
		{},
	} {
		if in, err := deps.TaskInNext(absent); err != nil || in {
			t.Fatalf("TaskInNext(%+v) = %v, %v; want false, nil", absent, in, err)
		}
	}
}

func TestManagerRuntimeDependenciesRoundTripsNextOrder(t *testing.T) {
	store := newIsolatedStore(t)
	planning := NewPlanningState()
	planning.AddToNext("next-1", "proj-1")
	if err := store.SavePlanning(planning); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}

	deps := NewManager(filepath.Join(store.DataDir(), ".tt.json")).RuntimeDependencies()
	if deps == nil {
		t.Fatal("Manager.RuntimeDependencies returned nil")
	}
	locator := ProjectTaskLocator{TaskID: "next-1", ProjectID: "proj-1"}
	if in, err := deps.TaskInNext(locator); err != nil || !in {
		t.Fatalf("TaskInNext(%+v) = %v, %v", locator, in, err)
	}
}
