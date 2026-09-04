package tui

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/route"
)

func TestProjectHubListsEveryProjectByLifecycle(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Slug = "archived"
	archived.Archived = true
	archived.Path = t.TempDir()
	addTestAppProject(app, archived)
	app.projects["alpha"].Pinned = true
	setTestAppProjects(app, app.projects["alpha"], app.projects["beta"], app.projects["parked"], app.projects["archived"])

	hub := openProjectHub(t, app)
	wantCategories := []listsurface.ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
	if got := hub.surface.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("project categories = %#v, want %#v", got, wantCategories)
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
	app := projectHubApp(t)
	alpha := app.projects["alpha"]
	alpha.Pinned = true
	parked := scopeTestProject("parked", "Parked")
	parked.Pinned = true
	parked.Tags = []string{"parked"}
	parked.Path = t.TempDir()
	addTestAppProject(app, parked)
	archived := scopeTestProject("archived", "Archived")
	archived.Pinned = true
	archived.Archived = true
	archived.Path = t.TempDir()
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app, route.ProjectHubModePinned)
	if hub.Mode() != route.ProjectHubModePinned {
		t.Fatalf("Hub mode = %v, want pinned", hub.Mode())
	}
	if params := app.CurrentRoute().Params().(route.ManageParams); params.ProjectHubMode != route.ProjectHubModePinned {
		t.Fatalf("pinned Hub route params = %+v", params)
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
	app := projectHubFilterApp(t)
	app.projects["beta"].Pinned = true
	app.projects["empty"].Pinned = true
	app.projects["alpha"].Pinned = false
	setTestAppProjects(app, app.projects["alpha"], app.projects["beta"], app.projects["empty"])
	hub := openProjectHub(t, app, route.ProjectHubModePinned)

	hub.SetSearchQuery("bet")
	hub.applyProjectFilter()
	if got := len(hub.surface.Matches()); got != 2 {
		t.Fatalf("pinned search matches = %d, want Beta and Beta Empty", got)
	}
	hub.cycleProjectHubTaskFilter()
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
	app := projectHubApp(t)
	alpha := app.projects["alpha"]
	alpha.Pinned = true
	setTestAppProjects(app, app.projects["alpha"], app.projects["beta"])
	hub := openProjectHub(t, app, route.ProjectHubModePinned)
	focusProjectHubItem(t, hub, "project-hub:project:alpha")
	hub.toggleProjectHubSelection()

	alpha.Pinned = false
	setTestAppProjects(app, app.projects["alpha"], app.projects["beta"])
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
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 3)
	addTestAppProject(app, parked)
	archived := projectHubDiskProject(app, "archived", "Archived")
	archived.Archived = true
	writeProjectHubTasks(t, archived, 2)
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:parked").SecondaryText; got != "3 open · 3 tasks · #parked" {
		t.Fatalf("parked row secondary = %q, want the real count", got)
	}
	want := "Archived · 2 open · 2 tasks · slug archived"
	if got := projectHubPreviewSummary(t, hub, "project-hub:archived:archived"); got != want {
		t.Fatalf("archived preview meta = %q, want %q", got, want)
	}
}

func TestProjectHubCountsOpenTasksBesideTotal(t *testing.T) {
	app := projectHubDiskApp(t)
	mixed := projectHubDiskProject(app, "mixed", "Mixed")
	writeProjectHubTasks(t, mixed, 3)
	finishProjectHubTasks(t, mixed, 2)
	addTestAppProject(app, mixed)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:mixed").SecondaryText; got != "1 open · 3 tasks" {
		t.Fatalf("partly finished row secondary = %q, want %q", got, "1 open · 3 tasks")
	}
	want := "Active · 1 open · 3 tasks · slug mixed"
	if got := projectHubPreviewSummary(t, hub, "project-hub:project:mixed"); got != want {
		t.Fatalf("partly finished preview meta = %q, want %q", got, want)
	}
}

func TestProjectHubHidesAllFinishedProjectUnderTheOpenTaskFilter(t *testing.T) {
	app := projectHubDiskApp(t)
	finished := projectHubDiskProject(app, "finished", "Finished")
	writeProjectHubTasks(t, finished, 2)
	finishProjectHubTasks(t, finished, 2)
	addTestAppProject(app, finished)

	hub := openProjectHub(t, app)
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
	app := projectHubDiskApp(t)
	empty := projectHubDiskProject(app, "empty", "Empty")
	if err := os.MkdirAll(empty.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", empty.Path, err)
	}
	addTestAppProject(app, empty)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:empty").SecondaryText; got != "0 tasks" {
		t.Fatalf("empty project row secondary = %q, want %q", got, "0 tasks")
	}
}

func TestProjectHubKeepsUnknownCountForUnreadableProject(t *testing.T) {
	app := projectHubDiskApp(t)
	corrupt := projectHubDiskProject(app, "corrupt", "Corrupt")
	if err := os.MkdirAll(corrupt.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", corrupt.Path, err)
	}
	tasksPath := storage.TasksFilePath(corrupt.Path)
	if err := os.WriteFile(tasksPath, []byte("this is not a task document\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", tasksPath, err)
	}
	addTestAppProject(app, corrupt)

	hub := openProjectHub(t, app)
	if got := projectHubItem(t, hub, "project-hub:project:corrupt").SecondaryText; got != "tasks not loaded" {
		t.Fatalf("unreadable project row secondary = %q, want %q", got, "tasks not loaded")
	}
	if hub.notice != "" {
		t.Fatalf("unreadable project raised notice %q, want the Hub silent", hub.notice)
	}
}

func TestProjectHubReusesOutlinerLoaderAcrossOpens(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 1)
	addTestAppProject(app, parked)

	loader := app.crossOutliner.projectService.WorkspaceLoader()
	before := loader.ParseCount()
	openProjectHub(t, app)
	first := loader.ParseCount()
	if first == before {
		t.Fatalf("first Hub open parsed nothing, parse count stayed %d", first)
	}
	openProjectHub(t, app)
	if second := loader.ParseCount(); second != first {
		t.Fatalf("second Hub open parse count = %d, want %d", second, first)
	}
}
