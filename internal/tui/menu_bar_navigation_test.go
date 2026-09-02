package tui

import (
	"strings"
	"testing"
)

// This file covers the sub-state a user drives: revealing the band, walking it,
// what an open menu draws, and what Enter and an unrecognized key each do. The
// parity rules the bar is held to are in menu_bar_parity_test.go.

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
