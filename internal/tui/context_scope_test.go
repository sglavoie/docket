package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// scopeFixture builds a workspace with one active/one archived project per
// context plus a "Work" custom context holding an active, an archived, and a
// deleted project reference.
func scopeFixture(t *testing.T) *config.RuntimeState {
	t.Helper()
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
		{ID: "p2", Slug: "bravo", Name: "Bravo", Context: "work"},
		{ID: "p3", Slug: "charlie", Name: "Charlie", Context: "home"},
		{ID: "p4", Slug: "delta", Name: "Delta", Context: "work", Archived: true},
	})
	runtime.Machine().SetCustomContext("Work", []string{"p1", "p4", "missing"})
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
	if summary.ArchivedProjectCount != 0 || summary.StaleReferenceCount != 0 {
		t.Fatalf("All scope reported archived=%d stale=%d, want 0/0",
			summary.ArchivedProjectCount, summary.StaleReferenceCount)
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
	if summary.ArchivedProjectCount != 0 || summary.StaleReferenceCount != 0 {
		t.Fatalf("regular scope reported archived=%d stale=%d, want 0/0",
			summary.ArchivedProjectCount, summary.StaleReferenceCount)
	}
}

func TestCustomContextScopeApplyDropsArchivedAndStaleReferences(t *testing.T) {
	runtime := scopeFixture(t)

	assertIDs(t, CustomContextScope("Work").ProjectIDs(runtime), "p1")

	summary := SummarizeContextScope(runtime, CustomContextScope("Work"))
	if summary.ActiveProjectCount != 1 {
		t.Fatalf("active count = %d, want 1", summary.ActiveProjectCount)
	}
	if summary.ArchivedProjectCount != 1 {
		t.Fatalf("archived count = %d, want 1", summary.ArchivedProjectCount)
	}
	if summary.StaleReferenceCount != 1 {
		t.Fatalf("stale count = %d, want 1", summary.StaleReferenceCount)
	}

	// Summarizing must not prune the stored references.
	stored := runtime.Machine().GetCustomContext("Work")
	if len(stored) != 3 {
		t.Fatalf("stored custom context IDs = %v, want 3 entries", stored)
	}
}

// hiddenMemberFixture stores a custom context whose membership spans every
// eligibility buckets at once: one active project, one archived project, and
// one reference to a project that no longer exists.
func hiddenMemberFixture(t *testing.T) *config.RuntimeState {
	t.Helper()
	runtime := newTestConfig()
	setTestProjects(runtime, []*models.Project{
		{ID: "p1", Slug: "alpha", Name: "Alpha", Context: "work"},
		{ID: "p2", Slug: "bravo", Name: "Bravo", Context: "work"},
		{ID: "p4", Slug: "delta", Name: "Delta", Context: "work", Archived: true},
	})
	runtime.Machine().SetCustomContext("Work", []string{"p1", "p4", "missing"})
	return runtime
}

// TestCustomContextScopeClassifiesArchivedAndStaleMembers pins the eligibility
// rule and ensures the counts partition stored membership.
func TestCustomContextScopeClassifiesArchivedAndStaleMembers(t *testing.T) {
	runtime := hiddenMemberFixture(t)

	assertIDs(t, CustomContextScope("Work").ProjectIDs(runtime), "p1")

	summary := SummarizeContextScope(runtime, CustomContextScope("Work"))
	if summary.ActiveProjectCount != 1 || summary.ArchivedProjectCount != 1 ||
		summary.StaleReferenceCount != 1 {
		t.Fatalf("summary = %+v, want active=1 archived=1 missing=1", summary)
	}

	stored := runtime.Machine().GetCustomContext("Work")
	counted := summary.ActiveProjectCount + summary.ArchivedProjectCount +
		summary.StaleReferenceCount
	if counted != len(stored) {
		t.Fatalf("counts total %d, want the %d stored members %v", counted, len(stored), stored)
	}

	// Regular scopes are derived from the active-project set.
	assertIDs(t, RegularContextScope("work").ProjectIDs(runtime), "p1", "p2")
}

// TestCustomContextScopeResolvesOnlyLoadableProjects locks the hub preview to
// the Outliner: every project a scope reports must be one the active-only
// cross-project loader actually selects, or the hub counts rows no view shows.
func TestCustomContextScopeResolvesOnlyLoadableProjects(t *testing.T) {
	runtime := hiddenMemberFixture(t)

	loadable := make(map[string]bool)
	for _, project := range selectCrossScopeProjects(runtime, CrossScopeOpts{ActiveOnly: true}) {
		loadable[project.ID] = true
	}
	if loadable["p5"] || loadable["p6"] {
		t.Fatalf("the active-only loader selected a sandbox or internal project: %v", loadable)
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

func TestCustomContextScopeIsDistinctFromSameNamedRegularContext(t *testing.T) {
	runtime := scopeFixture(t)

	custom := CustomContextScope("Work")
	regular := RegularContextScope("work")
	if custom.Kind == regular.Kind {
		t.Fatal("custom and regular scopes must not share a kind")
	}
	assertIDs(t, custom.ProjectIDs(runtime), "p1")
	assertIDs(t, regular.ProjectIDs(runtime), "p1", "p2")
}

func TestContextScopeSummariesOrderAllCustomThenRegular(t *testing.T) {
	runtime := scopeFixture(t)
	runtime.Machine().SetCustomContext("Admin", []string{"p2"})

	summaries := ContextScopeSummaries(runtime)

	type entry struct {
		kind   ContextScopeKind
		name   string
		active int
	}
	want := []entry{
		{ContextScopeAll, "", 3},
		{ContextScopeCustom, "Admin", 1},
		{ContextScopeCustom, "Work", 1},
		{ContextScopeRegular, "home", 1},
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
	if summaries[2].StaleReferenceCount != 1 {
		t.Fatalf("Work stale count = %d, want 1", summaries[2].StaleReferenceCount)
	}
}

func TestContextScopeEmptyNameFallsBackToAll(t *testing.T) {
	for _, scope := range []ContextScope{RegularContextScope(""), CustomContextScope("")} {
		if !scope.IsAll() {
			t.Fatalf("scope %+v with empty name should be the All scope", scope)
		}
	}
}
