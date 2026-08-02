package tui

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// mutationFixture builds an in-memory workspace: two "work" projects, one
// personal project, and one archived "work" project.
func mutationFixture(t *testing.T) (*ContextMutator, *config.RuntimeState) {
	t.Helper()
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
		{ID: "p2", Slug: "bravo", Name: "Bravo", Context: "work"},
		{ID: "p3", Slug: "charlie", Name: "Charlie", Context: "personal"},
		{ID: "p4", Slug: "delta", Name: "Delta", Context: "work", Archived: true},
	})
	return NewContextMutator(runtime, nil, nil), runtime
}

func contextOf(t *testing.T, runtime *config.RuntimeState, projectID string) string {
	t.Helper()
	project := runtime.GetProject(projectID)
	if project == nil {
		t.Fatalf("project %q is absent from the catalog", projectID)
	}
	return project.Context
}

func TestCreateRegularContextRequiresActiveMembership(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{Name: "focus"}); err == nil {
		t.Fatal("creating a context without an active project should fail")
	}
	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		Name:       "focus",
		ProjectIDs: []string{"p4"},
	}); err == nil {
		t.Fatal("creating a context with only an archived project should fail")
	}

	result, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		Name:       "focus",
		ProjectIDs: []string{"p1"},
	})
	if err != nil {
		t.Fatalf("create regular context: %v", err)
	}
	if !result.IsNew || result.Assigned != 1 {
		t.Fatalf("result = %+v, want a one-project creation", result)
	}
	if got := contextOf(t, runtime, "p1"); got != "focus" {
		t.Fatalf("p1 context = %q, want focus", got)
	}
}

func TestSaveRegularContextRelabelsEveryMatchingProject(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	result, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "work",
		Name:         "Deep Work",
		ProjectIDs:   []string{"p1", "p2", "p4"},
	})
	if err != nil {
		t.Fatalf("SaveRegularContext: %v", err)
	}
	if !result.Renamed || result.RelabeledCount != 3 {
		t.Fatalf("result = %+v, want 3 relabeled projects (archived members included)", result)
	}
	if result.Assigned != 0 || result.Removed != 0 {
		t.Fatalf("result = %+v, want a rename that changes no membership", result)
	}
	for _, id := range []string{"p1", "p2", "p4"} {
		if got := contextOf(t, runtime, id); got != "Deep Work" {
			t.Fatalf("project %s context = %q, want Deep Work", id, got)
		}
	}
	if got := contextOf(t, runtime, "p3"); got != "personal" {
		t.Fatalf("unrelated project context = %q, want personal", got)
	}
}

func TestSaveRegularContextRejectsCollisionButAllowsRecasing(t *testing.T) {
	mutator, runtime := mutationFixture(t)
	members := []string{"p1", "p2", "p4"}

	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "work",
		Name:         "Personal",
		ProjectIDs:   members,
	}); err == nil {
		t.Fatal("renaming onto an existing label should fail case-insensitively")
	}
	if got := contextOf(t, runtime, "p1"); got != "work" {
		t.Fatalf("rejected rename mutated live state: p1 context = %q", got)
	}

	if _, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "work",
		Name:         "Work",
		ProjectIDs:   members,
	}); err != nil {
		t.Fatalf("case-only rename: %v", err)
	}
	if got := contextOf(t, runtime, "p1"); got != "Work" {
		t.Fatalf("p1 context = %q, want Work", got)
	}
}

func TestDeleteRegularContextReassignsMembersToDefault(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	count, err := mutator.DeleteRegularContext("work")
	if err != nil {
		t.Fatalf("DeleteRegularContext: %v", err)
	}
	if count != 3 {
		t.Fatalf("reassigned %d projects, want 3", count)
	}
	for _, id := range []string{"p1", "p2", "p4"} {
		if got := contextOf(t, runtime, id); got != "personal" {
			t.Fatalf("project %s context = %q, want the default context", id, got)
		}
	}
}

func TestDeleteRegularContextBlocksTheDefaultContext(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	if _, err := mutator.DeleteRegularContext("Personal"); !errors.Is(err, ErrDefaultContextProtected) {
		t.Fatalf("err = %v, want ErrDefaultContextProtected", err)
	}
	if got := contextOf(t, runtime, "p3"); got != "personal" {
		t.Fatalf("blocked delete mutated live state: p3 context = %q", got)
	}
}

func TestSaveRegularContextBlocksPersonalRename(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	_, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "personal",
		Name:         "private",
		ProjectIDs:   []string{"p3"},
	})
	if !errors.Is(err, ErrDefaultContextProtected) {
		t.Fatalf("err = %v, want ErrDefaultContextProtected", err)
	}
	if got := contextOf(t, runtime, "p3"); got != "personal" {
		t.Fatalf("blocked rename mutated p3 context to %q", got)
	}
}

func TestSaveRegularContextAssignsAndFallsBackToDefault(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	result, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "work",
		Name:         "work",
		ProjectIDs:   []string{"p1", "p3"},
	})
	if err != nil {
		t.Fatalf("SaveRegularContext: %v", err)
	}
	if result.Renamed {
		t.Fatalf("result = %+v, want a membership-only save", result)
	}
	if result.Assigned != 1 || result.Removed != 2 {
		t.Fatalf("result = %+v, want {Assigned:1 Removed:2}", result)
	}
	if got := contextOf(t, runtime, "p3"); got != "work" {
		t.Fatalf("checked project context = %q, want work", got)
	}
	for _, id := range []string{"p2", "p4"} {
		if got := contextOf(t, runtime, id); got != "personal" {
			t.Fatalf("unchecked project %s context = %q, want the default context", id, got)
		}
	}
}

func TestSaveRegularContextFromDefaultNeedsADestination(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	_, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "personal",
		Name:         "personal",
		ProjectIDs:   nil,
	})
	if !errors.Is(err, ErrDestinationContextRequired) {
		t.Fatalf("err = %v, want ErrDestinationContextRequired", err)
	}
	if got := contextOf(t, runtime, "p3"); got != "personal" {
		t.Fatalf("rejected removal mutated live state: p3 context = %q", got)
	}
	_, err = mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "personal",
		Name:         "personal",
		Destination:  "missing",
	})
	if err == nil {
		t.Fatal("a destination that is not an existing context should fail")
	}

	result, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "personal",
		Name:         "personal",
		Destination:  "work",
	})
	if err != nil {
		t.Fatalf("SaveRegularContext with destination: %v", err)
	}
	if result.Removed != 1 {
		t.Fatalf("result = %+v, want one removal", result)
	}
	if got := contextOf(t, runtime, "p3"); got != "work" {
		t.Fatalf("p3 context = %q, want the destination context", got)
	}
}

func TestSaveRegularContextFromDefaultBlockedWithoutAnyOtherContext(t *testing.T) {
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "personal"},
	})
	mutator := NewContextMutator(runtime, nil, nil)

	_, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "personal",
		Name:         "personal",
	})
	if !errors.Is(err, ErrNoDestinationContextAvailable) {
		t.Fatalf("err = %v, want ErrNoDestinationContextAvailable", err)
	}
	if got := contextOf(t, runtime, "p1"); got != "personal" {
		t.Fatalf("blocked removal mutated live state: p1 context = %q", got)
	}
}

func TestSaveRegularContextIntoDefaultNeedsNoDestination(t *testing.T) {
	mutator, runtime := mutationFixture(t)

	result, err := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "personal",
		Name:         "personal",
		ProjectIDs:   []string{"p1", "p3"},
	})
	if err != nil {
		t.Fatalf("SaveRegularContext: %v", err)
	}
	if result.Assigned != 1 || result.Removed != 0 {
		t.Fatalf("result = %+v, want {Assigned:1 Removed:0}", result)
	}
	if got := contextOf(t, runtime, "p1"); got != "personal" {
		t.Fatalf("p1 context = %q, want personal", got)
	}
}

func TestRegularContextMutationCommitsCheckedCatalogArtifact(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()

	seed := newTestConfigAt(dataDir)
	setTestProjects(seed, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
		{ID: "p2", Slug: "bravo", Name: "Bravo", Context: "personal"},
	})
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if err := saveTestRuntime(manager, seed); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	service := newTUIAppService(dataDir, manager.RuntimeDependencies(), workspaceLedgerFromConfig(runtime))

	mutator := NewContextMutator(runtime, manager, service)
	if _, saveErr := mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: "work",
		Name:         "Deep Work",
		ProjectIDs:   []string{"p1"},
	}); saveErr != nil {
		t.Fatalf("SaveRegularContext: %v", saveErr)
	}

	if got := contextOf(t, runtime, "p1"); got != "Deep Work" {
		t.Fatalf("runtime snapshot context = %q, want Deep Work", got)
	}
	reloaded, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	if got := contextOf(t, reloaded, "p1"); got != "Deep Work" {
		t.Fatalf("persisted context = %q, want Deep Work", got)
	}

	// The snapshot advanced to the committed revision, so a follow-up mutation
	// commits against a fresh baseline instead of a stale one.
	if _, deleteErr := mutator.DeleteRegularContext("Deep Work"); deleteErr != nil {
		t.Fatalf("DeleteRegularContext after refresh: %v", deleteErr)
	}
	if got := contextOf(t, runtime, "p1"); got != "personal" {
		t.Fatalf("context after delete = %q, want personal", got)
	}
}
