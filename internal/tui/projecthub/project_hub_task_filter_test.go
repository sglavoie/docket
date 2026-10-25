package projecthub

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/listsurface"
)

// newTaskFilterFixture catalogs a project the count pass read as empty beside
// two holding open tasks, so one Ctrl+T press has something to hide. The empty
// project's name shares the "bet" prefix with Beta, so one query can match a
// counted and an empty row.
func newTaskFilterFixture(t *testing.T) *projectHubFixture {
	t.Helper()
	return newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
		scopeTestProject("empty", "Beta Empty"),
	)
}

func TestProjectHubTaskFilterCyclesThreeStates(t *testing.T) {
	hub := newTaskFilterFixture(t).open()
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("fixture omitted the empty project before filtering")
	}

	// Routed as the Hub routes it: the press resolves through the Hub's own
	// strategy, so the registered task-filter operation is what runs.
	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterWithOpenTasks {
		t.Fatalf("first Ctrl+T left filter=%v, want with open tasks", hub.taskFilter)
	}
	if projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("the with-open-tasks filter still lists a project holding no open tasks")
	}
	if !projectHubHasItem(hub, "project-hub:project:alpha") {
		t.Fatal("the with-open-tasks filter dropped a project holding open tasks")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterWithoutOpenTasks {
		t.Fatalf("second Ctrl+T left filter=%v, want without open tasks", hub.taskFilter)
	}
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("the without-open-tasks filter dropped a project holding no open tasks")
	}
	if projectHubHasItem(hub, "project-hub:project:alpha") {
		t.Fatal("the without-open-tasks filter still lists a project holding open tasks")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterAll {
		t.Fatalf("third Ctrl+T left filter=%v, want every project", hub.taskFilter)
	}
	for _, id := range []listsurface.ListItemID{"project-hub:project:empty", "project-hub:project:alpha"} {
		if !projectHubHasItem(hub, id) {
			t.Fatalf("the cleared axis omitted %q", id)
		}
	}
}

func TestProjectHubTaskFilterKeepsParkedAndArchivedProjectsWithOpenTasks(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	parked := scopeTestProject("parked", "Parked", "1", "2", "3")
	parked.Tags = []string{"parked"}
	fixture.add(parked)
	archived := scopeTestProject("archived", "Archived", "1", "2")
	archived.Archived = true
	fixture.add(archived)
	stale := scopeTestProject("stale", "Stale")
	stale.Archived = true
	fixture.add(stale)

	hub := fixture.open()
	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:parked") {
		t.Error("filter hid a parked project holding open tasks")
	}
	if !projectHubHasItem(hub, "project-hub:archived:archived") {
		t.Error("filter hid an archived project holding open tasks")
	}
	if projectHubHasItem(hub, "project-hub:archived:stale") {
		t.Error("filter left an empty archived project listed")
	}

	hub.Update(syntheticCtrlKey('t'))
	if projectHubHasItem(hub, "project-hub:project:parked") {
		t.Error("the without-open-tasks filter left a parked project holding open tasks listed")
	}
	if !projectHubHasItem(hub, "project-hub:archived:stale") {
		t.Error("the without-open-tasks filter hid an empty archived project")
	}
}

func TestProjectHubTaskFilterKeepsUnknownCounts(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	// A project whose tasks.md failed to parse never reaches the counts map, so
	// the Hub knows neither that it holds work nor that it holds none.
	fixture.addUncounted(scopeTestProject("corrupt", "Corrupt"))
	fixture.add(scopeTestProject("empty", "Empty"))

	hub := fixture.open()
	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:corrupt") {
		t.Error("the with-tasks filter hid a project whose count could not be read")
	}
	if projectHubHasItem(hub, "project-hub:project:empty") {
		t.Error("the with-tasks filter left a known-empty project listed")
	}

	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:corrupt") {
		t.Error("the without-tasks filter hid a project whose count could not be read")
	}
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Error("the without-tasks filter hid a known-empty project")
	}
}

func TestProjectHubTaskFilterTogglesUnderActiveSearch(t *testing.T) {
	hub := newTaskFilterFixture(t).open()
	hub.Update(syntheticKey('/'))
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	if hub.SearchQuery() != "bet" || len(hub.surface.Matches()) != 2 {
		t.Fatalf("query=%q matches=%d, want Beta and Beta Empty", hub.SearchQuery(), len(hub.surface.Matches()))
	}

	hub.Update(syntheticCtrlKey('t'))
	if !hub.HasActiveSearch() || hub.SearchQuery() != "bet" {
		t.Fatalf("toggle left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	matches := hub.surface.Matches()
	if len(matches) != 1 || matches[0].Item.ID != "project-hub:project:beta" {
		t.Fatalf("filtered matches = %d, want Beta alone", len(matches))
	}

	for _, r := range "zzz" {
		hub.Update(syntheticKey(r))
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != listsurface.ListEmptyNoMatches {
		t.Fatalf("no-match context = (%+v, %v)", context, ok)
	}
	if want := "Press Esc to clear the query, or Ctrl+T to show projects without open tasks."; context.State.Detail != want {
		t.Fatalf("no-match detail = %q, want %q", context.State.Detail, want)
	}
}

func TestProjectHubTaskFilterIgnoredUnderActionSheet(t *testing.T) {
	hub := newTaskFilterFixture(t).open()
	focusProjectHubItem(t, hub, "project-hub:project:empty")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not open the action sheet")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterAll {
		t.Fatal("Ctrl+T reached the Hub while the action sheet owned the key stream")
	}
	if hub.actions == nil {
		t.Fatal("Ctrl+T dismissed the action sheet")
	}
	if !projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("Ctrl+T filtered the list from under the action sheet")
	}
}

func TestProjectHubTaskFilterDropsHiddenProjectsFromSelection(t *testing.T) {
	hub := newTaskFilterFixture(t).open()
	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey('v'))
	focusProjectHubItem(t, hub, "project-hub:project:empty")
	hub.Update(syntheticKey('v'))
	if got := hub.surface.SelectedIDs(); len(got) != 2 {
		t.Fatalf("selected IDs = %v, want both projects", got)
	}

	hub.Update(syntheticCtrlKey('t'))
	got := hub.surface.SelectedIDs()
	if len(got) != 1 || got[0] != "project-hub:project:beta" {
		t.Fatalf("selected IDs after filtering = %v, want the surviving project alone", got)
	}

	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not open the bulk action sheet")
	}
	identities := hub.actions.Context.Selection.EntityIdentities
	if len(identities) != 1 || identities[0].ProjectID != "beta" {
		t.Fatalf("bulk targets = %+v, want beta alone", identities)
	}
}

func TestProjectHubTaskFilterEmptyCopyNamesTheToggle(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha"),
		scopeTestProject("beta", "Beta"),
	).open()

	hub.Update(syntheticCtrlKey('t'))
	if got := hub.surface.Items(); len(got) != 0 {
		t.Fatalf("filtered items = %d, want every project hidden", len(got))
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != listsurface.ListEmptySource {
		t.Fatalf("empty context = (%+v, %v), want the source-empty kind", context, ok)
	}
	if context.State.Title != "No projects hold open tasks." ||
		context.State.Detail != "Press Ctrl+T to show projects without open tasks." {
		t.Fatalf("filtered empty copy = %+v, want the filter-specific copy", context.State)
	}

	hub.Update(syntheticCtrlKey('t'))
	if _, ok := hub.surface.EmptyContext(); ok {
		t.Fatal("the without-open-tasks filter left the Hub empty")
	}
}

func TestProjectHubTaskFilterEmptyCopyNamesTheFullList(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	).open()
	hub.Update(syntheticCtrlKey('t'))
	hub.Update(syntheticCtrlKey('t'))
	if got := hub.surface.Items(); len(got) != 0 {
		t.Fatalf("without-open-tasks items = %d, want every project hidden", len(got))
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != listsurface.ListEmptySource {
		t.Fatalf("empty context = (%+v, %v), want the source-empty kind", context, ok)
	}
	if context.State.Title != "Every project holds open tasks." ||
		context.State.Detail != "Press Ctrl+T to show every project." {
		t.Fatalf("without-open-tasks empty copy = %+v, want the way back to the full list", context.State)
	}
}

func TestProjectHubReopenClearsTaskFilter(t *testing.T) {
	fixture := newTaskFilterFixture(t)
	hub := fixture.open()
	hub.Update(syntheticCtrlKey('t'))
	if projectHubHasItem(hub, "project-hub:project:empty") {
		t.Fatal("filter did not hide the empty project")
	}

	reopened := fixture.open()
	if reopened == hub {
		t.Fatal("reopening the Hub reused the filtered model")
	}
	if reopened.taskFilter != projectHubTaskFilterAll || !projectHubHasItem(reopened, "project-hub:project:empty") {
		t.Fatalf("reopened hub filter=%v, want it cleared", reopened.taskFilter)
	}
}

func TestProjectHubSubtitleNamesTheTaskFilter(t *testing.T) {
	hub := newTaskFilterFixture(t).open()

	counts := hub.subtitle()
	if strings.Contains(counts, "with open tasks") {
		t.Fatalf("unfiltered subtitle = %q, want no filter marker", counts)
	}

	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey('v'))
	hub.Update(syntheticCtrlKey('t'))
	if !projectHubHasItem(hub, "project-hub:project:beta") {
		t.Fatal("the filter hid the selected project")
	}
	// The leading counts are asserted verbatim from the unfiltered read, so a
	// subtitle reporting filtered totals rather than workspace totals fails here.
	if want := counts + " · with open tasks · 1 selected"; hub.subtitle() != want {
		t.Fatalf("filtered subtitle = %q, want %q", hub.subtitle(), want)
	}

	// The next press hides the selected project, so the selection it names is
	// dropped with it and only the marker is left to change.
	hub.Update(syntheticCtrlKey('t'))
	if want := counts + " · without open tasks"; hub.subtitle() != want {
		t.Fatalf("without-open-tasks subtitle = %q, want %q", hub.subtitle(), want)
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.subtitle() != counts {
		t.Fatalf("cleared subtitle = %q, want %q", hub.subtitle(), counts)
	}
}

func TestProjectHubTaskFilterHintSurvivesTheEmptiedList(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha"),
		scopeTestProject("beta", "Beta"),
	).open()

	spec, ok := hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "Ctrl+T with open tasks") {
		t.Fatalf("browse guidance omits the filter hint: %q", help)
	}

	hub.Update(syntheticCtrlKey('t'))
	if got := hub.surface.Items(); len(got) != 0 {
		t.Fatalf("filtered items = %d, want every project hidden", len(got))
	}
	spec, ok = hub.ContextualGuidance()
	help := guidanceEntryText(spec)
	if !ok || !strings.Contains(help, "Ctrl+T without open tasks") {
		t.Fatalf("emptied guidance omits the next state: %q", help)
	}
	if strings.Contains(help, "/ search") {
		t.Fatalf("emptied guidance still offers search: %q", help)
	}

	hub.Update(syntheticCtrlKey('t'))
	spec, ok = hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "Ctrl+T all projects") {
		t.Fatalf("without-open-tasks guidance omits the way back: %q", help)
	}
}
