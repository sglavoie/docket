package tui

import (
	"strings"
	"testing"
)

// The view-local modals answer the pointer ahead of the rows they stand on: an
// open menu and the relation picker each read where the last render drew them,
// and the host beneath receives nothing.

// modalPointerMenuDriver opens the first menu, and reports the screen row the
// host drew item 193 on before the dropdown covered part of it.
func modalPointerMenuDriver(t *testing.T) (*UITestDriver, int) {
	t.Helper()
	driver := outlinerPointerDriver(t, FixtureRelease)
	driver.Resize(100, 35)
	y := pointerRowY(t, driver, "Release acceptance item 193")
	pointerKey(t, driver, "f10")
	if !pointerOutliner(t, driver).MenuBar.Open {
		t.Fatal("F10 did not open a menu")
	}
	return driver, y
}

func TestOutsideClickClosesTheMenuAndDoesNothingMore(t *testing.T) {
	driver, y := modalPointerMenuDriver(t)
	out := pointerOutliner(t, driver)
	cursor, rows, depth := out.Cursor, len(out.Rows), len(out.Focus.Stack)

	pointerClickAt(t, driver, 90, y, "", 1)
	if out.MenuBar.Open {
		t.Fatal("an outside click left the menu open")
	}
	if out.Cursor != cursor || len(out.Rows) != rows || len(out.Focus.Stack) != depth {
		t.Fatalf("the dismissing click reached the host: cursor %d->%d, rows %d->%d, focus %d->%d",
			cursor, out.Cursor, rows, len(out.Rows), depth, len(out.Focus.Stack))
	}
}

func TestDismissingClickIsNoHalfOfADoubleClick(t *testing.T) {
	driver, y := modalPointerMenuDriver(t)
	out := pointerOutliner(t, driver)
	depth := len(out.Focus.Stack)

	pointerClickAt(t, driver, 90, y, "", 1)
	pointerClickAt(t, driver, 90, y, "", 1)
	if len(out.Focus.Stack) != depth {
		t.Fatal("the press after a dismissing click was read as a double-click and zoomed in")
	}
	if !strings.Contains(out.Rows[out.Cursor].Task.Title, "193") {
		t.Fatalf("the second press is a click of its own and seats its row, got %q", out.Rows[out.Cursor].Task.Title)
	}
}

func TestClickOnTheDropdownFrameKeepsTheMenuOpen(t *testing.T) {
	driver, _ := modalPointerMenuDriver(t)
	out := pointerOutliner(t, driver)
	x, y, err := driver.pointerCell(UITestAction{On: "Capture"})
	if err != nil {
		t.Fatal(err)
	}
	item := out.MenuBar.ItemIndex

	// The border line above the first entry and the border cell beside it.
	pointerClickAt(t, driver, x, y-1, "", 1)
	pointerClickAt(t, driver, x-3, y, "", 1)
	if !out.MenuBar.Open || out.MenuBar.ItemIndex != item {
		t.Fatalf("a click on the frame acted: open=%v item %d->%d", out.MenuBar.Open, item, out.MenuBar.ItemIndex)
	}
	// A right-click on an entry is not the click that runs it.
	pointerClickAt(t, driver, x, y, "right", 1)
	if !out.MenuBar.Open {
		t.Fatal("a right-click ran an entry")
	}
}

func TestWheelOverTheDropdownWalksTheHighlight(t *testing.T) {
	driver, _ := modalPointerMenuDriver(t)
	out := pointerOutliner(t, driver)
	x, y, err := driver.pointerCell(UITestAction{On: "Capture"})
	if err != nil {
		t.Fatal(err)
	}
	item := out.MenuBar.ItemIndex
	if err := driver.runPointer(UITestAction{Type: UITestActionWheel, X: &x, Y: &y, Delta: 1}); err != nil {
		t.Fatal(err)
	}
	if out.MenuBar.ItemIndex == item {
		t.Fatal("the wheel over the dropdown did not move the highlight")
	}
	far := 95
	if err := driver.runPointer(UITestAction{Type: UITestActionWheel, X: &far, Y: &y, Delta: 1}); err != nil {
		t.Fatal(err)
	}
	if !out.MenuBar.Open {
		t.Fatal("a wheel step outside the dropdown closed the menu")
	}
}

func TestClickOutsideTheRelationPickerReachesNothing(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureMultiProject)
	driver.Resize(100, 35)
	if err := driver.runCommand("Clear All Folds"); err != nil {
		t.Fatal(err)
	}
	pointerKey(t, driver, "a")
	for _, r := range "Wire the importer %client-alpha ~client-beta:1" {
		pointerKey(t, driver, string(r))
	}
	pointerKey(t, driver, "enter")
	pointerKey(t, driver, "home")
	pointerKey(t, driver, "j")
	pointerKey(t, driver, "g")
	pointerKey(t, driver, "~")
	out := pointerOutliner(t, driver)
	if !out.RelationPickerOpen() {
		t.Fatal("the relation picker did not open")
	}
	cursor := out.Cursor
	y := pointerRowY(t, driver, "Renew passport")

	pointerClickAt(t, driver, 2, y, "", 2)
	pointerClickAt(t, driver, 2, y, "right", 1)
	if !out.RelationPickerOpen() || out.Cursor != cursor {
		t.Fatalf("a click beside the picker reached the host: open=%v cursor %d->%d",
			out.RelationPickerOpen(), cursor, out.Cursor)
	}
}
