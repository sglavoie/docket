package config

import (
	"path/filepath"
	"testing"
)

// fakeDomainStore is a minimal DomainStore that only serves a fixed planning
// aggregate; it verifies that RuntimeDependencies reads Focus Queue through the
// injected Planning dependency rather than any config.Config facade.
type fakeDomainStore struct {
	DomainStore // embed for the unused domains; nil is fine since they are never called
	planning    *PlanningState
	loadCalls   int
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
	if deps.Catalog == nil || deps.Planning == nil || deps.Settings == nil ||
		deps.Automation == nil || deps.Machine == nil {
		t.Fatalf("every domain must be wired: %+v", deps)
	}
	if NewRuntimeDependencies(nil) != nil {
		t.Fatal("NewRuntimeDependencies(nil) must be nil")
	}
}

func TestRuntimeDependenciesFocusQueueReadsPlanningDependency(t *testing.T) {
	fake := &fakeDomainStore{planning: &PlanningState{
		FocusQueue: []FocusQueueItem{
			{TaskID: "t1", ProjectID: "p1"},
			{TaskID: "t2", ProjectID: "p1"},
		},
	}}
	deps := NewRuntimeDependencies(fake)

	queue, err := deps.FocusQueue()
	if err != nil {
		t.Fatalf("FocusQueue: %v", err)
	}
	if len(queue) != 2 {
		t.Fatalf("expected 2 focus queue items, got %d", len(queue))
	}
	if fake.loadCalls == 0 {
		t.Fatal("FocusQueue must load through the Planning dependency")
	}

	in, err := deps.TaskInFocusQueue("t2")
	if err != nil {
		t.Fatalf("TaskInFocusQueue: %v", err)
	}
	if !in {
		t.Fatal("t2 should be reported in the focus queue")
	}
	out, err := deps.TaskInFocusQueue("missing")
	if err != nil {
		t.Fatalf("TaskInFocusQueue: %v", err)
	}
	if out {
		t.Fatal("unknown task must not be reported in the focus queue")
	}
	if empty, _ := deps.TaskInFocusQueue(""); empty {
		t.Fatal("empty task id must not be a member")
	}
}

func TestManagerRuntimeDependenciesRoundTripsFocusQueue(t *testing.T) {
	store := newIsolatedStore(t)
	if err := store.SavePlanning(&PlanningState{
		FocusQueue: []FocusQueueItem{{TaskID: "focus-1", ProjectID: "proj-1"}},
	}); err != nil {
		t.Fatalf("SavePlanning: %v", err)
	}

	mgr := NewManager(filepath.Join(store.DataDir(), ".tt.json"))
	deps := mgr.RuntimeDependencies()
	if deps == nil {
		t.Fatal("Manager.RuntimeDependencies returned nil")
	}
	in, err := deps.TaskInFocusQueue("focus-1")
	if err != nil {
		t.Fatalf("TaskInFocusQueue: %v", err)
	}
	if !in {
		t.Fatal("persisted focus-queue member should be visible through the runtime seam")
	}
}
