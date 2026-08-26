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

	// A lens is machine-local like the theme, but only its definitions are
	// durable: the frame the user was browsing under is in-session state exactly
	// like the axes above it.
	saved := config.StarterNamedLenses()[0]
	saved.ID = deterministicStartupLensID
	saved.Name = "Persisted Lens"
	outliner.publishLens(saved, outliner.captureLensState(), nil, outliner.lensRestorationPoint())
	if !outliner.hasActiveLens() {
		t.Fatal("seeding failed to install an active lens")
	}

	machine := app.runtime.Machine()
	machine.Theme = "light"
	machine.NamedLenses = append(config.StarterNamedLenses(), saved)
	if err := app.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("persist theme and lenses: %v", err)
	}

	driver.dispatch(launchOutlinerMsg{})
	assertCleanLaunchRoot(t, app)
	if got := app.runtime.Machine().Theme; got != "light" {
		t.Fatalf("theme after launch reset = %q, want light", got)
	}
	assertPersistedLensDefinitions(t, app)

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
	assertPersistedLensDefinitions(t, relaunched)
}

// deterministicStartupLensID is the one saved lens this test writes beside the
// three starters. It is fixed so the reloaded process names the same record, and
// it deliberately sits outside the frozen `6f1a0c4e-…` built-in catalog prefix:
// a saved record wearing a template's stable ID would read as that template
// already added, which is a different claim than the one this test makes.
const deterministicStartupLensID = "4b7d2c19-8e35-4f60-9a11-7c2e5d840b3f"

// assertPersistedLensDefinitions is the other half of the launch contract: the
// saved records survive, and nothing about the frame the user left does.
func assertPersistedLensDefinitions(t *testing.T, app *AppModel) {
	t.Helper()
	lenses := app.runtime.Machine().NamedLenses
	if len(lenses) != len(config.StarterNamedLenses())+1 {
		t.Fatalf("launch kept %d lens definitions, want the three starters plus one saved", len(lenses))
	}
	found := false
	for _, lens := range lenses {
		if lens.ID == deterministicStartupLensID {
			found = true
			if lens.Name != "Persisted Lens" {
				t.Fatalf("saved lens name = %q, want Persisted Lens", lens.Name)
			}
		}
	}
	if !found {
		t.Fatalf("launch dropped the saved lens %s", deterministicStartupLensID)
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
	if outliner.scopedProjectID != "" || len(outliner.scopeStack) != 0 {
		t.Fatalf("launch retained scope: project=%q stack=%d", outliner.scopedProjectID, len(outliner.scopeStack))
	}
	if outliner.HasActiveSearch() || outliner.SearchQuery() != "" || outliner.ActiveFilterCount() != 0 {
		t.Fatalf("launch retained search/filter: active=%v query=%q filters=%d", outliner.HasActiveSearch(), outliner.SearchQuery(), outliner.ActiveFilterCount())
	}
	if outliner.cursor != 0 || outliner.IsMultiSelectActive() || outliner.SelectedCount() != 0 {
		t.Fatalf("launch retained cursor/selection: cursor=%d active=%v selected=%d", outliner.cursor, outliner.IsMultiSelectActive(), outliner.SelectedCount())
	}
	if outliner.hasActiveLens() || outliner.lensQuery != "" || outliner.editingLensQuery() {
		t.Fatalf("launch retained lens state: active=%v query=%q editing=%v", outliner.hasActiveLens(), outliner.lensQuery, outliner.editingLensQuery())
	}
	if len(outliner.collapsedSections) != 0 || len(outliner.expandedNodes) != 0 || len(outliner.focusStack) != 0 || len(outliner.crossFocusStack) != 0 {
		t.Fatalf("launch retained folds/focus: sections=%v nodes=%v focus=%v crossFocus=%v", outliner.collapsedSections, outliner.expandedNodes, outliner.focusStack, outliner.crossFocusStack)
	}
}
