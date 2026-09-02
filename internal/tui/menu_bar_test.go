package tui

import (
	"strings"
	"testing"
)

// The menu bar is a projection of the operation catalog, so these are the
// parity assertions that keep it one: the bar may only name operations the
// registry holds, it may never spell a key the strategy owns, the modes it
// offers are the modes the catalog declares, and an operation the bar is the
// only route to is never an entity operation. A menu that drifts from the
// grammar fails here rather than in front of a user.

// productionInputStrategies is every grammar a key can be pressed in. An
// operation absent from all five has no key anywhere, which is what "menu-only"
// means; reading them off surfaceOperationGates would only name the four entity
// ones plus the Outliner, in a shape built for a different question.
var productionInputStrategies = []InputStrategyID{
	InputStrategySimplified,
	InputStrategyProjectHub,
	InputStrategyNotes,
	InputStrategyAttachments,
	InputStrategyComments,
}

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
	for _, spec := range outlinerMenuSpecs(ModeAll, false, true) {
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
		outlinerMenuSpecs(ModeAll, false, true),
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
// validateOperationOnePath and TestTaskOperationsCannotAppearInBothDirectGrammarAndMenu
// are untouched by this rule and keep governing only that sheet.
func TestAMenuOnlyOperationIsNeverAnEntityOperation(t *testing.T) {
	registry := parityAuditApp(t).GetOperationRegistry()

	bound := make(map[OperationID]bool)
	for _, id := range productionInputStrategies {
		strategy, ok := registry.InputStrategy(id)
		if !ok {
			t.Fatalf("input strategy %s is not registered", id)
		}
		for _, binding := range strategy.Bindings {
			bound[binding.OperationID] = true
		}
	}

	menuOnly := make([]OperationID, 0, 1)
	for _, spec := range outlinerMenuSpecs(ModeAll, false, true) {
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
			if descriptor.TargetSource != OperationTargetGlobal &&
				descriptor.TargetSource != OperationTargetScopedProjects {
				t.Errorf(
					"menu-only operation %s resolves its target from %q; a keyless operation must be target-free or view-scoped",
					descriptor.ID,
					descriptor.TargetSource,
				)
			}
			if descriptor.TargetCardinality != TargetCardinalityNone {
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
	driver, outliner := menuBarDriver(t)
	registry := driver.App().GetOperationRegistry()

	for _, id := range productionInputStrategies {
		strategy, ok := registry.InputStrategy(id)
		if !ok {
			t.Fatalf("input strategy %s is not registered", id)
		}
		for _, binding := range strategy.Bindings {
			if binding.OperationID == operationCoreClearFolds {
				t.Fatalf("%s bound the keyless fold clear to %v", id, binding.paths())
			}
		}
	}

	if !outliner.toggleAllSubtrees() {
		t.Fatal("the rich fixture has no subtree to fold, so the clear proves nothing")
	}
	if !outliner.anyTaskFoldCollapsed() {
		t.Fatal("folding every subtree left nothing folded")
	}
	outliner.collapsedSections["_xp:pinned"] = true

	pressMenuBarKey(t, driver, "ctrl+m", "l")
	view := menuWithID(t, openMenuBarMenusForTest(t, outliner), menuIDView)
	if !menuHasLabel(view, "Clear All Folds") {
		t.Fatalf("the View menu does not offer the keyless entry: %+v", view.Entries)
	}
	index := -1
	for position, entry := range view.Entries {
		if entry.OperationID == operationCoreClearFolds {
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

	outliner.menuBarItemIndex = index
	pressMenuBarKey(t, driver, "enter")

	if outliner.menuBarOpen {
		t.Error("running the keyless entry left the menu open")
	}
	if driver.App().crossOutliner.anyTaskFoldCollapsed() {
		t.Error("the menu-only fold clear left a subtree folded")
	}
	for key, collapsed := range driver.App().crossOutliner.collapsedSections {
		if collapsed {
			t.Errorf("the menu-only fold clear left section %s collapsed", key)
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

// Opening is its own operation because the bar may be hidden when a user reaches
// for it. One press has to reveal the band and drop its first menu, or the same
// accelerator would mean "show" the first time and "open" the second.
func TestOpeningTheMenuBarRevealsItAndDropsItsFirstMenu(t *testing.T) {
	driver, outliner := menuBarDriver(t)

	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	if !driver.App().runtime.Machine().MenuBarVisible {
		t.Error("opening a menu did not reveal the hidden bar")
	}
	if !outliner.menuBarOpen {
		t.Fatal("ctrl+m revealed the bar without opening a menu")
	}

	menus := openMenuBarMenusForTest(t, outliner)
	if outliner.menuBarMenuIndex != 0 {
		t.Errorf("opened on menu %d, want the leftmost", outliner.menuBarMenuIndex)
	}
	if menus[0].ID != menuIDTask {
		t.Errorf("opened menu %s, want %s", menus[0].ID, menuIDTask)
	}
	if len(menus[0].Entries) == 0 {
		t.Fatal("the opened menu resolved to no entries at all")
	}

	screen := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(screen, "Capture Child") {
		t.Errorf("the dropdown is not drawn under the bar:\n%s", screen)
	}

	if err := driver.SendKey("esc"); err != nil {
		t.Fatalf("press esc: %v", err)
	}
	if outliner.menuBarOpen {
		t.Error("esc left the menu open")
	}
	// Esc puts the menu away, not the bar: hiding the band is `M`'s alone, and it
	// is machine-local state rather than a sub-state.
	if !driver.App().runtime.Machine().MenuBarVisible {
		t.Error("closing the menu also hid the bar it revealed")
	}
	if closed := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(closed, "Capture Child") {
		t.Errorf("the dropdown survived its close:\n%s", closed)
	}
}

// An entry the cursor cannot run is removed rather than greyed, so what the
// dropdown lists is what pressing Enter on it would do. A visual selection is
// the cheapest proof: it takes every single-task row away at once.
func TestOpenMenuOmitsAnEntryTheCursorCannotRun(t *testing.T) {
	driver, outliner := menuBarDriver(t)

	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	before := menuWithID(t, openMenuBarMenusForTest(t, outliner), menuIDTask)
	if !menuHasLabel(before, "External Editor") {
		t.Fatalf("the Task menu does not offer External Editor to begin with: %+v", before.Entries)
	}
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "External Editor") {
		t.Errorf("the applicable entry is not drawn:\n%s", screen)
	}

	outliner.multiSelectMode = true

	after := menuWithID(t, openMenuBarMenusForTest(t, outliner), menuIDTask)
	if menuHasLabel(after, "External Editor") {
		t.Errorf("External Editor survived a visual selection: %+v", after.Entries)
	}
	if len(after.Entries) == 0 {
		t.Fatal("the Task menu resolved to nothing, so the omission proves nothing")
	}
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(screen, "External Editor") {
		t.Errorf("the dropdown still draws the inapplicable entry:\n%s", screen)
	}
}

// The vertical walk steps over the rules a separator draws, and the horizontal
// one rings around the menus that survived resolution — never around the specs,
// which may hold a menu the bar is not drawing.
func TestMenuNavigationSkipsSeparatorsAndRingsSurvivingMenus(t *testing.T) {
	entries := []resolvedMenuEntry{
		{OperationID: "test.menu.a", Label: "A"},
		{},
		{OperationID: "test.menu.b", Label: "B"},
		{},
	}
	steps := []struct {
		name         string
		index, delta int
		want         int
	}{
		{name: "forward over a separator", index: 0, delta: 1, want: 2},
		{name: "backward over a separator", index: 2, delta: -1, want: 0},
		// Neither end wraps: a menu is a short list a user overshoots silently.
		{name: "forward off a trailing separator", index: 2, delta: 1, want: 2},
		{name: "backward off the top", index: 0, delta: -1, want: 0},
	}
	for _, tc := range steps {
		if got := stepMenuBarItem(entries, tc.index, tc.delta); got != tc.want {
			t.Errorf("%s: stepped %d to %d, want %d", tc.name, tc.index, got, tc.want)
		}
	}

	driver, outliner := menuBarDriver(t)
	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	menus := openMenuBarMenusForTest(t, outliner)
	if len(menus) < 2 {
		t.Fatalf("the bar drew %d menus, too few to cycle", len(menus))
	}

	// Walking the whole first menu must never rest on a rule, and the walk must
	// stop at the bottom rather than ring back to the top.
	live := menus[0].Entries
	for step := 0; step < len(live)+2; step++ {
		if err := driver.SendKey("j"); err != nil {
			t.Fatalf("press j: %v", err)
		}
		index := outliner.menuBarItemIndex
		if index < 0 || index >= len(live) {
			t.Fatalf("step %d left the highlight at %d, outside a menu of %d", step, index, len(live))
		}
		if live[index].OperationID == "" {
			t.Fatalf("step %d rested the highlight on the separator at %d", step, index)
		}
	}
	if got := outliner.menuBarItemIndex; got == 0 {
		t.Error("walking past the last entry rang back to the first")
	}

	for step := 1; step <= len(menus); step++ {
		if err := driver.SendKey("l"); err != nil {
			t.Fatalf("press l: %v", err)
		}
		if got, want := outliner.menuBarMenuIndex, step%len(menus); got != want {
			t.Fatalf("l %d times opened menu %d, want %d", step, got, want)
		}
		// A new menu opens at its own top; carrying an offset across would land the
		// highlight on whatever happened to sit at that depth.
		if got := outliner.menuBarItemIndex; got != 0 {
			t.Errorf("moving along the bar left the highlight at entry %d, want the top", got)
		}
	}
	if err := driver.SendKey("h"); err != nil {
		t.Fatalf("press h: %v", err)
	}
	if got, want := outliner.menuBarMenuIndex, len(menus)-1; got != want {
		t.Errorf("h from the leftmost menu opened %d, want the rightmost %d", got, want)
	}
}

// The stored index outlives the resolution it was taken from, because an entry
// leaves the moment the cursor stops being able to run it. So the render resolves
// it against the entries this frame actually holds and writes the answer back,
// which is what keeps the painted highlight and the row Enter runs the same row.
func TestAStoredItemIndexReResolvesDuringRender(t *testing.T) {
	driver, outliner := menuBarDriver(t)
	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	entries := openMenuBarMenusForTest(t, outliner)[0].Entries

	separator := -1
	for index, entry := range entries {
		if entry.OperationID == "" {
			separator = index
			break
		}
	}
	if separator < 0 {
		t.Fatalf("the Task menu draws no separator: %+v", entries)
	}

	cases := []struct {
		name  string
		index int
	}{
		{name: "parked on a separator", index: separator},
		{name: "past the end", index: len(entries) + 40},
		{name: "before the start", index: -7},
	}
	for _, tc := range cases {
		outliner.menuBarItemIndex = tc.index
		driver.Render()
		got := outliner.menuBarItemIndex
		if got < 0 || got >= len(entries) {
			t.Errorf("%s: rendered at %d, outside a menu of %d", tc.name, got, len(entries))
			continue
		}
		if entries[got].OperationID == "" {
			t.Errorf("%s: rendered at %d, which is a separator", tc.name, got)
		}
	}
}

// Enter is the whole point of the sub-state: the row names an operation, the
// menu closes, and the shell runs it through the one executor a key press
// reaches. Choosing a mode proves the round trip without opening an overlay to
// read it back through.
func TestEnterRunsTheHighlightedEntryAndClosesTheMenu(t *testing.T) {
	driver, outliner := menuBarDriver(t)
	pressMenuBarKey(t, driver, "ctrl+m")
	menus := openMenuBarMenusForTest(t, outliner)
	if len(menus) < 2 || menus[1].ID != menuIDView {
		t.Fatalf("the second menu is not %s: %+v", menuIDView, menus)
	}
	if outliner.primaryMode != ModeAll {
		t.Fatalf("the fixture launched in %s, want the All the View menu checks", outliner.primaryMode)
	}

	pressMenuBarKey(t, driver, "l", "j")
	entry := menus[1].Entries[outliner.menuBarItemIndex]
	if got := gotoModeOperations[entry.OperationID]; got != ModeToday {
		t.Fatalf("the highlighted entry is %s, want the Today mode row", entry.OperationID)
	}

	pressMenuBarKey(t, driver, "enter")
	if outliner.menuBarOpen {
		t.Error("enter left the menu open")
	}
	if got := driver.App().crossOutliner.primaryMode; got != ModeToday {
		t.Errorf("enter left the Outliner in %s, want Today", got)
	}
}

// The one deliberate divergence from the `Q` readout: this sub-state is not
// modal. The bar is a map of the grammar, so a user who reads a key off a row
// and presses it must get that act rather than a swallowed keystroke. The menu
// closes and the key is re-emitted, which is why the whole root grammar stays
// reachable one update later rather than being shadowed.
func TestAnUnrecognizedKeyClosesTheMenuAndStillRuns(t *testing.T) {
	driver, outliner := menuBarDriver(t)
	pressMenuBarKey(t, driver, "ctrl+m")
	if !outliner.menuBarOpen {
		t.Fatal("ctrl+m opened no menu")
	}
	if outliner.queryReadoutExpanded {
		t.Fatal("the fixture launched with the readout already open")
	}

	// Q is an accelerator the open Task menu says nothing about, which is exactly
	// the case a modal sub-state would eat.
	pressMenuBarKey(t, driver, "Q")
	if outliner.menuBarOpen {
		t.Error("an unrecognized key left the menu open")
	}
	if !outliner.queryReadoutExpanded {
		t.Error("the re-emitted key never reached the root grammar")
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
