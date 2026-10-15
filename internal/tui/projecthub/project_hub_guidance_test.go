package projecthub

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProjectHubGuidanceFollowsBrowseSearchSelectionAndActions(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	).open()

	spec, ok := hub.ContextualGuidance()
	assertGuidanceContains(t, "project browse", spec, ok,
		"Enter open", "Space actions", "a new", "e edit", "* pin",
		"# project tags", "Ctrl+N notes", "/ search")
	if spec.Rows != 1 {
		t.Fatalf("browse guidance rows = %d, want 1", spec.Rows)
	}

	_, _ = hub.Update(syntheticKey('/'))
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project search editing", spec, ok,
		"Enter open", "Esc browse matches", "↑/↓ or Ctrl+J/K navigate")
	if spec.Rows != 2 {
		t.Fatalf("search-edit guidance rows = %d, want 2", spec.Rows)
	}
	if len(spec.Hints) < 2 || !spec.Hints[0].Required || !spec.Hints[1].Required {
		t.Fatalf("required search controls do not precede browse hints: %+v", spec.Hints)
	}
	// Typing the query is not the moment to advertise the whole grammar.
	if got := guidanceEntryText(spec); strings.Contains(got, "a new") {
		t.Fatalf("search-edit guidance still offers the browse grammar: %q", got)
	}
	_, _ = hub.Update(syntheticSpecialKey(tea.KeyEsc))

	hub.SetSearchQuery("alpha")
	hub.applyProjectFilter()
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project applied search", spec, ok, "/ search", "Esc clear query")
	if !strings.Contains(spec.Supplemental, "search: alpha") {
		t.Fatalf("applied-search supplemental = %q", spec.Supplemental)
	}

	hub.ClearSearch()
	hub.applyProjectFilter()
	hub.ToggleProjectHubSelection()
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project selection", spec, ok, "Space actions", "v select", "Ctrl+A select all")
	if spec.Supplemental != "1 selected" {
		t.Fatalf("selection supplemental = %q", spec.Supplemental)
	}

	hub.surface.ClearSelection()
	hub.OpenProjectHubActions()
	if hub.actions == nil {
		t.Fatal("project action sheet did not open")
	}
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project actions", spec, ok, "Enter run", "Esc close")
	_, _ = hub.Update(syntheticKey('/'))
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project action search", spec, ok, "Enter apply search", "Esc cancel search")
	if spec.Rows != 2 {
		t.Fatalf("action-search guidance rows = %d, want 2", spec.Rows)
	}
}

func TestProjectHubGuidanceUsesCausalEmptyRecovery(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	).open()
	if err := hub.surface.Reload(nil, hub.projectHubCategories()); err != nil {
		t.Fatalf("empty Project Hub surface: %v", err)
	}
	spec, ok := hub.ContextualGuidance()
	assertGuidanceContains(t, "empty projects", spec, ok, "a new")
	if len(spec.Hints) == 0 || !spec.Hints[0].Required {
		t.Fatalf("empty-project recovery is not required: %+v", spec.Hints)
	}
}
