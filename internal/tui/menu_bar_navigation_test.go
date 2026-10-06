package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// This file covers the sub-state a user drives: revealing the band, walking it,
// what an open menu draws, and what Enter and an unrecognized key each do. The
// parity rules the bar is held to are in menu_bar_parity_test.go.

// Opening is its own operation because the bar may be hidden when a user reaches
// for it. One press has to reveal the band and drop its first menu, or the same
// accelerator would mean "show" the first time and "open" the second.
func TestOpeningTheMenuBarRevealsItAndDropsItsFirstMenu(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)

	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	if !outlinerView.MenuBarVisible() {
		t.Error("opening a menu did not reveal the hidden bar")
	}
	// The reveal is the menu's, not the machine's: `M` is the only key that says
	// keep the bar, so a borrowed row must not be written down as a choice.
	if driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Error("opening a menu committed the machine-local visibility flag")
	}
	if !outlinerView.MenuBar.Open {
		t.Fatal("ctrl+m revealed the bar without opening a menu")
	}

	menus := openMenuBarMenusForTest(t, outlinerView)
	if outlinerView.MenuBar.MenuIndex != 0 {
		t.Errorf("opened on menu %d, want the leftmost", outlinerView.MenuBar.MenuIndex)
	}
	if menus[0].ID != outliner.MenuIDTask {
		t.Errorf("opened menu %s, want %s", menus[0].ID, outliner.MenuIDTask)
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
	if outlinerView.MenuBar.Open {
		t.Error("esc left the menu open")
	}
	// Esc gives the borrowed band back. Visibility is a machine-local choice `M`
	// alone writes, so a menu that revealed the bar to hang off it leaves the
	// Outliner exactly as hidden as it found it.
	if outlinerView.MenuBarVisible() {
		t.Error("closing the menu left the bar it borrowed on screen")
	}
	if driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Error("closing the menu committed the machine-local visibility flag")
	}
	if closed := driver.Observe(ObserveOptions{}).PlainScreen; strings.Contains(closed, "Capture Child") {
		t.Errorf("the dropdown survived its close:\n%s", closed)
	}
}

func TestMenuBarF10OpensAndClosesWithoutEnhancedKeyboardEvents(t *testing.T) {
	driver, view := menuBarDriver(t)
	pressMenuBarKey(t, driver, "f10")
	if !view.MenuBar.Open || !view.MenuBarVisible() {
		t.Fatal("F10 did not open the hidden menu bar")
	}
	pressMenuBarKey(t, driver, "f10")
	if view.MenuBar.Open || view.MenuBarVisible() {
		t.Fatal("F10 did not close the menu and release its borrowed row")
	}
	pressMenuBarKey(t, driver, "?")
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "F10/Ctrl+M") {
		// The System category may be below the viewport; search its exact key.
		pressMenuBarKey(t, driver, "/", "F", "1", "0", "enter")
		if screen = driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "F10/Ctrl+M") {
			t.Fatalf("Help does not advertise F10:\n%s", screen)
		}
	}
}

func TestMenuHistoryLabelsFollowTheWorkspaceLedger(t *testing.T) {
	driver, view := menuBarDriver(t)
	pressMenuBarKey(t, driver, "!", "1")
	undo, redo := driver.App().workspaceHistoryPreview()
	if undo == "" || redo != "" {
		t.Fatalf("after priority change: undo=%q redo=%q", undo, redo)
	}
	pressMenuBarKey(t, driver, "ctrl+m")
	menu := menuWithID(t, openMenuBarMenusForTest(t, view), outliner.MenuIDTask)
	if !menuHasLabel(menu, "Undo: "+undo) {
		t.Fatalf("menu does not name the pending undo: %+v", menu.Entries)
	}
	pressMenuBarKey(t, driver, "esc", "u", "ctrl+m")
	menu = menuWithID(t, openMenuBarMenusForTest(t, view), outliner.MenuIDTask)
	if !menuHasLabel(menu, "Redo: "+undo) {
		t.Fatalf("reopened menu does not name the pending redo: %+v", menu.Entries)
	}
	pressMenuBarKey(t, driver, "esc", "ctrl+r", "ctrl+m")
	menu = menuWithID(t, openMenuBarMenusForTest(t, view), outliner.MenuIDTask)
	if !menuHasLabel(menu, "Undo: "+undo) || menuHasLabel(menu, "Redo") {
		t.Fatalf("menu retained stale history labels after redo: %+v", menu.Entries)
	}
}

// The other half of the same rule: a menu opened out of a bar the machine asked
// for leaves that bar up. Closing a menu restores the visibility the menu found,
// which is the one fact both directions share — the dropdown is a sub-state and
// never a write to the chrome flag underneath it.
func TestClosingAMenuLeavesAShownBarShown(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)

	if err := driver.SendKey("M"); err != nil {
		t.Fatalf("press M: %v", err)
	}
	if !driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Fatal("M did not show the bar")
	}

	pressMenuBarKey(t, driver, "ctrl+m")
	if !outlinerView.MenuBar.Open {
		t.Fatal("ctrl+m opened no menu")
	}

	pressMenuBarKey(t, driver, "esc")
	if outlinerView.MenuBar.Open {
		t.Fatal("esc left the menu open")
	}
	if !outlinerView.MenuBarVisible() {
		t.Error("closing the menu hid a bar the machine had asked for")
	}
	if !driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Error("closing the menu cleared the machine-local visibility flag")
	}
}

// An entry the cursor cannot run is removed rather than greyed, so what the
// dropdown lists is what pressing Enter on it would do. A visual selection is
// the cheapest proof: it takes every single-task row away at once.
func TestOpenMenuOmitsAnEntryTheCursorCannotRun(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)

	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	before := menuWithID(t, openMenuBarMenusForTest(t, outlinerView), outliner.MenuIDTask)
	if !menuHasLabel(before, "External Editor") {
		t.Fatalf("the Task menu does not offer External Editor to begin with: %+v", before.Entries)
	}
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "External Editor") {
		t.Errorf("the applicable entry is not drawn:\n%s", screen)
	}

	outlinerView.MultiSelectMode = true

	after := menuWithID(t, openMenuBarMenusForTest(t, outlinerView), outliner.MenuIDTask)
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
	entries := []outliner.ResolvedMenuEntry{
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
		if got := outliner.StepMenuBarItem(entries, tc.index, tc.delta); got != tc.want {
			t.Errorf("%s: stepped %d to %d, want %d", tc.name, tc.index, got, tc.want)
		}
	}

	driver, outlinerView := menuBarDriver(t)
	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	menus := openMenuBarMenusForTest(t, outlinerView)
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
		index := outlinerView.MenuBar.ItemIndex
		if index < 0 || index >= len(live) {
			t.Fatalf("step %d left the highlight at %d, outside a menu of %d", step, index, len(live))
		}
		if live[index].OperationID == "" {
			t.Fatalf("step %d rested the highlight on the separator at %d", step, index)
		}
	}
	if got := outlinerView.MenuBar.ItemIndex; got == 0 {
		t.Error("walking past the last entry rang back to the first")
	}

	for step := 1; step <= len(menus); step++ {
		if err := driver.SendKey("l"); err != nil {
			t.Fatalf("press l: %v", err)
		}
		if got, want := outlinerView.MenuBar.MenuIndex, step%len(menus); got != want {
			t.Fatalf("l %d times opened menu %d, want %d", step, got, want)
		}
		// A new menu opens at its own top; carrying an offset across would land the
		// highlight on whatever happened to sit at that depth.
		if got := outlinerView.MenuBar.ItemIndex; got != 0 {
			t.Errorf("moving along the bar left the highlight at entry %d, want the top", got)
		}
	}
	if err := driver.SendKey("h"); err != nil {
		t.Fatalf("press h: %v", err)
	}
	if got, want := outlinerView.MenuBar.MenuIndex, len(menus)-1; got != want {
		t.Errorf("h from the leftmost menu opened %d, want the rightmost %d", got, want)
	}
}

// The stored index outlives the resolution it was taken from, because an entry
// leaves the moment the cursor stops being able to run it. So the render resolves
// it against the entries this frame actually holds and writes the answer back,
// which is what keeps the painted highlight and the row Enter runs the same row.
func TestAStoredItemIndexReResolvesDuringRender(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)
	if err := driver.SendKey("ctrl+m"); err != nil {
		t.Fatalf("press ctrl+m: %v", err)
	}
	entries := openMenuBarMenusForTest(t, outlinerView)[0].Entries

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
		outlinerView.MenuBar.ItemIndex = tc.index
		driver.Render()
		got := outlinerView.MenuBar.ItemIndex
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
	driver, outlinerView := menuBarDriver(t)
	pressMenuBarKey(t, driver, "ctrl+m")
	menus := openMenuBarMenusForTest(t, outlinerView)
	if len(menus) < 2 || menus[1].ID != outliner.MenuIDView {
		t.Fatalf("the second menu is not %s: %+v", outliner.MenuIDView, menus)
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("the fixture launched in %s, want the All the View menu checks", outlinerView.PrimaryMode)
	}

	pressMenuBarKey(t, driver, "l", "j")
	entry := menus[1].Entries[outlinerView.MenuBar.ItemIndex]
	if got := operation.GotoModeOperations[entry.OperationID]; got != chrome.ModeToday {
		t.Fatalf("the highlighted entry is %s, want the Today mode row", entry.OperationID)
	}

	pressMenuBarKey(t, driver, "enter")
	if outlinerView.MenuBar.Open {
		t.Error("enter left the menu open")
	}
	if got := driver.App().crossOutliner.PrimaryMode; got != chrome.ModeToday {
		t.Errorf("enter left the Outliner in %s, want Today", got)
	}
}

// The one deliberate divergence from the `Q` readout: this sub-state is not
// modal. The bar is a map of the grammar, so a user who reads a key off a row
// and presses it must get that act rather than a swallowed keystroke. The menu
// closes and the key is re-emitted, which is why the whole root grammar stays
// reachable one update later rather than being shadowed.
func TestAnUnrecognizedKeyClosesTheMenuAndStillRuns(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)
	pressMenuBarKey(t, driver, "ctrl+m")
	if !outlinerView.MenuBar.Open {
		t.Fatal("ctrl+m opened no menu")
	}
	if outlinerView.QueryReadout.Expanded {
		t.Fatal("the fixture launched with the readout already open")
	}

	// Q is an accelerator the open Task menu says nothing about, which is exactly
	// the case a modal sub-state would eat.
	pressMenuBarKey(t, driver, "Q")
	if outlinerView.MenuBar.Open {
		t.Error("an unrecognized key left the menu open")
	}
	if !outlinerView.QueryReadout.Expanded {
		t.Error("the re-emitted key never reached the root grammar")
	}
}
