package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// This file holds the parity rules themselves: the bar may only name operations
// the registry holds, it may never spell a key the strategy owns, the modes it
// offers are the modes the catalog declares, and an operation the bar is the
// only route to is never an entity operation. The fixtures and the resolution
// unit tests are in menu_bar_test.go; opening, walking, and running a menu is
// in menu_bar_navigation_test.go.

func TestEveryMenuEntryNamesARegisteredOperation(t *testing.T) {
	registry := parityAuditApp(t).GetOperationRegistry()

	named := 0
	for _, spec := range outliner.OutlinerMenuSpecs(chrome.ModeAll, false, true) {
		for index, entry := range spec.Entries {
			if entry.OperationID == "" {
				if entry.Label != "" || entry.Checked != nil {
					t.Errorf("menu %s entry %d is a separator carrying content", spec.ID, index)
				}
				continue
			}
			named++
			if _, exists := registry.Descriptor(entry.OperationID); !exists {
				t.Errorf("menu %s names unregistered operation %s", spec.ID, entry.OperationID)
			}
		}
	}
	if named == 0 {
		t.Fatal("the bar names no operations at all")
	}
}

func TestMenuEntriesNeverSpellTheirOwnKey(t *testing.T) {
	app := parityAuditApp(t)
	strategy, ok := app.GetOperationRegistry().InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	hinted := 0
	for _, menu := range resolveOutlinerMenus(t, app, chrome.ModeAll) {
		for _, entry := range menu.Entries {
			if entry.OperationID == "" {
				continue
			}
			want := operation.DisplayKeysForOperation(strategy, entry.OperationID)
			if entry.Key != want {
				t.Errorf(
					"menu %s entry %s hints %q, want the strategy's %q",
					menu.ID,
					entry.OperationID,
					entry.Key,
					want,
				)
			}
			if want != "" {
				hinted++
			}
		}
	}
	if hinted == 0 {
		t.Fatal("no menu entry carries a key hint, so this proves nothing")
	}
}

// A rebinding must move the menu without anyone editing it. Rebinding help in a
// copy of the strategy is the cheapest proof that the hint is projected rather
// than stored beside the entry.
func TestMenuKeyHintsFollowTheStrategy(t *testing.T) {
	app := parityAuditApp(t)
	strategy, ok := app.GetOperationRegistry().InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	for index := range strategy.Bindings {
		if strategy.Bindings[index].OperationID == operation.OperationCoreHelp {
			strategy.Bindings[index].DisplayKey = "F1"
		}
	}

	menus, err := outliner.ResolveMenuEntries(
		app.GetOperationRegistry(),
		strategy,
		menuBarCommandContext(),
		outliner.OutlinerMenuSpecs(chrome.ModeAll, false, true),
	)
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}
	help := menuWithID(t, menus, outliner.MenuIDHelp)
	if len(help.Entries) != 2 || help.Entries[0].Key != "F1" ||
		help.Entries[1].OperationID != operation.OperationCoreLastMessage || help.Entries[1].Key != "" {
		t.Fatalf("help menu did not follow the rebinding: %+v", help.Entries)
	}
}

func TestViewMenuCoversEveryOutlinerModeInCatalogOrder(t *testing.T) {
	app := parityAuditApp(t)
	view := menuWithID(t, resolveOutlinerMenus(t, app, chrome.ModeToday), outliner.MenuIDView)

	modes := make([]chrome.OutlinerModeID, 0, len(chrome.OutlinerModes))
	checked := make([]chrome.OutlinerModeID, 0, 1)
	for _, entry := range view.Entries {
		mode, ok := operation.GotoModeOperations[entry.OperationID]
		if !ok {
			continue
		}
		modes = append(modes, mode)
		if entry.Checked == nil {
			t.Errorf("view menu mode %s carries no radio state", mode)
			continue
		}
		if *entry.Checked {
			checked = append(checked, mode)
		}
	}

	if len(modes) != len(chrome.OutlinerModes) {
		t.Fatalf("view menu offers %d modes, want the catalog's %d", len(modes), len(chrome.OutlinerModes))
	}
	for index, mode := range chrome.OutlinerModes {
		if modes[index] != mode.ID {
			t.Errorf("view menu mode %d is %s, want catalog order's %s", index, modes[index], mode.ID)
		}
	}
	if len(checked) != 1 || checked[0] != chrome.ModeToday {
		t.Errorf("view menu checks %v, want exactly the active mode Today", checked)
	}
}

func TestEveryMenuIsUniquelyIdentifiedAndInhabited(t *testing.T) {
	menus := resolveOutlinerMenus(t, parityAuditApp(t), chrome.ModeAll)

	seen := make(map[string]bool, len(menus))
	for _, menu := range menus {
		if seen[menu.ID] {
			t.Errorf("menu ID %s appears twice in the bar", menu.ID)
		}
		seen[menu.ID] = true
		if menu.Title == "" {
			t.Errorf("menu %s has no title", menu.ID)
		}
		if len(menu.Entries) == 0 {
			t.Errorf("menu %s survived resolution with no entries", menu.ID)
		}
		if entries := menu.Entries; len(entries) > 0 {
			if entries[0].OperationID == "" || entries[len(entries)-1].OperationID == "" {
				t.Errorf("menu %s opens or ends on a separator", menu.ID)
			}
		}
		for index := 1; index < len(menu.Entries); index++ {
			if menu.Entries[index].OperationID == "" && menu.Entries[index-1].OperationID == "" {
				t.Errorf("menu %s shows two separators in a row at %d", menu.ID, index)
			}
		}
	}
	for _, id := range []string{outliner.MenuIDTask, outliner.MenuIDView, outliner.MenuIDGo, outliner.MenuIDFind, outliner.MenuIDHelp} {
		if !seen[id] {
			t.Errorf("menu %s is missing from the bar", id)
		}
	}
}

// The fourth parity rule, and the only one the bar itself creates. A descriptor
// with no binding in any InputStrategy was always legal — nothing forces a key.
// What is new is that the bar makes such an operation reachable, which mints a
// category the grammar has to bound:
//
// A menu-only operation may not be an entity operation. Its TargetSource is
// target-free or view-scoped, never a task or a project.
//
// An entity act whose one route is a band that is off by default would be
// discoverable only by a user who already knew to press `M`, and the entity
// grammar answers that question elsewhere: the `Space` sheet is where
// direct-only-or-menu-only earns its keep, which is why
// ValidateOperationOnePath and TestTaskOperationsCannotAppearInBothDirectGrammarAndMenu
// are untouched by this rule and keep governing only that sheet.
func TestAMenuOnlyOperationIsNeverAnEntityOperation(t *testing.T) {
	registry := parityAuditApp(t).GetOperationRegistry()

	bound := make(map[operation.OperationID]bool)
	for _, id := range productionInputStrategies {
		strategy, ok := registry.InputStrategy(id)
		if !ok {
			t.Fatalf("input strategy %s is not registered", id)
		}
		for _, binding := range strategy.Bindings {
			bound[binding.OperationID] = true
		}
	}

	menuOnly := make([]operation.OperationID, 0, 1)
	for _, spec := range outliner.OutlinerMenuSpecs(chrome.ModeAll, false, true) {
		for _, entry := range spec.Entries {
			if entry.OperationID == "" || bound[entry.OperationID] {
				continue
			}
			menuOnly = append(menuOnly, entry.OperationID)
			descriptor, exists := registry.Descriptor(entry.OperationID)
			if !exists {
				t.Errorf("menu %s names unregistered operation %s", spec.ID, entry.OperationID)
				continue
			}
			// Global is target-free and ScopedProjects is the view's own scope; the
			// four remaining sources all name an entity the cursor supplies.
			if descriptor.TargetSource != operation.OperationTargetGlobal &&
				descriptor.TargetSource != operation.OperationTargetScopedProjects {
				t.Errorf(
					"menu-only operation %s resolves its target from %q; a keyless operation must be target-free or view-scoped",
					descriptor.ID,
					descriptor.TargetSource,
				)
			}
			if descriptor.TargetCardinality != operation.TargetCardinalityNone {
				t.Errorf(
					"menu-only operation %s wants %v target(s); a keyless operation carries none",
					descriptor.ID,
					descriptor.TargetCardinality,
				)
			}
		}
	}
	if len(menuOnly) == 0 {
		t.Fatal("no operation is reachable only through the menu, so the category is theory rather than a rule")
	}
}

// The menu-only category is only worth a rule if something inhabits it, and only
// worth a menu row if running it from that row actually works. Clearing the folds
// three separate keys wrote is the proof: no key spells it, the View menu is the
// only place it exists, and the tree is open afterwards.
func TestTheKeylessFoldClearIsReachableOnlyFromTheMenu(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)
	registry := driver.App().GetOperationRegistry()

	for _, id := range productionInputStrategies {
		strategy, ok := registry.InputStrategy(id)
		if !ok {
			t.Fatalf("input strategy %s is not registered", id)
		}
		for _, binding := range strategy.Bindings {
			if binding.OperationID == operation.OperationCoreClearFolds {
				t.Fatalf("%s bound the keyless fold clear to %v", id, binding.Paths())
			}
		}
	}

	if !outlinerView.ToggleAllSubtrees() {
		t.Fatal("the rich fixture has no subtree to fold, so the clear proves nothing")
	}
	if !outlinerView.AnyTaskFoldCollapsed() {
		t.Fatal("folding every subtree left nothing folded")
	}
	outlinerView.Folds.CollapsedSections["_xp:pinned"] = true

	pressMenuBarKey(t, driver, "ctrl+m", "l")
	view := menuWithID(t, openMenuBarMenusForTest(t, outlinerView), outliner.MenuIDView)
	if !menuHasLabel(view, "Clear All Folds") {
		t.Fatalf("the View menu does not offer the keyless entry: %+v", view.Entries)
	}
	index := -1
	for position, entry := range view.Entries {
		if entry.OperationID == operation.OperationCoreClearFolds {
			index = position
			// A keyless operation spells no key, which is the visible half of the rule.
			if entry.Key != "" {
				t.Errorf("the keyless entry hints key %q", entry.Key)
			}
		}
	}
	if index < 0 {
		t.Fatal("the keyless entry resolved without its operation ID")
	}

	outlinerView.MenuBar.ItemIndex = index
	pressMenuBarKey(t, driver, "enter")

	if outlinerView.MenuBar.Open {
		t.Error("running the keyless entry left the menu open")
	}
	if driver.App().crossOutliner.AnyTaskFoldCollapsed() {
		t.Error("the menu-only fold clear left a subtree folded")
	}
	for key, collapsed := range driver.App().crossOutliner.Folds.CollapsedSections {
		if collapsed {
			t.Errorf("the menu-only fold clear left section %s collapsed", key)
		}
	}
}
