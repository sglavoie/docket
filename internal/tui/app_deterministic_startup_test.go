package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

//nolint:govet // Commit and reload stages intentionally use local error scopes.
func TestLaunchResetsEveryPresentationAxisAndRelaunchPreservesOnlyTheme(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	app := driver.App()
	outlinerView := app.crossOutliner
	projects := app.taskWorkspaceProjects()
	if len(projects) == 0 {
		t.Fatal("rich fixture has no active project")
	}

	// Seed every launch-sensitive axis with stale in-session state.
	outlinerView.SelectOutlinerMode(chrome.ModeDue)
	outlinerView.SetSearchActive(true)
	outlinerView.SetSearchQuery("stale search")
	outlinerView.Scope.ProjectID = projects[0].ID
	outlinerView.Scope.Stack = []outliner.OutlinerScopeFrame{{}}
	outlinerView.Filter.Priority = 3
	outlinerView.Filter.Tags = map[string]bool{"stale": true}
	adoptFilterCarriers(outlinerView)
	outlinerView.Cursor = 7
	outlinerView.Folds.CollapsedSections["stale"] = true
	outlinerView.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: projects[0].ID, TaskID: "stale"}] = true
	outlinerView.MultiSelectMode = true
	outlinerView.Selected[taskref.TaskIdentity{ProjectID: projects[0].ID, TaskID: "stale"}] = true
	app.currentProject = projects[0]
	pushTestPresentationOverlay(app, presentation.ViewHelp)

	// A lens is machine-local like the theme, but only its definitions are
	// durable: the frame the user was browsing under is in-session state exactly
	// like the axes above it.
	saved := config.StarterNamedLenses()[0]
	saved.ID = deterministicStartupLensID
	saved.Name = "Persisted Lens"
	outlinerView.PublishLens(saved, outlinerView.CaptureLensState(), nil, outlinerView.LensRestorationPoint())
	if !outlinerView.HasActiveLens() {
		t.Fatal("seeding failed to install an active lens")
	}

	machine := app.domain.runtime.Machine()
	machine.Theme = "light"
	machine.NamedLenses = append(config.StarterNamedLenses(), saved)
	if err := app.domain.runtime.CommitMachine(machine); err != nil {
		t.Fatalf("persist theme and lenses: %v", err)
	}

	driver.dispatch(launchOutlinerMsg{})
	assertCleanLaunchRoot(t, app)
	if got := app.domain.runtime.Machine().Theme; got != "light" {
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
	if got := relaunched.domain.runtime.Machine().Theme; got != "light" {
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
	lenses := app.domain.runtime.Machine().NamedLenses
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
	if app.currentView() != presentation.ViewCrossProjectOutliner || app.viewNav.navigation.Primary.Depth() != 1 || app.viewNav.navigation.Overlays.Active() {
		t.Fatalf("launch navigation = view %s depth %d overlay %v", app.currentView(), app.viewNav.navigation.Primary.Depth(), app.viewNav.navigation.Overlays.Active())
	}
	if app.currentProject != nil || len(app.projectStack) != 0 {
		t.Fatalf("launch retained project state: current=%v stack=%v", app.currentProject, app.projectStack)
	}
	outlinerView := app.crossOutliner
	if outlinerView == nil {
		t.Fatal("launch has no global Outliner")
	}
	if got := outlinerView.ActiveOutlinerMode(); got != chrome.ModeAll {
		t.Fatalf("launch mode = %q, want %q", got, chrome.ModeAll)
	}
	if outlinerView.Scope.ProjectID != "" || len(outlinerView.Scope.Stack) != 0 {
		t.Fatalf("launch retained scope: project=%q stack=%d", outlinerView.Scope.ProjectID, len(outlinerView.Scope.Stack))
	}
	if outlinerView.HasActiveSearch() || outlinerView.SearchQuery() != "" || outlinerView.ActiveFilterCount() != 0 {
		t.Fatalf("launch retained search/filter: active=%v query=%q filters=%d", outlinerView.HasActiveSearch(), outlinerView.SearchQuery(), outlinerView.ActiveFilterCount())
	}
	if outlinerView.Cursor != 0 || outlinerView.IsMultiSelectActive() || outlinerView.SelectedCount() != 0 {
		t.Fatalf("launch retained cursor/selection: cursor=%d active=%v selected=%d", outlinerView.Cursor, outlinerView.IsMultiSelectActive(), outlinerView.SelectedCount())
	}
	if outlinerView.HasActiveLens() || outlinerView.Lens.Query != "" || outlinerView.EditingLensQuery() {
		t.Fatalf("launch retained lens state: active=%v query=%q editing=%v", outlinerView.HasActiveLens(), outlinerView.Lens.Query, outlinerView.EditingLensQuery())
	}
	if len(outlinerView.Folds.CollapsedSections) != 0 || len(outlinerView.Folds.ExpandedNodes) != 0 || len(outlinerView.Focus.Stack) != 0 || len(outlinerView.Focus.CrossStack) != 0 {
		t.Fatalf("launch retained folds/focus: sections=%v nodes=%v focus=%v crossFocus=%v", outlinerView.Folds.CollapsedSections, outlinerView.Folds.ExpandedNodes, outlinerView.Focus.Stack, outlinerView.Focus.CrossStack)
	}
}
