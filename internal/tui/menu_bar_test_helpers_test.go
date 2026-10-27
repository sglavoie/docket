package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// The menu-bar suite's shared fixtures and driver helpers. They live in a file
// that declares no test because five other files read them: a helper is resolved
// by package rather than by file, so keeping the fan-out legible from the
// filename is what makes moving one an obvious unit of work.

// productionInputStrategies is every grammar a key can be pressed in. An
// operation absent from all five has no key anywhere, which is what "menu-only"
// means; reading them off surfaceOperationGates would only name the four entity
// ones plus the Outliner, in a shape built for a different question.
var productionInputStrategies = []operation.InputStrategyID{
	operation.InputStrategySimplified,
	operation.InputStrategyProjectHub,
	operation.InputStrategyNotes,
	operation.InputStrategyAttachments,
	operation.InputStrategyComments,
}

// menuBarCommandContext is the launch context: the Outliner presentation with
// the simplified grammar and a cursor that names nothing.
func menuBarCommandContext() operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	return ctx
}

// resolveOutlinerMenus projects the production bar against the production
// catalog, which is what makes these assertions audit the shipped specs rather
// than a test-local subset of them.
func resolveOutlinerMenus(t *testing.T, app *AppModel, activeMode chrome.OutlinerModeID) []outliner.ResolvedMenu {
	t.Helper()
	strategy, ok := app.GetOperationRegistry().InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	menus, err := outliner.ResolveMenuEntries(
		app.GetOperationRegistry(),
		strategy,
		menuBarCommandContext(),
		outliner.OutlinerMenuSpecs(activeMode, false, true, true),
	)
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}
	return menus
}

func menuWithID(t *testing.T, menus []outliner.ResolvedMenu, id string) outliner.ResolvedMenu {
	t.Helper()
	for _, menu := range menus {
		if menu.ID == id {
			return menu
		}
	}
	t.Fatalf("menu %s is missing from the bar", id)
	return outliner.ResolvedMenu{}
}

// menuBarDriver launches the rich fixture with the bar hidden, which is where
// every open case starts: revealing the band is part of what opening it means.
func menuBarDriver(t *testing.T) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	outlinerView := driver.App().crossOutliner
	if outlinerView == nil {
		t.Fatal("the rich fixture launched without a cross-project Outliner")
	}
	if driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Fatal("the fixture launched with the bar already shown, so opening it proves nothing")
	}
	return driver, outlinerView
}

// pressMenuBarKey sends a key and drains what it returned. A menu key that only
// moves the highlight returns nothing, but Enter and the non-modal fall-through
// both hand the shell a message to run, so a test that skipped the drain would
// be asserting against half of the press.
func pressMenuBarKey(t *testing.T, driver *UITestDriver, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("press %s: %v", key, err)
		}
		driver.drain()
	}
}

// openMenuBarMenusForTest reads the resolution the dropdown and the keymap share,
// which is the only honest way to ask what an open menu is offering: the specs
// are not it, because filtering happens between them.
func openMenuBarMenusForTest(t *testing.T, outlinerView *outliner.OutlinerModel) []outliner.ResolvedMenu {
	t.Helper()
	menus, ok := outlinerView.OpenMenuBarMenus()
	if !ok {
		t.Fatal("no menu is open")
	}
	return menus
}

func menuHasLabel(menu outliner.ResolvedMenu, label string) bool {
	for _, entry := range menu.Entries {
		if entry.OperationID != "" && entry.Label == label {
			return true
		}
	}
	return false
}
