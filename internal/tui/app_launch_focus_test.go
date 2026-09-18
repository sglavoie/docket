package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// launchCursorRowKind names what the launch rule landed on, so a failure reads
// as the contract rather than as a row index.
func launchCursorRowKind(t *testing.T, view *outliner.OutlinerModel) string {
	t.Helper()
	if view == nil || len(view.Rows) == 0 {
		return "no rows"
	}
	if view.Cursor < 0 || view.Cursor >= len(view.Rows) {
		t.Fatalf("launch cursor %d is outside the %d rows it was measured against", view.Cursor, len(view.Rows))
	}
	row := view.Rows[view.Cursor]
	switch {
	case row.IsTaskRow() && row.Task != nil:
		return "task"
	case row.IsProjectHeader:
		return "project header"
	default:
		return "heading"
	}
}

// firstTaskRow is the row the launch rule is supposed to find: the first task
// the settled projection draws, independent of how the projection groups it.
func firstTaskRow(view *outliner.OutlinerModel) (int, bool) {
	for index, row := range view.Rows {
		if row.IsTaskRow() && row.Task != nil {
			return index, true
		}
	}
	return 0, false
}

// A real startup lands on work, not on the heading above it. The fixture driver
// performs the whole boot — the launch message, the population read it starts,
// and the commands that read settles — so this is the startup a user sees.
func TestLaunchSeatsTheCursorOnTheFirstVisibleTask(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	view := driver.App().crossOutliner
	wanted, ok := firstTaskRow(view)
	if !ok {
		t.Fatal("the rich fixture's launch projection draws no task")
	}
	if wanted == 0 {
		t.Fatal("the rich fixture opens on a task row, so this proves nothing about the rule")
	}
	if view.Cursor != wanted {
		t.Fatalf("launch cursor = %d (%s), want the first task row %d",
			view.Cursor, launchCursorRowKind(t, view), wanted)
	}
}

// A projection with nothing in it is not a projection that failed to load, and
// the rule must still resolve: with no task to find, the cursor keeps the row
// the ordinary clamp leaves rather than staying owed forever.
func TestLaunchFallsBackToTheFirstHeadingWhenNoTaskIsVisible(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureEmpty, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	app := driver.App()
	if app.lifecycle.launchFocusArmed() {
		t.Fatal("an empty workspace left the launch intent owed, so a later reload would still spend it")
	}
	if got := app.crossOutliner.Cursor; got != 0 {
		t.Fatalf("launch cursor over a task-less projection = %d, want the first row", got)
	}
}

// The intent is spent by the boot it was armed for and by nothing else: a later
// reload of the same population must leave the cursor where the user put it.
func TestLaunchFocusIsNotReappliedByALaterReload(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	app := driver.App()
	view := app.crossOutliner
	if app.lifecycle.launchFocusArmed() {
		t.Fatal("the launch intent survived the boot that spent it")
	}

	// Park the cursor above the first task, where only the launch rule would
	// have moved it, then reload the very population the rule read.
	view.Cursor = 0
	driver.dispatch(view.ReloadCrossProjects()())
	driver.drain()

	if got := view.Cursor; got != 0 {
		t.Fatalf("a later reload moved the cursor to %d, reapplying the startup rule", got)
	}
}

// A saved lens owns the axes the launch rule then answers for, so the rule is
// spent after the lens has published rather than against the base workspace it
// replaced.
func TestLaunchFocusWaitsForASavedLensToPublish(t *testing.T) {
	app := launchLensApp(t)
	app.lifecycle.armLaunchFocus()
	lens := scopedWaitingLens(lensTestID(1), "Beta Waiting", lensScopeBetaID)
	app.lenses.SetPendingLaunch(&lens)

	// A population still reading is not one the rule may answer for.
	app.crossOutliner.Population.SnapshotReady = false
	app.crossOutliner.Population.Loading = true
	app.settleLaunchFocus()
	if !app.lifecycle.launchFocusArmed() {
		t.Fatal("the launch intent was spent against a population that had not landed")
	}

	app.crossOutliner.Population.Loading = false
	app.crossOutliner.Population.SnapshotReady = true
	app.restorePendingLaunchLens()
	app.settleLaunchFocus()

	if app.lifecycle.launchFocusArmed() {
		t.Fatal("a settled population left the launch intent owed")
	}
	if app.crossOutliner.Scope.ProjectID != lensScopeBetaID {
		t.Fatalf("the launch rule ran against scope %q rather than the lens's own %q",
			app.crossOutliner.Scope.ProjectID, lensScopeBetaID)
	}
}

// A superseded read publishes nothing, so it may not be mistaken for the
// accepted population: the intent stays owed for the load that still owns the
// launch.
func TestLaunchFocusIgnoresASupersededLoad(t *testing.T) {
	app := launchLensApp(t)
	app.lifecycle.armLaunchFocus()
	app.crossOutliner.Population.SnapshotReady = false
	app.crossOutliner.Population.Loading = true
	app.crossOutliner.Population.LoadGen = 7

	handled, _ := app.handleGlobalLifecycleMessage(outliner.CrossProjectsLoadedMsg{Generation: 3})
	if !handled {
		t.Fatal("the shell did not route the cross-project result")
	}
	if !app.lifecycle.launchFocusArmed() {
		t.Fatal("a superseded load spent the launch intent")
	}
}

// The first keystroke is the user taking the cursor. A startup that completes
// after it finds nothing to apply, so the rule never overrides newer navigation;
// a bare modifier is not a keystroke and costs nothing.
func TestLaunchFocusYieldsToNewerNavigation(t *testing.T) {
	app := launchLensApp(t)
	app.lifecycle.armLaunchFocus()

	app.handleGlobalInputMessage(tea.KeyPressMsg{Code: tea.KeyLeftShift, Mod: tea.ModShift})
	if !app.lifecycle.launchFocusArmed() {
		t.Fatal("a bare modifier abandoned the launch intent")
	}

	app.handleGlobalInputMessage(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if app.lifecycle.launchFocusArmed() {
		t.Fatal("a keystroke left the launch intent owed, so a late load would still move the cursor")
	}

	app.crossOutliner.Population.SnapshotReady = true
	app.crossOutliner.Population.Loading = false
	app.crossOutliner.Cursor = 0
	app.settleLaunchFocus()
	if app.crossOutliner.Cursor != 0 {
		t.Fatalf("a load landing after a keystroke moved the cursor to %d", app.crossOutliner.Cursor)
	}
}

// LaunchFocusSettled is the whole distinction between a loading projection and
// an empty one, and neither half implies the other.
func TestLaunchFocusSettledSeparatesLoadingFromEmpty(t *testing.T) {
	cold := scopeTestOutliner()
	cold.Population.Loading = true
	if cold.LaunchFocusSettled() {
		t.Error("a cold read counted as a settled population")
	}

	refreshing := scopeTestOutliner(lensScopeProject(lensScopeAlphaID))
	refreshing.Population.SnapshotReady = true
	refreshing.Population.Loading = true
	if refreshing.LaunchFocusSettled() {
		t.Error("a population reading its replacement counted as settled")
	}

	empty := scopeTestOutliner()
	empty.Population.SnapshotReady = true
	if !empty.LaunchFocusSettled() {
		t.Error("an accepted read of an empty workspace was treated as still loading")
	}

	var absent *outliner.OutlinerModel
	if absent.LaunchFocusSettled() {
		t.Error("an absent Outliner reported a settled population")
	}
}

// FocusFirstLaunchRow writes the cursor and nothing else: the window stays
// ComputeOutlinerViewport's, and the folds, mode, and scope stay as found.
func TestFocusFirstLaunchRowWritesTheCursorAlone(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	view := driver.App().crossOutliner
	mode := view.ActiveOutlinerMode()
	scope := view.Scope.ProjectID
	folds := len(view.Folds.CollapsedSections)
	offset := view.ViewScrollOffset

	view.Cursor = 0
	if !view.FocusFirstLaunchRow() {
		t.Fatal("the rich fixture's projection reported no task to seat the cursor on")
	}
	if got := view.ActiveOutlinerMode(); got != mode {
		t.Fatalf("the launch rule changed the mode to %q", got)
	}
	if view.Scope.ProjectID != scope {
		t.Fatalf("the launch rule changed the scope to %q", view.Scope.ProjectID)
	}
	if len(view.Folds.CollapsedSections) != folds {
		t.Fatal("the launch rule opened or closed a fold")
	}
	if view.ViewScrollOffset != offset {
		t.Fatalf("the launch rule moved the window to %d", view.ViewScrollOffset)
	}
	if got := launchCursorRowKind(t, view); got != "task" {
		t.Fatalf("the launch rule seated the cursor on a %s", got)
	}
}

// A projection drawing no task at all still resolves, and a projection with no
// rows leaves the cursor alone rather than inventing a position.
func TestFocusFirstLaunchRowFallsBackWithoutATask(t *testing.T) {
	view := scopeTestOutliner()
	if view.FocusFirstLaunchRow() {
		t.Fatal("an unbuilt projection claimed to have found a task")
	}

	mode := chrome.ModeAll
	if got := view.ActiveOutlinerMode(); got != mode {
		t.Fatalf("the fallback changed the mode to %q", got)
	}
}

// The saved-lens pointer is machine-local, and a launch reading one still owes
// the cursor the same starting position a lens-free launch does.
func TestLaunchWithASavedLensStillSeatsTheCursorOnWork(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	app := driver.App()
	machine := app.domain.runtime.Machine()
	starters := config.StarterNamedLenses()
	if len(starters) == 0 {
		t.Fatal("a fresh machine seeds no starter lenses")
	}
	machine.NamedLenses = starters
	machine.ActiveLensID = starters[0].ID
	if commitErr := app.domain.runtime.CommitMachine(machine); commitErr != nil {
		t.Fatalf("persist the active-lens pointer: %v", commitErr)
	}

	relaunched, err := driver.relaunch()
	if err != nil {
		t.Fatalf("relaunch: %v", err)
	}
	relaunchDriver := NewUITestDriver(relaunched, 100, 35)
	if relaunched.lifecycle.startup != nil {
		relaunchDriver.dispatch(relaunched.lifecycle.startup)
		relaunchDriver.drain()
	}
	relaunchDriver.bootLaunchCommands()

	if relaunched.lifecycle.launchFocusArmed() {
		t.Fatal("a saved-lens launch left the starting position owed")
	}
	view := relaunched.crossOutliner
	if wanted, ok := firstTaskRow(view); ok && view.Cursor != wanted {
		t.Fatalf("saved-lens launch cursor = %d (%s), want the first task row %d",
			view.Cursor, launchCursorRowKind(t, view), wanted)
	}
}
