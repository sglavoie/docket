package projecthub

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/route"
)

// The tallies here are handed to the Hub the way the shell's count pass hands
// them over. Whether that pass reads a parked project's tasks.md off disk is the
// shell's own contract and is asserted beside it; what the Hub owes is the row
// and preview text it draws from whatever it was given.

func TestProjectHubListsEveryProjectByLifecycle(t *testing.T) {
	fixture := newProjectHubFixture(t)
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Pinned = true
	fixture.add(alpha)
	fixture.add(scopeTestProject("beta", "Beta", "1"))
	parked := scopeTestProject("parked", "Parked")
	parked.Tags = []string{"parked"}
	fixture.add(parked)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	fixture.add(archived)

	hub := fixture.open()
	wantCategories := []listsurface.ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryParked, Label: "Parked Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
	if got := hub.surface.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("project categories = %#v, want %#v", got, wantCategories)
	}
	for id, want := range map[listsurface.ListItemID]listsurface.ListCategoryID{
		"project-hub:project:alpha":     projectHubCategoryActive,
		"project-hub:project:parked":    projectHubCategoryParked,
		"project-hub:archived:archived": projectHubCategoryArchived,
	} {
		if got := projectHubItem(t, hub, id).CategoryID; got != want {
			t.Errorf("%s drew under %q, want %q", id, got, want)
		}
	}
	for _, id := range []listsurface.ListItemID{
		"project-hub:project:alpha",
		"project-hub:project:beta",
		"project-hub:archived:archived",
	} {
		if !hub.surface.SetCursorToID(id) {
			t.Errorf("Project Hub omitted %q", id)
		}
	}
	for _, item := range hub.surface.Items() {
		if _, ok := item.Data.(projectHubEntry); !ok {
			t.Fatalf("Project Hub row carries %#v, want a project entry", item.Data)
		}
	}
}

func TestPinnedProjectHubListsPinnedActiveAndParkedProjectsOnly(t *testing.T) {
	fixture := newProjectHubFixture(t)
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Pinned = true
	fixture.add(alpha)
	fixture.add(scopeTestProject("beta", "Beta", "1"))
	parked := scopeTestProject("parked", "Parked")
	parked.Pinned = true
	parked.Tags = []string{"parked"}
	fixture.add(parked)
	archived := scopeTestProject("archived", "Archived")
	archived.Pinned = true
	archived.Archived = true
	fixture.add(archived)

	hub := fixture.open(route.ProjectHubModePinned)
	if hub.Mode() != route.ProjectHubModePinned {
		t.Fatalf("Hub mode = %v, want pinned", hub.Mode())
	}
	wantCategories := []listsurface.ListCategory{{ID: projectHubCategoryActive, Label: "Pinned Projects"}}
	if got := hub.surface.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("pinned categories = %#v, want %#v", got, wantCategories)
	}
	for _, id := range []listsurface.ListItemID{"project-hub:project:alpha", "project-hub:project:parked"} {
		if !projectHubHasItem(hub, id) {
			t.Errorf("pinned Hub omitted %q", id)
		}
	}
	for _, id := range []listsurface.ListItemID{"project-hub:project:beta", "project-hub:archived:archived"} {
		if projectHubHasItem(hub, id) {
			t.Errorf("pinned Hub included %q", id)
		}
	}
	view := hub.View().Content
	for _, copy := range []string{"Select Pinned Project", "2 pinned active", "Pinned Projects"} {
		if !strings.Contains(strings.ToLower(view), strings.ToLower(copy)) {
			t.Errorf("pinned Hub view omitted %q:\n%s", copy, view)
		}
	}
}

func TestPinnedProjectHubSearchAndTaskFilterIntersectPinnedPopulation(t *testing.T) {
	fixture := newProjectHubFixture(t)
	fixture.add(scopeTestProject("alpha", "Alpha", "1", "2"))
	beta := scopeTestProject("beta", "Beta", "1")
	beta.Pinned = true
	fixture.add(beta)
	empty := scopeTestProject("empty", "Beta Empty")
	empty.Pinned = true
	fixture.add(empty)
	hub := fixture.open(route.ProjectHubModePinned)

	hub.SetSearchQuery("bet")
	hub.applyProjectFilter()
	if got := len(hub.surface.Matches()); got != 2 {
		t.Fatalf("pinned search matches = %d, want Beta and Beta Empty", got)
	}
	hub.CycleProjectHubTaskFilter()
	matches := hub.surface.Matches()
	if len(matches) != 1 || matches[0].Item.ID != "project-hub:project:beta" {
		t.Fatalf("pinned filtered matches = %+v, want Beta alone", matches)
	}
	hub.SetSearchQuery("missing")
	hub.applyProjectFilter()
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != listsurface.ListEmptyNoMatches || context.State.Title != "No pinned projects match this search." {
		t.Fatalf("pinned no-match context = (%+v, %v)", context, ok)
	}
}

func TestPinnedProjectHubRefreshPrunesRemovedCursorAndSelection(t *testing.T) {
	fixture := newProjectHubFixture(t)
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Pinned = true
	fixture.add(alpha)
	fixture.add(scopeTestProject("beta", "Beta", "1"))
	hub := fixture.open(route.ProjectHubModePinned)
	focusProjectHubItem(t, hub, "project-hub:project:alpha")
	hub.ToggleProjectHubSelection()

	alpha.Pinned = false
	fixture.sync()
	hub.RefreshProjectHub()
	if hub.surface.Cursor() != -1 || len(hub.surface.SelectedIDs()) != 0 {
		t.Fatalf("removed pinned row left cursor=%d selection=%v", hub.surface.Cursor(), hub.surface.SelectedIDs())
	}
	context, ok := hub.surface.EmptyContext()
	if !ok || context.Kind != listsurface.ListEmptySource || context.State.Title != "No active projects are pinned." {
		t.Fatalf("last-row empty context = (%+v, %v)", context, ok)
	}
	if !strings.Contains(context.State.Detail, "Esc") || !strings.Contains(context.State.Detail, "P") {
		t.Fatalf("pinned empty detail = %q, want Esc and P routes", context.State.Detail)
	}
}

func TestProjectHubCountsParkedAndArchivedProjects(t *testing.T) {
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

	hub := fixture.open()
	// The Parked heading says what the chip would, so the row drops it.
	if got := projectHubItem(t, hub, "project-hub:project:parked").SecondaryText; got != "3 open · 3 tasks" {
		t.Fatalf("parked row secondary = %q, want the real count without #parked", got)
	}
	if got, want := projectHubPreviewSummary(t, hub, "project-hub:project:parked"), "Parked · 3 open · 3 tasks · slug parked"; got != want {
		t.Fatalf("parked preview meta = %q, want %q", got, want)
	}
	want := "Archived · 2 open · 2 tasks · slug archived"
	if got := projectHubPreviewSummary(t, hub, "project-hub:archived:archived"); got != want {
		t.Fatalf("archived preview meta = %q, want %q", got, want)
	}
}

func TestProjectHubCountsOpenTasksBesideTotal(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	mixed := scopeTestProject("mixed", "Mixed", "1", "2", "3")
	markDone(mixed.Tasks[0])
	markDone(mixed.Tasks[1])
	fixture.add(mixed)

	hub := fixture.open()
	if got := projectHubItem(t, hub, "project-hub:project:mixed").SecondaryText; got != "1 open · 3 tasks" {
		t.Fatalf("partly finished row secondary = %q, want %q", got, "1 open · 3 tasks")
	}
	want := "Active · 1 open · 3 tasks · slug mixed"
	if got := projectHubPreviewSummary(t, hub, "project-hub:project:mixed"); got != want {
		t.Fatalf("partly finished preview meta = %q, want %q", got, want)
	}
}

func TestProjectHubHidesAllFinishedProjectUnderTheOpenTaskFilter(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	finished := scopeTestProject("finished", "Finished", "1", "2")
	markDone(finished.Tasks[0])
	markDone(finished.Tasks[1])
	fixture.add(finished)

	hub := fixture.open()
	if got := projectHubItem(t, hub, "project-hub:project:finished").SecondaryText; got != "0 open · 2 tasks" {
		t.Fatalf("all-finished row secondary = %q, want %q", got, "0 open · 2 tasks")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterWithOpenTasks {
		t.Fatalf("Ctrl+T left filter=%v, want projects holding open tasks", hub.taskFilter)
	}
	if projectHubHasItem(hub, "project-hub:project:finished") {
		t.Error("the with-open-tasks filter listed a project whose every task is finished")
	}

	hub.Update(syntheticCtrlKey('t'))
	if hub.taskFilter != projectHubTaskFilterWithoutOpenTasks {
		t.Fatalf("second Ctrl+T left filter=%v, want projects without open tasks", hub.taskFilter)
	}
	if !projectHubHasItem(hub, "project-hub:project:finished") {
		t.Error("the without-open-tasks filter hid a project whose every task is finished")
	}
}

func TestProjectHubCountsProjectWithoutTasksFileAsZero(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	fixture.add(scopeTestProject("empty", "Empty"))

	hub := fixture.open()
	if got := projectHubItem(t, hub, "project-hub:project:empty").SecondaryText; got != "0 tasks" {
		t.Fatalf("empty project row secondary = %q, want %q", got, "0 tasks")
	}
}

func TestProjectHubKeepsUnknownCountForUnreadableProject(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	// A project the count pass could not read is absent from the tallies, which
	// is the whole of what the Hub sees of an unparseable tasks.md.
	fixture.addUncounted(scopeTestProject("corrupt", "Corrupt"))

	hub := fixture.open()
	if got := projectHubItem(t, hub, "project-hub:project:corrupt").SecondaryText; got != "tasks not loaded" {
		t.Fatalf("unreadable project row secondary = %q, want %q", got, "tasks not loaded")
	}
	if hub.notice != "" {
		t.Fatalf("unreadable project raised notice %q, want the Hub silent", hub.notice)
	}
}

// TestProjectTaskTallyCountsNoHiddenWork holds the Hub's numbers to the preview
// beside them: hidden work, stored or inherited, adds to neither count nor the
// overdue tally, while Tasks keeps the whole tree for inheritance to resolve.
func TestProjectTaskTallyCountsNoHiddenWork(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -3)
	project := scopeTestProject("alpha", "Alpha", "1", "2", "3")
	shown, root, child := project.Tasks[0], project.Tasks[1], project.Tasks[2]
	root.Hidden = true
	rootID := root.ID
	child.ParentID = &rootID
	root.Children = []*models.Task{child}
	root.DueDate, child.DueDate = &past, &past

	tally := ProjectTaskTallyOf(project)
	if tally.Open != 1 || tally.Total != 1 || len(tally.Tasks) != 3 {
		t.Fatalf("tally = open %d total %d tasks %d, want 1/1 over all 3", tally.Open, tally.Total, len(tally.Tasks))
	}
	if got := tally.OverdueAt(now); got != 0 {
		t.Fatalf("overdue = %d, want hidden deadlines uncounted", got)
	}
	shown.DueDate = &past
	if got := tally.OverdueAt(now); got != 1 {
		t.Fatalf("overdue = %d, want the visible task alone", got)
	}
}
