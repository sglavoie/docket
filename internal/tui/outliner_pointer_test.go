package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/outliner"
)

// The Outliner's pointer answer: a click is j/k motion onto the row the last
// render drew under it, a fold glyph is h or l, a double-click is Enter and a
// right-click is Space, each through the key's own path.

func outlinerPointerDriver(t *testing.T, fixture string) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(fixture, 60, 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver
}

func pointerOutliner(t *testing.T, driver *UITestDriver) *outliner.OutlinerModel {
	t.Helper()
	out, ok := driver.App().outlinerPrimary()
	if !ok || out == nil {
		t.Fatal("no Outliner is mounted")
	}
	return out
}

func pointerClickAt(t *testing.T, driver *UITestDriver, x, y int, button string, count int) {
	t.Helper()
	if err := driver.runPointer(UITestAction{Type: UITestActionClick, X: &x, Y: &y, Button: button, Count: count}); err != nil {
		t.Fatal(err)
	}
}

// pointerRowY is the screen row the rendered screen draws a literal on.
func pointerRowY(t *testing.T, driver *UITestDriver, literal string) int {
	t.Helper()
	_, y, err := driver.pointerCell(UITestAction{On: literal})
	if err != nil {
		t.Fatal(err)
	}
	return y
}

func pointerKey(t *testing.T, driver *UITestDriver, key string) {
	t.Helper()
	if err := driver.SendKey(key); err != nil {
		t.Fatal(err)
	}
	driver.drain()
}

// pointerChecklistDriver opens the checklist fixture's Backlog lane and the
// runbook's checklist, so every row kind it holds is drawn.
func pointerChecklistDriver(t *testing.T) *UITestDriver {
	t.Helper()
	driver := outlinerPointerDriver(t, FixtureChecklist)
	pointerClickAt(t, driver, 0, pointerRowY(t, driver, "Backlog"), "", 1)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, checklistFixtureFoldTitle), "", 1)
	pointerKey(t, driver, "l")
	return driver
}

func TestClickSeatsTheCursorOnEveryRowKind(t *testing.T) {
	driver := pointerChecklistDriver(t)
	out := pointerOutliner(t, driver)
	cases := []struct {
		literal string
		holds   func(outliner.OutlinerRow) bool
	}{
		{"Backlog", func(row outliner.OutlinerRow) bool { return row.IsSectionHeader }},
		{"[checklist-delivery]", func(row outliner.OutlinerRow) bool { return row.IsProjectHeader }},
		{checklistFixtureSecondItem, func(row outliner.OutlinerRow) bool { return row.IsChecklistItem }},
		{checklistFixtureParentTitle, func(row outliner.OutlinerRow) bool {
			return row.Task != nil && row.Task.Title == checklistFixtureParentTitle
		}},
	}
	for _, tc := range cases {
		pointerClickAt(t, driver, 30, pointerRowY(t, driver, tc.literal), "", 1)
		if out.Cursor < 0 || out.Cursor >= len(out.Rows) || !tc.holds(out.Rows[out.Cursor]) {
			t.Errorf("a click on %q left the cursor on row %d", tc.literal, out.Cursor)
		}
	}
}

func TestClickSeatsTheCursorOnMergedRankedNoteAndEmptyStateRows(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureRich)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 30, pointerRowY(t, driver, "Triage support inbox"), "", 1)
	if row := out.Rows[out.Cursor]; !row.LeadsMergedGroup || row.Task == nil || row.Task.Title != "Triage support inbox" {
		t.Fatalf("a click on a merged group's row left the cursor on %+v", row)
	}

	for _, key := range []string{"/", "b", "r", "i", "e", "f", "enter"} {
		pointerKey(t, driver, key)
	}
	out.Cursor = -1
	pointerClickAt(t, driver, 30, pointerRowY(t, driver, "Workspace launch brief"), "", 1)
	if out.Cursor < 0 || !out.Rows[out.Cursor].IsNoteRow {
		t.Fatalf("a click on a ranked note row left the cursor on %d", out.Cursor)
	}

	driver = outlinerPointerDriver(t, FixtureRich)
	out = pointerOutliner(t, driver)
	for _, key := range []string{"g", "p", "/", "z", "q", "x", "enter"} {
		pointerKey(t, driver, key)
	}
	if out.HasContentRows() {
		t.Fatal("the search was meant to leave an empty projection")
	}
	header := pointerRowY(t, driver, "[rich-project]")
	out.Cursor = 3
	pointerClickAt(t, driver, 30, header+1, "", 1)
	if out.Cursor != 3 {
		t.Fatalf("a click on the empty state's explanation moved the cursor to %d", out.Cursor)
	}
	pointerClickAt(t, driver, 30, header, "", 1)
	if out.Cursor != 0 || !out.Rows[0].IsFocusedProjectHeader {
		t.Fatalf("a click on the empty state's header left the cursor on %d", out.Cursor)
	}
}

func TestClickOnChromeAndPaddingDoesNothing(t *testing.T) {
	driver := pointerChecklistDriver(t)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 30, pointerRowY(t, driver, checklistFixtureParentTitle), "", 1)
	want := out.Cursor
	before := driver.Render()
	for _, y := range []int{0, 15, 18, 19} {
		pointerClickAt(t, driver, 30, y, "", 1)
		pointerClickAt(t, driver, 30, y, "right", 1)
		if out.Cursor != want {
			t.Fatalf("a click on screen row %d moved the cursor to %d", y, out.Cursor)
		}
	}
	if after := driver.Render(); after != before {
		t.Fatalf("clicks on chrome changed the screen:\n%s", after)
	}
}

func TestClickOnThePinnedContextRowSeatsItsHeader(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureBusy)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 0, pointerRowY(t, driver, "Backlog"), "", 1)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, "Untriaged inbox item 1"), "", 1)
	pointerKey(t, driver, "b")
	pointerKey(t, driver, "G")
	y := pointerRowY(t, driver, "Inbox › [inbox-backlog]")
	pointerClickAt(t, driver, 20, y, "", 1)
	if out.Cursor < 0 || !out.Rows[out.Cursor].IsSectionHeader || out.Rows[out.Cursor].SectionName != "Inbox" {
		t.Fatalf("a click on the pinned context row left the cursor on %d", out.Cursor)
	}
	// The dock below the list is not a row either.
	want := out.Cursor
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, "No detail to show"), "", 1)
	if out.Cursor != want {
		t.Fatalf("a click on the Peek dock moved the cursor to %d", out.Cursor)
	}
}

// pointerTwins returns two drivers on one fixture, so a click and the key it
// re-expresses can be compared screen for screen.
func pointerTwins(t *testing.T, fixture string, setup func(*UITestDriver)) (*UITestDriver, *UITestDriver) {
	t.Helper()
	clicked, keyed := outlinerPointerDriver(t, fixture), outlinerPointerDriver(t, fixture)
	setup(clicked)
	setup(keyed)
	return clicked, keyed
}

func TestDoubleClickAndRightClickAnswerAsEnterAndSpace(t *testing.T) {
	cases := []struct {
		name, fixture string
		setup         func(*UITestDriver)
		literal       string
		button, key   string
		count         int
	}{
		{"right-click on a section header refuses as Space does", FixtureChecklist, func(*UITestDriver) {}, "Backlog", "right", "space", 1},
		{"right-click on a task opens its actions", FixtureChecklist, func(d *UITestDriver) {
			pointerClickAt(t, d, 0, pointerRowY(t, d, "Backlog"), "", 1)
		}, checklistFixturePlainTitle, "right", "space", 1},
		{"double-click on a leaf task refuses as Enter does", FixtureChecklist, func(d *UITestDriver) {
			pointerClickAt(t, d, 0, pointerRowY(t, d, "Backlog"), "", 1)
		}, checklistFixturePlainTitle, "", "enter", 2},
		{"double-click on a parent focuses it", FixtureChecklist, func(d *UITestDriver) {
			pointerClickAt(t, d, 0, pointerRowY(t, d, "Backlog"), "", 1)
		}, checklistFixtureParentTitle, "", "enter", 2},
		{"right-click on a note row refuses as Space does", FixtureRich, func(d *UITestDriver) {
			for _, key := range []string{"/", "b", "r", "i", "e", "f", "enter"} {
				pointerKey(t, d, key)
			}
		}, "Workspace launch brief", "right", "space", 1},
		{"double-click on a note row opens Notes", FixtureRich, func(d *UITestDriver) {
			for _, key := range []string{"/", "b", "r", "i", "e", "f", "enter"} {
				pointerKey(t, d, key)
			}
		}, "Workspace launch brief", "", "enter", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clicked, keyed := pointerTwins(t, tc.fixture, tc.setup)
			y := pointerRowY(t, clicked, tc.literal)
			pointerClickAt(t, clicked, 30, y, tc.button, tc.count)
			pointerClickAt(t, keyed, 30, y, "", 1)
			pointerKey(t, keyed, tc.key)
			if got, want := clicked.Render(), keyed.Render(); got != want {
				t.Fatalf("the click drew\n%s\nwhere %s drew\n%s", got, tc.key, want)
			}
		})
	}
}

func TestClickOnAFoldGlyphTogglesThatFoldAsHAndLWould(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureChecklist)
	out := pointerOutliner(t, driver)
	backlog := pointerRowY(t, driver, "Backlog")
	pointerClickAt(t, driver, 0, backlog, "", 1)
	if !hasLiteral(driver, checklistFixturePlainTitle) {
		t.Fatal("a click on a collapsed section's glyph did not expand it")
	}

	// A task's glyph sits after its tree prefix: the runbook's checklist opens and
	// closes on it, and the cursor lands on the runbook either way.
	runbook := pointerRowY(t, driver, checklistFixtureFoldTitle)
	glyph := pointerGlyphX(t, driver, runbook)
	pointerClickAt(t, driver, glyph, runbook, "", 1)
	if !hasLiteral(driver, checklistFixtureSecondItem) {
		t.Fatal("a click on a task's collapsed glyph did not open its checklist")
	}
	pointerClickAt(t, driver, glyph+1, runbook, "", 1)
	if hasLiteral(driver, checklistFixtureSecondItem) {
		t.Fatal("a click beside a task's open glyph did not close its checklist")
	}
	if row := out.Rows[out.Cursor]; row.Task == nil || row.Task.Title != checklistFixtureFoldTitle {
		t.Fatalf("the fold click left the cursor on %d", out.Cursor)
	}

	// The glyph column of a row with nothing to fold is a plain click.
	plain := pointerRowY(t, driver, checklistFixturePlainTitle)
	before := len(out.Rows)
	pointerClickAt(t, driver, glyph, plain, "", 1)
	if len(out.Rows) != before || out.Rows[out.Cursor].Task.Title != checklistFixturePlainTitle {
		t.Fatal("a click in a fold-less row's glyph column did more than seat the cursor")
	}

	// h on a root task folds its group and remembers it; the header's glyph is l,
	// which puts the cursor back on the task the fold was covering.
	pointerKey(t, driver, "h")
	header := pointerRowY(t, driver, "[checklist-delivery]")
	pointerClickAt(t, driver, 1, header, "", 1)
	if row := out.Rows[out.Cursor]; row.Task == nil || row.Task.Title != checklistFixturePlainTitle {
		t.Fatalf("expanding the group by its glyph lost the remembered task, cursor on %d", out.Cursor)
	}
	pointerClickAt(t, driver, 1, header, "", 1)
	if hasLiteral(driver, checklistFixturePlainTitle) {
		t.Fatal("a click on an open project header's glyph did not fold it")
	}
}

func TestFoldGlyphClickRefusesWhereTheKeysDo(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureChecklist)
	out := pointerOutliner(t, driver)
	pointerKey(t, driver, "enter")
	if !out.Focus.Zoom.Active() {
		t.Fatal("Enter on the section header was meant to zoom it")
	}
	backlog := pointerRowY(t, driver, "▾  Backlog")
	before := driver.Render()
	pointerClickAt(t, driver, 0, backlog, "", 1)
	if after := driver.Render(); after != before {
		t.Fatalf("a click on a zoomed section's glyph changed the screen, as h would not:\n%s", after)
	}
}

func TestClickUnderAVisualSelectionIsMotionOnly(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureChecklist)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 0, pointerRowY(t, driver, "Backlog"), "", 1)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, checklistFixtureFoldTitle), "", 1)

	// An active range stretches from its anchor to the clicked row, exactly as
	// the j/k presses between them would leave it.
	pointerKey(t, driver, "V")
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, "Confirm the staging soak"), "", 1)
	if got := len(out.SelectedTaskIdentities()); got != 3 {
		t.Fatalf("a click stretched the range to %d tasks, want 3", got)
	}
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, checklistFixtureParentTitle), "", 1)
	if got := len(out.SelectedTaskIdentities()); got != 2 {
		t.Fatalf("a click back up shrank the range to %d tasks, want 2", got)
	}

	// A frozen selection is not reshaped by motion: the cursor moves, the
	// members stay.
	pointerKey(t, driver, "esc")
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, checklistFixturePlainTitle), "", 1)
	if got := len(out.SelectedTaskIdentities()); got != 2 || !out.IsMultiSelectActive() {
		t.Fatalf("a click under a frozen selection left %d members", got)
	}
	if out.Rows[out.Cursor].Task.Title != checklistFixturePlainTitle {
		t.Fatal("a click under a frozen selection did not move the cursor")
	}
}

func TestClickPushesNoJumpFrame(t *testing.T) {
	driver := outlinerPointerDriver(t, FixtureChecklist)
	out := pointerOutliner(t, driver)
	pointerClickAt(t, driver, 0, pointerRowY(t, driver, "Backlog"), "", 1)
	before := len(out.RelocationHistory.Back)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, checklistFixturePlainTitle), "", 1)
	pointerClickAt(t, driver, 20, pointerRowY(t, driver, checklistFixtureFoldTitle), "", 1)
	if got := len(out.RelocationHistory.Back); got != before {
		t.Fatalf("clicks pushed %d jump frames; a click is navigation, as j/k is", got-before)
	}
}

// pointerGlyphX is the column of the fold glyph a screen row draws.
func pointerGlyphX(t *testing.T, driver *UITestDriver, y int) int {
	t.Helper()
	line := []rune(screenLines(driver)[y])
	for x, r := range line {
		if r == '▸' || r == '▾' {
			return x
		}
	}
	t.Fatalf("screen row %d draws no fold glyph", y)
	return 0
}

func screenLines(driver *UITestDriver) []string {
	return strings.Split(driver.Render(), "\n")
}

func hasLiteral(driver *UITestDriver, literal string) bool {
	return strings.Contains(driver.Render(), literal)
}
