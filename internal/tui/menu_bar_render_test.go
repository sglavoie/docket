package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The bar is chrome with a budget: it must be exactly as wide as every other
// Outliner line, it must cost exactly the one row contentHeight subtracts, and
// it must drop whole menus rather than truncate a label. These are the
// assertions that hold it to all three.

func fakeMenus(titles ...string) []resolvedMenu {
	menus := make([]resolvedMenu, 0, len(titles))
	for _, title := range titles {
		menus = append(menus, resolvedMenu{
			ID:      strings.ToLower(title),
			Title:   title,
			Entries: []resolvedMenuEntry{{OperationID: "test.menu.entry", Label: title}},
		})
	}
	return menus
}

func menuTitles(boxes []menuLabelBox) []string {
	titles := make([]string, 0, len(boxes))
	for _, box := range boxes {
		titles = append(titles, box.Title)
	}
	return titles
}

func TestMenuBarLayoutPlacesEveryLabelEndToEnd(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go", "Find", "Help")
	boxes := menuBarLayout(menus, 100)
	if len(boxes) != len(menus) {
		t.Fatalf("laid out %d labels at a width that fits them all, want %d", len(boxes), len(menus))
	}

	style := menuBarMeasureStyle()
	col := 0
	for index, box := range boxes {
		if box.Title != menus[index].Title {
			t.Errorf("box %d is %q, want the menu order's %q", index, box.Title, menus[index].Title)
		}
		if box.StartCol != col {
			t.Errorf("box %q starts at column %d, want %d", box.Title, box.StartCol, col)
		}
		if want := lipgloss.Width(style.Render(box.Title)); box.Width != want {
			t.Errorf("box %q is %d wide, want the padded cell's %d", box.Title, box.Width, want)
		}
		col += box.Width
	}
}

// A dropped menu is still reachable by the accelerators its entries carry; a
// truncated label is a lie about what the bar offers. So the ladder sheds Help,
// then Find, then Go, and never cuts a word in half.
func TestMenuBarLayoutDropsWholeLabelsFromTheRight(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go", "Find", "Help")
	full := menuBarLayout(menus, 100)
	total := 0
	for _, box := range full {
		total += box.Width
	}

	cases := []struct {
		width int
		want  []string
	}{
		{width: total, want: []string{"Task", "View", "Go", "Find", "Help"}},
		{width: total - 1, want: []string{"Task", "View", "Go", "Find"}},
		{width: full[0].Width + full[1].Width + full[2].Width, want: []string{"Task", "View", "Go"}},
		{width: full[0].Width + full[1].Width, want: []string{"Task", "View"}},
		{width: full[0].Width, want: []string{"Task"}},
		// Narrower than one label, the first menu still stands: the full-width fit
		// every Outliner line goes through is what clips it, not a half-word.
		{width: 2, want: []string{"Task"}},
	}
	for _, tc := range cases {
		got := menuTitles(menuBarLayout(menus, tc.width))
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("width %d laid out %v, want %v", tc.width, got, tc.want)
		}
	}

	if boxes := menuBarLayout(menus, 0); boxes != nil {
		t.Errorf("a zero width laid out %v, want nothing", menuTitles(boxes))
	}
	if boxes := menuBarLayout(nil, 100); boxes != nil {
		t.Errorf("an empty bar laid out %v, want nothing", menuTitles(boxes))
	}
}

func TestRenderMenuBarRowIsExactlyOneLineOfTheGivenWidth(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go", "Find", "Help")
	for _, width := range []int{100, 60, 30, 12} {
		row := renderMenuBarRow(menus, width, -1)
		if strings.Contains(row, "\n") {
			t.Errorf("width %d produced more than one row", width)
		}
		if got := lipgloss.Width(row); got != width {
			t.Errorf("width %d rendered %d cells", width, got)
		}
	}
}

// The open label is the only thing on this row that changes appearance, and it
// has to line up with the column the layout assigned it, so the highlight is
// decided beside the placement rather than by a second walk of the menus.
func TestRenderMenuBarRowMarksOnlyTheOpenMenu(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go")
	closed := renderMenuBarRow(menus, 60, -1)
	for openIdx := 0; openIdx < len(menus); openIdx++ {
		row := renderMenuBarRow(menus, 60, openIdx)
		if row == closed {
			t.Errorf("opening menu %d rendered the same row as an inert bar", openIdx)
		}
		if got := lipgloss.Width(row); got != 60 {
			t.Errorf("opening menu %d rendered %d cells, want 60", openIdx, got)
		}
		if plain := ansi.Strip(row); !strings.Contains(plain, menus[openIdx].Title) {
			t.Errorf("opening menu %d lost its label: %q", openIdx, plain)
		}
	}
}

// The View menu's own row is the one thing a user who found the bar by accident
// cannot read off the bar itself, so it has to describe live visibility rather
// than a constant.
func TestViewMenuBarRowCarriesLiveVisibility(t *testing.T) {
	for _, visible := range []bool{false, true} {
		var found bool
		for _, entry := range viewMenuEntries(ModeAll, visible) {
			if entry.OperationID != operationCoreToggleMenuBar {
				continue
			}
			found = true
			if entry.Label != "Menu Bar" {
				t.Errorf("the bar's own row is labeled %q", entry.Label)
			}
			if entry.Checked == nil {
				t.Fatal("the bar's own row carries no checkbox state")
			}
			if *entry.Checked != visible {
				t.Errorf("visible=%v resolved checked=%v", visible, *entry.Checked)
			}
		}
		if !found {
			t.Fatalf("visible=%v: the View menu does not offer the bar's own toggle", visible)
		}
	}
}

// An open menu is the whole frame: the bar row, the block under its label, and
// nothing else. The list is not shrunk to make room for a dropdown, it is not
// drawn, which is what stops every row on screen from stepping down and back up
// once per menu press. The window the last list frame drew has to survive that
// untouched, since it is the one the close redraws.
func TestAnOpenMenuDrawsNoOutlinerAndMovesNoWindow(t *testing.T) {
	driver, outliner := menuBarDriver(t)

	// Show the bar first: revealing it costs a list row, and this test is about
	// what opening a menu does on top of a bar that is already up.
	pressMenuBarKey(t, driver, "M")
	driver.Render()
	window := outliner.lastViewport
	if window.visibleRows <= 1 {
		t.Fatalf("the baseline window draws %d rows, too few to prove the list was drawn", window.visibleRows)
	}
	rowTitle := firstVisibleTaskTitle(t, outliner)
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
	if got := outliner.lastViewport; got != window {
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
	if got := outliner.lastViewport; got != window {
		t.Errorf("closing the menu redrew window %+v, want the one it left at %+v", got, window)
	}
}

// firstVisibleTaskTitle names a row the baseline frame drew, so the assertion
// that the menu hid the list is made against a row that was demonstrably on
// screen rather than against the fixture's contents in general.
func firstVisibleTaskTitle(t *testing.T, outliner *OutlinerModel) string {
	t.Helper()
	start, end := outliner.visibleRowWindow()
	for index := start; index < end; index++ {
		if row := outliner.rows[index]; row.task != nil {
			return row.task.Title
		}
	}
	t.Fatal("the visible window holds no task row")
	return ""
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

	outliner := driver.App().crossOutliner
	if outliner == nil {
		t.Fatal("the rich fixture launched without a cross-project Outliner")
	}
	// lastViewport is the window the last render actually drew, so the baseline
	// has to be taken after one.
	driver.Render()
	hiddenHeight := outliner.contentHeight()
	hiddenRows := outliner.lastViewport.visibleRows
	if hiddenRows <= 1 {
		t.Fatalf("the launch viewport draws %d rows, too few to prove a row was spent", hiddenRows)
	}

	if err := driver.SendKey("M"); err != nil {
		t.Fatalf("press M: %v", err)
	}
	if !driver.App().runtime.Machine().MenuBarVisible {
		t.Fatal("M did not commit the machine-local visibility flag")
	}
	if got := outliner.contentHeight(); got != hiddenHeight-1 {
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
	if got := outliner.lastViewport.visibleRows; got != hiddenRows-1 {
		t.Errorf("the drawn window holds %d rows, want %d", got, hiddenRows-1)
	}

	if err := driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !driver.App().runtime.Machine().MenuBarVisible {
		t.Fatal("the bar did not survive a relaunch")
	}
	if reopened := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(reopened, "Find") {
		t.Errorf("the reopened Outliner draws no bar:\n%s", reopened)
	}

	if err := driver.SendKey("M"); err != nil {
		t.Fatalf("press M again: %v", err)
	}
	if driver.App().runtime.Machine().MenuBarVisible {
		t.Fatal("a second M did not hide the bar again")
	}
	if got := driver.App().crossOutliner.contentHeight(); got != hiddenHeight {
		t.Errorf("hiding the bar left the budget at %d, want the original %d", got, hiddenHeight)
	}
}
