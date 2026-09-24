package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// The bar is chrome with a budget: it must be exactly as wide as every other
// Outliner line, it must cost exactly the one row contentHeight subtracts, and
// it must drop whole menus rather than truncate a label. These are the
// assertions that hold it to all three.

// An open menu is a popup over the view it was opened from: the bar row, the
// operational title, and every row the box does not reach stay exactly where the
// last list frame put them, dimmed. The box covers rows rather than displacing
// them, so nothing steps down and back up once per menu press, and the window
// the last list frame drew survives untouched — it is the one the close redraws.
func TestAnOpenMenuStandsOnTheViewItWasOpenedFrom(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)

	// Show the bar first: revealing it costs a list row, and this test is about
	// what opening a menu does on top of a bar that is already up.
	pressMenuBarKey(t, driver, "M")
	driver.Render()
	window := outlinerView.LastViewport
	if window.VisibleRows <= 1 {
		t.Fatalf("the baseline window draws %d rows, too few to prove the list was drawn", window.VisibleRows)
	}
	baseline := driver.Observe(ObserveOptions{}).PlainScreen

	pressMenuBarKey(t, driver, "ctrl+m")
	open := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(open, "Capture Child") {
		t.Fatalf("the dropdown is not drawn:\n%s", open)
	}
	if !strings.Contains(open, "Task") || !strings.Contains(open, "Help") {
		t.Errorf("the bar left the frame the menu hangs off:\n%s", open)
	}
	if !strings.Contains(open, "Active Projects · All") {
		t.Errorf("the operational title was covered by an open menu:\n%s", open)
	}
	assertHostSurvivesBesideDropdown(t, baseline, open)
	if !strings.Contains(driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, dimSequence(t)) {
		t.Errorf("the host under an open menu is not dimmed:\n%s", open)
	}
	if got := outlinerView.LastViewport; got != window {
		t.Errorf("opening a menu moved the window from %+v to %+v", window, got)
	}

	// The menu owns the keys it lists. A cursor key inside it moves its own
	// highlight and nothing on the view it is standing on.
	cursor := outlinerView.Cursor
	pressMenuBarKey(t, driver, "down")
	moved := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(moved, "› Capture Child") {
		t.Errorf("the menu did not take its own cursor key:\n%s", moved)
	}
	if got := outlinerView.Cursor; got != cursor {
		t.Errorf("the menu's cursor key reached the list under it: cursor %d, want %d", got, cursor)
	}
	if got := outlinerView.LastViewport; got != window {
		t.Errorf("moving inside the menu moved the window from %+v to %+v", window, got)
	}

	pressMenuBarKey(t, driver, "esc")
	closed := driver.Observe(ObserveOptions{}).PlainScreen
	if closed != baseline {
		t.Errorf("the close did not redraw the frame the menu was opened on:\nwant:\n%s\ngot:\n%s", baseline, closed)
	}
	if strings.Contains(closed, "Capture Child") {
		t.Errorf("the dropdown outlived its close:\n%s", closed)
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

	// The toggle also raises "Menu bar shown", and a footer carrying a message is
	// a row taller than one that is not — a cost the shell measures and the list
	// pays, exactly as it already pays for the status strip. This test is about
	// the bar's own row, so the message is retired before the window is read. The
	// resize is what re-measures: the budget is published once per Update rather
	// than per frame, so clearing the message alone would leave the stale one.
	driver.App().feedback.message = ""
	driver.Resize(100, 35)
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

// assertHostSurvivesBesideDropdown is the whole claim in one comparison: on every
// row the box hangs over, the columns it does not reach still read exactly as the
// frame before it opened. A view that had been replaced, shrunk, or rewrapped
// could not satisfy that, and naming one title could not prove it, since a narrow
// fixture's rows fit behind the box entirely.
func assertHostSurvivesBesideDropdown(t *testing.T, baseline, open string) {
	t.Helper()
	baseLines := strings.Split(baseline, "\n")
	openLines := strings.Split(open, "\n")
	top, bottom, right := -1, -1, -1
	for index, line := range openLines {
		corner := strings.Index(line, "╮")
		switch {
		case top < 0 && strings.Contains(line, "╭") && corner >= 0:
			top, right = index, lipgloss.Width(line[:corner])
		case top >= 0 && strings.Contains(line, "╰") && strings.Contains(line, "╯"):
			bottom = index
		}
	}
	if top < 0 || bottom < 0 {
		t.Fatalf("the open frame draws no dropdown box:\n%s", open)
	}
	// The cell right of the corner is the box's own halo; the host resumes after it.
	hostStart := right + 2
	for row := top; row <= bottom && row < len(baseLines) && row < len(openLines); row++ {
		width := lipgloss.Width(openLines[row])
		want := ansi.Cut(baseLines[row], hostStart, width)
		if got := ansi.Cut(openLines[row], hostStart, width); got != want {
			t.Errorf("row %d lost the host beside the box: %q, want %q", row, got, want)
		}
	}
}

// dimSequence is what DimContent writes, read off the theme rather than spelled
// as an escape, so the assertion follows a palette change instead of breaking on
// one.
func dimSequence(t *testing.T) string {
	t.Helper()
	rendered := theme.DimStyle.Render("x")
	prefix, _, found := strings.Cut(rendered, "x")
	if !found || prefix == "" {
		t.Fatal("the dim style renders no escape to look for")
	}
	return prefix
}
