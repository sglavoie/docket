package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The wheel scrolls what it is over: the list's window one row per step, the
// Peek dock through scrollPeek, and an open Q readout through its own path.

func pointerWheelAt(t *testing.T, driver *UITestDriver, y, delta int) {
	t.Helper()
	x := 20
	if err := driver.runPointer(UITestAction{Type: UITestActionWheel, X: &x, Y: &y, Delta: delta}); err != nil {
		t.Fatal(err)
	}
}

// wheelBusyDriver is the busy fixture at the minimum height with the Peek dock
// open, where its list outgrows the window.
func wheelBusyDriver(t *testing.T) *UITestDriver {
	t.Helper()
	driver := outlinerPointerDriver(t, FixtureBusy)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, "Untriaged inbox item 1"), "", 1)
	pointerKey(t, driver, "b")
	return driver
}

func TestWheelScrollsTheWindowAndLeavesTheCursorOnItsRow(t *testing.T) {
	driver := wheelBusyDriver(t)
	out := pointerOutliner(t, driver)
	listY := pointerRowY(t, driver, "Untriaged inbox item 1")
	pointerClickAt(t, driver, 20, listY+3, "", 1)
	cursor, offset := out.Cursor, out.ViewScrollOffset

	pointerWheelAt(t, driver, listY, 1)
	if out.ViewScrollOffset != offset+1 || out.Cursor != cursor {
		t.Fatalf("one step down left offset %d cursor %d, want %d and %d", out.ViewScrollOffset, out.Cursor, offset+1, cursor)
	}
	// The write is the window authority's own answer, so the next render keeps it.
	_ = driver.Render()
	if out.ViewScrollOffset != offset+1 || out.Cursor != cursor {
		t.Fatalf("the render moved the wheel's window to %d (cursor %d)", out.ViewScrollOffset, out.Cursor)
	}
	pointerWheelAt(t, driver, listY, -1)
	if out.ViewScrollOffset != offset || out.Cursor != cursor {
		t.Fatalf("one step back left offset %d cursor %d", out.ViewScrollOffset, out.Cursor)
	}
}

func TestWheelPullsTheCursorOnlyWhenItsRowLeaves(t *testing.T) {
	driver := wheelBusyDriver(t)
	out := pointerOutliner(t, driver)
	listY := pointerRowY(t, driver, "Untriaged inbox item 1")
	start, _ := out.VisibleRowWindow()
	out.Cursor = start
	_ = driver.Render()
	start, _ = out.VisibleRowWindow()

	pointerWheelAt(t, driver, listY, 1)
	_ = driver.Render()
	newStart, newEnd := out.VisibleRowWindow()
	if newStart == start {
		t.Fatal("the step down did not move the window")
	}
	if out.Cursor < newStart || out.Cursor >= newEnd || !out.Rows[out.Cursor].IsNavigable() {
		t.Fatalf("the cursor was left on row %d outside the window [%d, %d)", out.Cursor, newStart, newEnd)
	}
	if out.Cursor-newStart > 1 {
		t.Fatalf("the cursor was pulled to row %d, not the nearest row inside [%d, %d)", out.Cursor, newStart, newEnd)
	}
}

func TestWheelRefusesPastEitherEnd(t *testing.T) {
	driver := wheelBusyDriver(t)
	out := pointerOutliner(t, driver)
	listY := pointerRowY(t, driver, "Untriaged inbox item 1")
	pointerKey(t, driver, "g")
	pointerKey(t, driver, "g")
	_ = driver.Render()
	before := driver.Render()
	pointerWheelAt(t, driver, listY, -1)
	if after := driver.Render(); after != before || out.ViewScrollOffset != 0 {
		t.Fatalf("a step up at the top moved the list to %d", out.ViewScrollOffset)
	}

	pointerKey(t, driver, "G")
	before = driver.Render()
	offset, cursor := out.ViewScrollOffset, out.Cursor
	pointerWheelAt(t, driver, listY, 1)
	if after := driver.Render(); after != before || out.ViewScrollOffset != offset || out.Cursor != cursor {
		t.Fatalf("a step down at the bottom moved the list to %d (cursor %d)", out.ViewScrollOffset, out.Cursor)
	}
}

func TestWheelNeverChangesAVisualSelection(t *testing.T) {
	driver := wheelBusyDriver(t)
	out := pointerOutliner(t, driver)
	listY := pointerRowY(t, driver, "Untriaged inbox item 1")
	pointerClickAt(t, driver, 20, listY, "", 1)
	pointerKey(t, driver, "V")
	pointerKey(t, driver, "j")
	members := len(out.SelectedTaskIdentities())
	cursor := out.Cursor
	start, _ := out.VisibleRowWindow()

	// The window may move while the cursor stays inside it, and stops where the
	// cursor would have to.
	for range 10 {
		pointerWheelAt(t, driver, listY, 1)
	}
	_ = driver.Render()
	newStart, _ := out.VisibleRowWindow()
	if out.Cursor != cursor || len(out.SelectedTaskIdentities()) != members {
		t.Fatalf("the wheel moved the cursor to %d or reshaped the range to %d members", out.Cursor, len(out.SelectedTaskIdentities()))
	}
	if newStart > cursor || newStart < start {
		t.Fatalf("the window moved to %d past the cursor at %d", newStart, cursor)
	}
}

func TestWheelOverChromeDoesNothing(t *testing.T) {
	driver := wheelBusyDriver(t)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, "Untriaged inbox item 1")+3, "", 1)
	before := driver.Render()
	offset, cursor := out.ViewScrollOffset, out.Cursor
	for _, y := range []int{0, 18, 19} {
		pointerWheelAt(t, driver, y, 1)
	}
	if after := driver.Render(); after != before || out.ViewScrollOffset != offset || out.Cursor != cursor {
		t.Fatalf("the wheel over chrome changed the screen:\n%s", after)
	}
}

func TestWheelOverTheDockScrollsPeekAndNotTheList(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureRich)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 0, pointerRowY(t, driver, "Backlog"), "", 1)
	pointerKey(t, driver, "l")
	pointerClickAt(t, driver, 30, pointerRowY(t, driver, "Add password validat"), "", 1)
	pointerKey(t, driver, "b")
	dockY := pointerRowY(t, driver, "0 above · 2 below")
	offset, cursor := out.ViewScrollOffset, out.Cursor

	for _, want := range []string{"1 above · 1 below", "2 above · 0 below", "2 above · 0 below"} {
		pointerWheelAt(t, driver, dockY, 1)
		if !hasLiteral(driver, want) {
			t.Fatalf("a step down over the dock did not reach %q:\n%s", want, driver.Render())
		}
	}
	for _, want := range []string{"1 above · 1 below", "0 above · 2 below", "0 above · 2 below"} {
		pointerWheelAt(t, driver, dockY-2, -1)
		if !hasLiteral(driver, want) {
			t.Fatalf("a step up over the dock did not reach %q:\n%s", want, driver.Render())
		}
	}
	// The dock keeps every step at both ends rather than passing it to the list.
	if out.ViewScrollOffset != offset || out.Cursor != cursor {
		t.Fatal("a step over the dock moved the list")
	}
}

func TestWheelOverAnOpenReadoutScrollsItAndNothingElse(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	app := overflowingReadoutApp(t)
	out := app.crossOutliner
	_ = out.View()
	cursor, scroll := out.Cursor, out.ViewScrollOffset
	readoutY := out.Height() - out.ShellFrame.ExcessRows() - 1

	// The readout owns every key while it is open, so a step over the list is
	// swallowed rather than scrolling the list beneath it.
	out.HandlePointer(route.PointerEvent{X: 5, Y: 2, WheelDelta: 1})
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("a step over the list moved the readout:\n%s", got)
	}
	out.HandlePointer(route.PointerEvent{X: 5, Y: readoutY, WheelDelta: 1})
	if got := readoutBodyText(out, 60); !strings.Contains(got, "1 above") {
		t.Fatalf("a step over the readout did not scroll it:\n%s", got)
	}
	_ = out.View()
	out.HandlePointer(route.PointerEvent{X: 5, Y: readoutY, WheelDelta: -1})
	if got := readoutBodyText(out, 60); !strings.Contains(got, "0 above") {
		t.Fatalf("a step back over the readout did not return it:\n%s", got)
	}
	if out.Cursor != cursor || out.ViewScrollOffset != scroll {
		t.Fatal("the wheel over the readout moved the outline")
	}
}

// wheelRoundTrip steps the list down and back, asserting that each step moves
// the window by one row, survives the render after it, and leaves the cursor on
// a row the window draws.
func wheelRoundTrip(t *testing.T, driver *UITestDriver, y int) {
	t.Helper()
	out := pointerOutliner(t, driver)
	_ = driver.Render()
	offset := out.ViewScrollOffset
	for _, step := range []struct{ delta, want int }{{1, offset + 1}, {-1, offset}} {
		pointerWheelAt(t, driver, y, step.delta)
		_ = driver.Render()
		start, end := out.VisibleRowWindow()
		if out.ViewScrollOffset != step.want || out.Cursor < start || out.Cursor >= end {
			t.Fatalf("a step of %d left offset %d (want %d) and cursor %d outside [%d, %d):\n%s",
				step.delta, out.ViewScrollOffset, step.want, out.Cursor, start, end, driver.Render())
		}
	}
}

func TestWheelScrollsRankedResultsAndSingleProjectFormsAlike(t *testing.T) {
	driver := wheelBusyDriver(t)
	// The expanded dock leaves the list fewer rows than either form holds.
	for _, key := range []string{"B", "/", "i", "t", "e", "m", "enter", "g", "g"} {
		pointerKey(t, driver, key)
	}
	if !pointerOutliner(t, driver).ShowsRankedList() {
		t.Fatal("the search did not open the ranked list")
	}
	wheelRoundTrip(t, driver, pointerRowY(t, driver, "Untriaged inbox item 2"))

	driver = wheelBusyDriver(t)
	pointerKey(t, driver, "B")
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, "[inbox-backlog]"), "", 1)
	pointerKey(t, driver, "enter")
	pointerKey(t, driver, "g")
	pointerKey(t, driver, "g")
	if !hasLiteral(driver, "inbox-backlog") || hasLiteral(driver, "Waiting") {
		t.Fatalf("Enter on the project header did not zoom into it:\n%s", driver.Render())
	}
	wheelRoundTrip(t, driver, pointerRowY(t, driver, "Untriaged inbox item 2"))
}
