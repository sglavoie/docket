package tui

import (
	"testing"
)

// The menu bar is a projection of the operation catalog, so these are the
// parity assertions that keep it one: the bar may only name operations the
// registry holds, it may never spell a key the strategy owns, and the modes it
// offers are the modes the catalog declares. A menu that drifts from the
// grammar fails here rather than in front of a user.

// menuBarCommandContext is the launch context: the Outliner presentation with
// the simplified grammar and a cursor that names nothing.
func menuBarCommandContext() CommandContext {
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.Capabilities.Add(CapabilitySimplifiedInput)
	return ctx
}

// resolveOutlinerMenus projects the production bar against the production
// catalog, which is what makes these assertions audit the shipped specs rather
// than a test-local subset of them.
func resolveOutlinerMenus(t *testing.T, app *AppModel, activeMode OutlinerModeID) []resolvedMenu {
	t.Helper()
	strategy, ok := app.GetOperationRegistry().InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	menus, err := resolveMenuEntries(
		app.GetOperationRegistry(),
		strategy,
		menuBarCommandContext(),
		outlinerMenuSpecs(activeMode, false),
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
	registry *OperationRegistry,
	id OperationID,
	label string,
	applicable bool,
) {
	t.Helper()
	if err := registry.Register(OperationDescriptor{
		ID:                id,
		Label:             label,
		Category:          OperationCategorySystem,
		TargetCardinality: TargetCardinalityNone,
		Risk:              OperationRiskReadOnly,
		Applicability:     func(CommandContext) bool { return applicable },
	}); err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
}

func TestEveryMenuEntryNamesARegisteredOperation(t *testing.T) {
	registry := parityAuditApp(t).GetOperationRegistry()

	named := 0
	for _, spec := range outlinerMenuSpecs(ModeAll, false) {
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
	strategy, ok := app.GetOperationRegistry().InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	hinted := 0
	for _, menu := range resolveOutlinerMenus(t, app, ModeAll) {
		for _, entry := range menu.Entries {
			if entry.OperationID == "" {
				continue
			}
			want := displayKeysForOperation(strategy, entry.OperationID)
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
	strategy, ok := app.GetOperationRegistry().InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	for index := range strategy.Bindings {
		if strategy.Bindings[index].OperationID == operationCoreHelp {
			strategy.Bindings[index].DisplayKey = "F1"
		}
	}

	menus, err := resolveMenuEntries(
		app.GetOperationRegistry(),
		strategy,
		menuBarCommandContext(),
		outlinerMenuSpecs(ModeAll, false),
	)
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}
	help := menuWithID(t, menus, menuIDHelp)
	if len(help.Entries) != 1 || help.Entries[0].Key != "F1" {
		t.Fatalf("help menu did not follow the rebinding: %+v", help.Entries)
	}
}

func TestViewMenuCoversEveryOutlinerModeInCatalogOrder(t *testing.T) {
	app := parityAuditApp(t)
	view := menuWithID(t, resolveOutlinerMenus(t, app, ModeToday), menuIDView)

	modes := make([]OutlinerModeID, 0, len(outlinerModes))
	checked := make([]OutlinerModeID, 0, 1)
	for _, entry := range view.Entries {
		mode, ok := gotoModeOperations[entry.OperationID]
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

	if len(modes) != len(outlinerModes) {
		t.Fatalf("view menu offers %d modes, want the catalog's %d", len(modes), len(outlinerModes))
	}
	for index, mode := range outlinerModes {
		if modes[index] != mode.ID {
			t.Errorf("view menu mode %d is %s, want catalog order's %s", index, modes[index], mode.ID)
		}
	}
	if len(checked) != 1 || checked[0] != ModeToday {
		t.Errorf("view menu checks %v, want exactly the active mode Today", checked)
	}
}

func TestEveryMenuIsUniquelyIdentifiedAndInhabited(t *testing.T) {
	menus := resolveOutlinerMenus(t, parityAuditApp(t), ModeAll)

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
	for _, id := range []string{menuIDTask, menuIDView, menuIDGo, menuIDFind, menuIDHelp} {
		if !seen[id] {
			t.Errorf("menu %s is missing from the bar", id)
		}
	}
}

func TestResolveMenuEntriesRefusesAnUnregisteredOperation(t *testing.T) {
	registry := NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.present", "Present", true)

	_, err := resolveMenuEntries(registry, InputStrategy{}, menuBarCommandContext(), []menuSpec{{
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
	registry := NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.here", "Here", true)
	registerMenuTestOperation(t, registry, "test.menu.gone", "Gone", false)

	menus, err := resolveMenuEntries(registry, InputStrategy{}, menuBarCommandContext(), []menuSpec{
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

	inbox := menuWithID(t, resolveOutlinerMenus(t, app, ModeInbox), menuIDView)
	recent := menuWithID(t, resolveOutlinerMenus(t, app, ModeRecent), menuIDView)
	if len(inbox.Entries) != len(recent.Entries) {
		t.Fatalf("view menu changed shape between resolutions: %d then %d", len(inbox.Entries), len(recent.Entries))
	}
	for index, entry := range inbox.Entries {
		mode, ok := gotoModeOperations[entry.OperationID]
		if !ok {
			continue
		}
		if got, want := *entry.Checked, mode == ModeInbox; got != want {
			t.Errorf("entry %d for mode %s checked=%v under Inbox, want %v", index, mode, got, want)
		}
		if got, want := *recent.Entries[index].Checked, mode == ModeRecent; got != want {
			t.Errorf("entry %d for mode %s checked=%v under Recent, want %v", index, mode, got, want)
		}
	}
}
