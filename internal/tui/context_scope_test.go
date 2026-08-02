package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// scopeFixture builds a workspace with active and archived regular-context
// members.
func scopeFixture(t *testing.T) *config.RuntimeState {
	t.Helper()
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
		{ID: "p2", Slug: "bravo", Name: "Bravo", Context: "work"},
		{ID: "p3", Slug: "charlie", Name: "Charlie", Context: "home"},
		{ID: "p4", Slug: "delta", Name: "Delta", Context: "work", Archived: true},
	})
	return runtime
}

func assertIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("project IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("project IDs = %v, want %v", got, want)
		}
	}
}

func TestContextScopeAllResolvesEveryActiveProject(t *testing.T) {
	runtime := scopeFixture(t)

	assertIDs(t, AllContextScope().ProjectIDs(runtime), "p1", "p2", "p3")

	summary := SummarizeContextScope(runtime, AllContextScope())
	if summary.ActiveProjectCount != 3 {
		t.Fatalf("active count = %d, want 3", summary.ActiveProjectCount)
	}
}

func TestContextScopeRegularResolvesMatchingActiveProjects(t *testing.T) {
	runtime := scopeFixture(t)

	// "Delta" carries the work context but is archived, so it is not selectable.
	assertIDs(t, RegularContextScope("work").ProjectIDs(runtime), "p1", "p2")
	assertIDs(t, RegularContextScope("home").ProjectIDs(runtime), "p3")
	assertIDs(t, RegularContextScope("nope").ProjectIDs(runtime))

	summary := SummarizeContextScope(runtime, RegularContextScope("work"))
	if summary.ActiveProjectCount != 2 {
		t.Fatalf("active count = %d, want 2", summary.ActiveProjectCount)
	}
}

// TestContextScopesResolveOnlyLoadableProjects locks the hub preview to
// the Outliner: every project a scope reports must be one the active-only
// cross-project loader actually selects, or the hub counts rows no view shows.
func TestContextScopesResolveOnlyLoadableProjects(t *testing.T) {
	runtime := scopeFixture(t)

	loadable := make(map[string]bool)
	for _, project := range selectCrossScopeProjects(runtime, CrossScopeOpts{ActiveOnly: true}) {
		loadable[project.ID] = true
	}
	for _, summary := range ContextScopeSummaries(runtime) {
		projects := summary.Scope.Projects(runtime)
		if len(projects) != summary.ActiveProjectCount {
			t.Fatalf("%#v previews %d projects but counts %d",
				summary.Scope, len(projects), summary.ActiveProjectCount)
		}
		for _, project := range projects {
			if !loadable[project.ID] {
				t.Fatalf("%#v resolves %q, which the Outliner never loads", summary.Scope, project.ID)
			}
		}
	}
}

func TestContextScopeSummariesOrderAllThenRegularAndKeepPersonal(t *testing.T) {
	runtime := scopeFixture(t)

	summaries := ContextScopeSummaries(runtime)

	type entry struct {
		kind   ContextScopeKind
		name   string
		active int
	}
	want := []entry{
		{ContextScopeAll, "", 3},
		{ContextScopeRegular, "home", 1},
		{ContextScopeRegular, "personal", 0},
		{ContextScopeRegular, "work", 2},
	}
	if len(summaries) != len(want) {
		t.Fatalf("summaries = %+v, want %d entries", summaries, len(want))
	}
	for i, w := range want {
		got := summaries[i]
		if got.Scope.Kind != w.kind || got.Scope.Name != w.name || got.ActiveProjectCount != w.active {
			t.Fatalf("summary[%d] = {kind:%d name:%q active:%d}, want {kind:%d name:%q active:%d}",
				i, got.Scope.Kind, got.Scope.Name, got.ActiveProjectCount, w.kind, w.name, w.active)
		}
	}
}

func TestContextScopeEmptyNameFallsBackToAll(t *testing.T) {
	scope := RegularContextScope("")
	if !scope.IsAll() {
		t.Fatalf("scope %+v with empty name should be the All scope", scope)
	}
}
