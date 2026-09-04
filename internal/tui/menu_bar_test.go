package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// The menu bar is a projection of the operation catalog, so these are the
// parity assertions that keep it one: the bar may only name operations the
// registry holds, it may never spell a key the strategy owns, the modes it
// offers are the modes the catalog declares, and an operation the bar is the
// only route to is never an entity operation. A menu that drifts from the
// grammar fails here rather than in front of a user.
//
// This file holds what the suite shares — the fixtures, the driver helpers, and
// the resolution's own unit tests. The parity rules are in
// menu_bar_parity_test.go and the open-menu sub-state in
// menu_bar_navigation_test.go.

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
func resolveOutlinerMenus(t *testing.T, app *AppModel, activeMode chrome.OutlinerModeID) []resolvedMenu {
	t.Helper()
	strategy, ok := app.GetOperationRegistry().InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	menus, err := resolveMenuEntries(
		app.GetOperationRegistry(),
		strategy,
		menuBarCommandContext(),
		outlinerMenuSpecs(activeMode, false, true),
	)
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}
	return menus
}

func menuWithID(t *testing.T, menus []resolvedMenu, id string) resolvedMenu {
	t.Helper()
	for _, menu := range menus {
		if menu.ID == id {
			return menu
		}
	}
	t.Fatalf("menu %s is missing from the bar", id)
	return resolvedMenu{}
}

func registerMenuTestOperation(
	t *testing.T,
	registry *operation.OperationRegistry,
	id operation.OperationID,
	label string,
	applicable bool,
) {
	t.Helper()
	if err := registry.Register(operation.OperationDescriptor{
		ID:                id,
		Label:             label,
		Category:          operation.OperationCategorySystem,
		TargetCardinality: operation.TargetCardinalityNone,
		Risk:              operation.OperationRiskReadOnly,
		Applicability:     func(operation.CommandContext) bool { return applicable },
	}); err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
}

// menuBarDriver launches the rich fixture with the bar hidden, which is where
// every open case starts: revealing the band is part of what opening it means.
func menuBarDriver(t *testing.T) (*UITestDriver, *OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	outliner := driver.App().crossOutliner
	if outliner == nil {
		t.Fatal("the rich fixture launched without a cross-project Outliner")
	}
	if driver.App().runtime.Machine().MenuBarVisible {
		t.Fatal("the fixture launched with the bar already shown, so opening it proves nothing")
	}
	return driver, outliner
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
func openMenuBarMenusForTest(t *testing.T, outliner *OutlinerModel) []resolvedMenu {
	t.Helper()
	menus, ok := outliner.openMenuBarMenus()
	if !ok {
		t.Fatal("no menu is open")
	}
	return menus
}

func menuHasLabel(menu resolvedMenu, label string) bool {
	for _, entry := range menu.Entries {
		if entry.OperationID != "" && entry.Label == label {
			return true
		}
	}
	return false
}

func TestResolveMenuEntriesRefusesAnUnregisteredOperation(t *testing.T) {
	registry := operation.NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.present", "Present", true)

	_, err := resolveMenuEntries(registry, operation.InputStrategy{}, menuBarCommandContext(), []menuSpec{{
		ID:    "test",
		Title: "Test",
		Entries: []menuEntrySpec{
			{OperationID: "test.menu.present"},
			{OperationID: "test.menu.absent"},
		},
	}})
	if err == nil {
		t.Fatal("a menu naming an unregistered operation resolved without an error")
	}
}

func TestResolveMenuEntriesRemovesRatherThanGreys(t *testing.T) {
	registry := operation.NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.here", "Here", true)
	registerMenuTestOperation(t, registry, "test.menu.gone", "Gone", false)

	menus, err := resolveMenuEntries(registry, operation.InputStrategy{}, menuBarCommandContext(), []menuSpec{
		{ID: "mixed", Title: "Mixed", Entries: []menuEntrySpec{
			{OperationID: "test.menu.here"},
			menuSeparator(),
			{OperationID: "test.menu.gone"},
		}},
		{ID: "empty", Title: "Empty", Entries: []menuEntrySpec{
			menuSeparator(),
			{OperationID: "test.menu.gone"},
			menuSeparator(),
		}},
	})
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}

	if len(menus) != 1 || menus[0].ID != "mixed" {
		t.Fatalf("resolved %d menus, want the one menu that kept an entry", len(menus))
	}
	if len(menus[0].Entries) != 1 || menus[0].Entries[0].Label != "Here" {
		t.Fatalf("mixed menu resolved to %+v, want the applicable entry alone", menus[0].Entries)
	}
}

// The specs carry live state, so a resolution that cached would show a stale
// checkmark the moment the mode changed under it.
func TestResolveMenuEntriesRebuildsOnEveryCall(t *testing.T) {
	app := parityAuditApp(t)

	inbox := menuWithID(t, resolveOutlinerMenus(t, app, chrome.ModeInbox), menuIDView)
	recent := menuWithID(t, resolveOutlinerMenus(t, app, chrome.ModeRecent), menuIDView)
	if len(inbox.Entries) != len(recent.Entries) {
		t.Fatalf("view menu changed shape between resolutions: %d then %d", len(inbox.Entries), len(recent.Entries))
	}
	for index, entry := range inbox.Entries {
		mode, ok := operation.GotoModeOperations[entry.OperationID]
		if !ok {
			continue
		}
		if got, want := *entry.Checked, mode == chrome.ModeInbox; got != want {
			t.Errorf("entry %d for mode %s checked=%v under Inbox, want %v", index, mode, got, want)
		}
		if got, want := *recent.Entries[index].Checked, mode == chrome.ModeRecent; got != want {
			t.Errorf("entry %d for mode %s checked=%v under Recent, want %v", index, mode, got, want)
		}
	}
}
