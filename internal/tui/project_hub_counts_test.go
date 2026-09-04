package tui

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/route"
)

// How the Hub draws a tally moved into internal/tui/projecthub with the surface.
// What stays here is the shell half: the count pass that reads every cataloged
// project off disk, the loader it reuses between opens, and the route the pinned
// mode records.

func TestProjectHubTaskCountsCoverEveryCatalogedProject(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 3)
	addTestAppProject(app, parked)
	archived := projectHubDiskProject(app, "archived", "Archived")
	archived.Archived = true
	writeProjectHubTasks(t, archived, 2)
	addTestAppProject(app, archived)
	mixed := projectHubDiskProject(app, "mixed", "Mixed")
	writeProjectHubTasks(t, mixed, 3)
	finishProjectHubTasks(t, mixed, 2)
	addTestAppProject(app, mixed)
	empty := projectHubDiskProject(app, "empty", "Empty")
	if err := os.MkdirAll(empty.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", empty.Path, err)
	}
	addTestAppProject(app, empty)

	counts := app.projectHubTaskCounts()
	for _, want := range []struct {
		id          string
		open, total int
	}{
		{"parked", 3, 3},
		{"archived", 2, 2},
		{"mixed", 1, 3},
		{"empty", 0, 0},
	} {
		tally, ok := counts[want.id]
		if !ok {
			t.Errorf("count pass skipped %q", want.id)
			continue
		}
		if tally.Open != want.open || tally.Total != want.total {
			t.Errorf("%q tally = %d open of %d, want %d of %d", want.id, tally.Open, tally.Total, want.open, want.total)
		}
	}
}

// A project the pass cannot parse is absent from the map rather than counted
// zero, which is what lets the Hub say "tasks not loaded" instead of lying.
func TestProjectHubTaskCountsOmitAnUnreadableProject(t *testing.T) {
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

	counts := app.projectHubTaskCounts()
	if tally, ok := counts["corrupt"]; ok {
		t.Fatalf("unreadable project counted %#v, want no entry at all", tally)
	}
	if _, ok := counts["alpha"]; !ok {
		t.Fatal("one unreadable project dropped the rest of the count pass")
	}
}

func TestPinnedProjectHubRecordsItsModeOnTheRoute(t *testing.T) {
	app := projectHubApp(t)
	app.domain.projects["alpha"].Pinned = true

	hub := openProjectHub(t, app, route.ProjectHubModePinned)
	if hub.Mode() != route.ProjectHubModePinned {
		t.Fatalf("Hub mode = %v, want pinned", hub.Mode())
	}
	params, ok := app.CurrentRoute().Params().(route.ManageParams)
	if !ok || params.ProjectHubMode != route.ProjectHubModePinned {
		t.Fatalf("pinned Hub route params = %#v", app.CurrentRoute().Params())
	}
}

func TestProjectHubReusesOutlinerLoaderAcrossOpens(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 1)
	addTestAppProject(app, parked)

	loader := app.crossOutliner.ProjectService.WorkspaceLoader()
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
