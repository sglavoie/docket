package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

//nolint:govet // Commit and reload stages intentionally use local error scopes.
func TestLaunchResetsEveryPresentationAxisAndRelaunchPreservesOnlyTheme(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	app := driver.App()
	outliner := app.crossOutliner
	projects := app.taskWorkspaceProjects()
	if len(projects) == 0 {
		t.Fatal("rich fixture has no active project")
	}

	// Seed every launch-sensitive axis with stale in-session state.
	outliner.selectOutlinerMode(ModeDue)
	outliner.SetSearchActive(true)
	outliner.SetSearchQuery("stale search")
	outliner.activeContext = RegularContextScope(projects[0].Context)
	outliner.scopedProjectID = projects[0].ID
	outliner.scopeStack = []outlinerScopeFrame{{}}
	outliner.filter.Priority = 3
	outliner.filter.Tags = map[string]bool{"stale": true}
	outliner.cursor = 7
	outliner.collapsedSections["stale"] = true
	outliner.expandedNodes[TaskIdentity{ProjectID: projects[0].ID, TaskID: "stale"}] = true
	outliner.multiSelectMode = true
	outliner.selectedTaskIdentities[TaskIdentity{ProjectID: projects[0].ID, TaskID: "stale"}] = true
	app.currentProject = projects[0]
	pushTestPresentationOverlay(app, ViewHelp)

	machine := app.runtime.Machine()
	machine.Theme = "light"
	if err := app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("persist theme: %v", err)
	}

	driver.dispatch(launchOutlinerMsg{})
	assertCleanLaunchRoot(t, app)
	if got := app.runtime.Machine().Theme; got != "light" {
		t.Fatalf("theme after launch reset = %q, want light", got)
	}

	reloaded, err := config.LoadRuntimeState(workspace.DataDir)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	relaunched, err := NewAppModel(reloaded, config.NewManager(workspace.ConfigPath))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	relaunchDriver := NewUITestDriver(relaunched, 100, 35)
	relaunchDriver.dispatch(launchOutlinerMsg{})
	assertCleanLaunchRoot(t, relaunched)
	if got := relaunched.runtime.Machine().Theme; got != "light" {
		t.Fatalf("theme after process relaunch = %q, want light", got)
	}
}

func assertCleanLaunchRoot(t *testing.T, app *AppModel) {
	t.Helper()
	if app.currentView() != ViewCrossProjectOutliner || app.navigation.Primary.Depth() != 1 || app.navigation.Overlays.Active() {
		t.Fatalf("launch navigation = view %s depth %d overlay %v", app.currentView(), app.navigation.Primary.Depth(), app.navigation.Overlays.Active())
	}
	if app.currentProject != nil || len(app.projectStack) != 0 {
		t.Fatalf("launch retained project state: current=%v stack=%v", app.currentProject, app.projectStack)
	}
	outliner := app.crossOutliner
	if outliner == nil {
		t.Fatal("launch has no global Outliner")
	}
	if got := outliner.activeOutlinerMode(); got != ModeAll {
		t.Fatalf("launch mode = %q, want %q", got, ModeAll)
	}
	if outliner.scopedProjectID != "" || len(outliner.scopeStack) != 0 || !outliner.activeContext.IsAll() {
		t.Fatalf("launch retained scope: project=%q stack=%d context=%+v", outliner.scopedProjectID, len(outliner.scopeStack), outliner.activeContext)
	}
	if outliner.HasActiveSearch() || outliner.SearchQuery() != "" || outliner.ActiveFilterCount() != 0 {
		t.Fatalf("launch retained search/filter: active=%v query=%q filters=%d", outliner.HasActiveSearch(), outliner.SearchQuery(), outliner.ActiveFilterCount())
	}
	if outliner.cursor != 0 || outliner.IsMultiSelectActive() || outliner.SelectedCount() != 0 {
		t.Fatalf("launch retained cursor/selection: cursor=%d active=%v selected=%d", outliner.cursor, outliner.IsMultiSelectActive(), outliner.SelectedCount())
	}
	if len(outliner.collapsedSections) != 0 || len(outliner.expandedNodes) != 0 || len(outliner.zoomStack) != 0 || len(outliner.crossZoomStack) != 0 {
		t.Fatalf("launch retained folds/zoom: sections=%v nodes=%v zoom=%v crossZoom=%v", outliner.collapsedSections, outliner.expandedNodes, outliner.zoomStack, outliner.crossZoomStack)
	}
}
