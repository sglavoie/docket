package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// The bar is chrome with a budget: it must be exactly as wide as every other
// Outliner line, it must cost exactly the one row contentHeight subtracts, and
// it must drop whole menus rather than truncate a label. These are the
// assertions that hold it to all three.

// An open menu is the whole frame: the bar row, the block under its label, and
// nothing else. The list is not shrunk to make room for a dropdown, it is not
// drawn, which is what stops every row on screen from stepping down and back up
// once per menu press. The window the last list frame drew has to survive that
// untouched, since it is the one the close redraws.
func TestAnOpenMenuDrawsNoOutlinerAndMovesNoWindow(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)

	// Show the bar first: revealing it costs a list row, and this test is about
	// what opening a menu does on top of a bar that is already up.
	pressMenuBarKey(t, driver, "M")
	driver.Render()
	window := outlinerView.LastViewport
	if window.VisibleRows <= 1 {
		t.Fatalf("the baseline window draws %d rows, too few to prove the list was drawn", window.VisibleRows)
	}
	rowTitle := firstVisibleTaskTitle(t, outlinerView)
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, rowTitle) {
		t.Fatalf("the baseline frame does not draw the row it named:\n%s", screen)
	}

	pressMenuBarKey(t, driver, "ctrl+m")
	open := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(open, "Capture Child") {
		t.Fatalf("the dropdown is not drawn:\n%s", open)
	}
	if !strings.Contains(open, "Task") || !strings.Contains(open, "Help") {
		t.Errorf("the bar left the frame the menu hangs off:\n%s", open)
	}
	if strings.Contains(open, "Active Projects · All") {
		t.Errorf("the operational title is still drawn under an open menu:\n%s", open)
	}
	if strings.Contains(open, rowTitle) {
		t.Errorf("the list is still drawn under an open menu:\n%s", open)
	}
	if got := outlinerView.LastViewport; got != window {
		t.Errorf("opening a menu moved the window from %+v to %+v", window, got)
	}

	pressMenuBarKey(t, driver, "esc")
	closed := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(closed, "Active Projects · All") {
		t.Errorf("the title did not come back:\n%s", closed)
	}
	if !strings.Contains(closed, rowTitle) {
		t.Errorf("the list did not come back:\n%s", closed)
	}
	if got := outlinerView.LastViewport; got != window {
		t.Errorf("closing the menu redrew window %+v, want the one it left at %+v", got, window)
	}
}

// A dropdown reads the way a desktop menu does: labels down the left, key hints
// flushed to one right column. Rows of very different label widths are what make
// that claim worth asserting, and the padding it takes is only honest if the
// hint it pushes right is still whole, so both are checked on the drawn frame.
func TestMenuDropdownFlushesEveryKeyHintToOneColumn(t *testing.T) {
	driver, _ := menuBarDriver(t)
	pressMenuBarKey(t, driver, "M", "ctrl+m")
	screen := driver.Observe(ObserveOptions{}).PlainScreen

	hints := []struct{ label, key string }{
		{"Capture", "a"},
		{"External Editor", "Ctrl+E"},
		{"Copy Title and Description", "Ctrl+Y"},
	}
	column := -1
	for _, hint := range hints {
		line := ""
		for _, candidate := range strings.Split(screen, "\n") {
			if strings.Contains(candidate, hint.label) {
				line = candidate
				break
			}
		}
		if line == "" {
			t.Fatalf("the open menu draws no %q row:\n%s", hint.label, screen)
		}
		// The hint is looked for after the label so a one-character key cannot
		// match a letter of the label itself.
		rest := line[strings.Index(line, hint.label)+len(hint.label):]
		offset := strings.Index(rest, hint.key)
		if offset < 0 {
			t.Fatalf("the %q row lost its %q hint: %q", hint.label, hint.key, line)
		}
		if offset == 0 {
			t.Errorf("the %q row runs its hint straight into the label: %q", hint.label, line)
		}
		end := lipgloss.Width(line[:len(line)-len(rest)+offset+len(hint.key)])
		if column < 0 {
			column = end
			continue
		}
		if end != column {
			t.Errorf("the %q hint ends at column %d, want the %d every other hint ends at:\n%s",
				hint.label, end, column, screen)
		}
	}
}

// The whole render-budget claim in one place: showing the bar costs exactly one
// list row, the bar sits above the operational title rather than replacing it,
// and the choice survives a relaunch because it lives in machine.json.
func TestMenuBarCostsExactlyOneListRowAndPersists(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	outlinerView := driver.App().crossOutliner
	if outlinerView == nil {
		t.Fatal("the rich fixture launched without a cross-project Outliner")
	}
	// lastViewport is the window the last render actually drew, so the baseline
	// has to be taken after one.
	driver.Render()
	hiddenHeight := outlinerView.ContentHeight()
	hiddenRows := outlinerView.LastViewport.VisibleRows
	if hiddenRows <= 1 {
		t.Fatalf("the launch viewport draws %d rows, too few to prove a row was spent", hiddenRows)
	}

	if err := driver.SendKey("M"); err != nil {
		t.Fatalf("press M: %v", err)
	}
	if !driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Fatal("M did not commit the machine-local visibility flag")
	}
	if got := outlinerView.ContentHeight(); got != hiddenHeight-1 {
		t.Errorf("showing the bar changed the row budget from %d to %d, want exactly one row less", hiddenHeight, got)
	}

	screen := driver.Observe(ObserveOptions{}).PlainScreen
	lines := strings.Split(screen, "\n")
	titleRow := -1
	barRow := -1
	for index, line := range lines {
		if titleRow < 0 && strings.Contains(line, "Active Projects · All") {
			titleRow = index
		}
		if barRow < 0 && strings.Contains(line, "Task") && strings.Contains(line, "View") &&
			strings.Contains(line, "Help") {
			barRow = index
		}
	}
	if barRow < 0 {
		t.Fatalf("the bar is not on screen:\n%s", screen)
	}
	if titleRow < 0 {
		t.Fatalf("the operational title left the screen:\n%s", screen)
	}
	if barRow+1 != titleRow {
		t.Errorf("the bar is on line %d and the title on line %d, want the bar immediately above it", barRow, titleRow)
	}
	if got := outlinerView.LastViewport.VisibleRows; got != hiddenRows-1 {
		t.Errorf("the drawn window holds %d rows, want %d", got, hiddenRows-1)
	}

	if err := driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Fatal("the bar did not survive a relaunch")
	}
	if reopened := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(reopened, "Find") {
		t.Errorf("the reopened Outliner draws no bar:\n%s", reopened)
	}

	if err := driver.SendKey("M"); err != nil {
		t.Fatalf("press M again: %v", err)
	}
	if driver.App().domain.runtime.Machine().MenuBarVisible {
		t.Fatal("a second M did not hide the bar again")
	}
	if got := driver.App().crossOutliner.ContentHeight(); got != hiddenHeight {
		t.Errorf("hiding the bar left the budget at %d, want the original %d", got, hiddenHeight)
	}
}

// firstVisibleTaskTitle names a row the baseline frame drew, so the assertion
// that the menu hid the list is made against a row that was demonstrably on
// screen rather than against the fixture's contents in general.
func firstVisibleTaskTitle(t *testing.T, outlinerView *outliner.OutlinerModel) string {
	t.Helper()
	start, end := outlinerView.VisibleRowWindow()
	for index := start; index < end; index++ {
		if row := outlinerView.Rows[index]; row.Task != nil {
			return row.Task.Title
		}
	}
	t.Fatal("the visible window holds no task row")
	return ""
}
