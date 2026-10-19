package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The four sizes the UX refinement is reviewed at. 60x20 is the app's own floor
// (chrome.MinTerminalWidth by chrome.MinTerminalHeight), 120x40 is the widest
// terminal the work claims to spend well, and the two in between are the sizes
// the program optimizes for. They are spelled here once so a size added to the
// review is added to every check below at once.
var uxPresentationMatrixSizes = [][2]int{{60, 20}, {80, 24}, {100, 35}, {120, 40}}

// uxPeekDockSpan finds the dock on a plain screen as the block of rows it says
// it drew, divider first, and returns the rows of its divider and of its last
// row, the position row. It matches the whole block rather than the position
// row's text because a document that fits leaves that row blank, and matching
// every row is what tells a dock drawn whole from one clipped into the frame.
func uxPeekDockSpan(screen []string, view *outliner.OutlinerModel) (divider, position int, ok bool) {
	dock := view.PeekDockLines()
	if len(dock) == 0 {
		return -1, -1, false
	}
	divider, position = -1, -1
	for start := 0; start+len(dock) <= len(screen); start++ {
		whole := true
		for offset, line := range dock {
			if strings.TrimSpace(screen[start+offset]) != strings.TrimSpace(ansi.Strip(line)) {
				whole = false
				break
			}
		}
		if whole {
			divider, position = start, start+len(dock)-1
		}
	}
	return divider, position, divider >= 0
}

// uxFullWidthRule matches a line that is nothing but a horizontal rule, which is
// how the pinned-lane separator, the Peek divider, and the footer divider are
// all spelled. The last one on screen is the footer's.
var uxFullWidthRule = regexp.MustCompile(`^─+$`)

// TestUXRefinementPresentationMatrix is the integrated acceptance pass over the
// whole refinement: one fixture where every axis is true at once, read at the
// four reviewed sizes in dark, light, and monochrome.
//
// It asks the questions a scenario cannot ask, because each of them is about
// the relationship between two renderings rather than about the text of one:
// that no line spends more display cells than the terminal has (which a
// double-width rune makes a different question from counting runes), that the
// list fills the window the viewport authority handed it, that the rows it
// fills it with are mostly work rather than chrome, that Peek occupies the same
// rows whatever the cursor stands on, that a title a row clipped is whole in
// Peek at every width, and that removing color removes no content.
func TestUXRefinementPresentationMatrix(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	for _, variant := range []string{"dark", "light"} {
		for _, size := range uxPresentationMatrixSizes {
			t.Run(fmt.Sprintf("%s-%dx%d", variant, size[0], size[1]), func(t *testing.T) {
				driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, size[0], size[1])
				if err != nil {
					t.Fatalf("fixture: %v", err)
				}
				t.Cleanup(func() { _ = workspace.Cleanup() })
				theme.ApplyTheme(theme.ThemeConfig{Name: variant})
				view := driver.App().crossOutliner
				if view == nil {
					t.Fatal("the fixture app has no Outliner")
				}

				assertUXLaunchPresentation(t, driver, view, size)
				assertUXHierarchyReadsByDepth(t, driver, view)
				assertUXSelectionPresentation(t, driver, view)
				assertUXPeekGeometryIsStable(t, driver, view)
				assertUXPeekCarriesTheWholeTitle(t, driver, view, size)
			})
		}
	}
}

// assertUXLaunchPresentation reads the launch view, which is the one screen a
// user sees before pressing anything and therefore the one the whole refinement
// is judged on.
func assertUXLaunchPresentation(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, size [2]int) {
	t.Helper()
	obs := driver.Observe(ObserveOptions{IncludeANSI: true})
	assertUXScreenFitsItsTerminal(t, obs, size)
	assertUXMonochromeKeepsContent(t, obs,
		// The operational title, one lane header, and a project heading. All
		// three are drawn at every width, so they survive the narrowest terminal
		// and the ASCII profile alike; the workspace pulse is not, since its
		// ladder drops entries.
		"Active Projects · All",
		"Doing  (",
		"[ux-",
	)

	// Every cross-project group names its project exactly once: a heading, or,
	// for an expanded group of one top-level task, the merged row that leads
	// with the label in the heading's place. A group with neither is anonymous
	// and a group with both says it twice. Groups are counted over the whole
	// projection, because the window only decides how much of it is on screen.
	plain := obs.PlainScreen
	groups := map[outliner.OutlinerRowGroup]bool{}
	carriers := map[outliner.OutlinerRowGroup]int{}
	for _, row := range view.Rows {
		switch {
		case row.IsProjectHeader:
			carriers[row.Group]++
		case row.IsTaskRow() && row.Group.ProjectGrouped():
			groups[row.Group] = true
			if row.LeadsMergedGroup {
				carriers[row.Group]++
			}
		}
	}
	if len(groups) == 0 {
		t.Fatal("the launch projection grouped no rows by project")
	}
	for group := range groups {
		if carriers[group] != 1 {
			t.Fatalf("group %+v named its project %d times, want exactly once", group, carriers[group])
		}
	}

	// The list fills the window the viewport authority handed it. A projection
	// shorter than the window is padded to it, so the honest expectation is the
	// smaller of the two, and anything less means a row was reserved twice.
	drawn := uxDrawnListLines(t, plain)
	want := min(len(view.Rows), view.ContentHeight())
	if drawn != want {
		t.Fatalf("the list drew %d rows of a %d-row window over a %d-row projection\n%s",
			drawn, view.ContentHeight(), len(view.Rows), plain)
	}
}

// assertUXHierarchyReadsByDepth reads the indent ladder back off the screen in
// the one mode where all three generations share a group, which is where depth
// is drawn rather than implied. Depth is the one thing a compact row may not pay
// for: the labels, the indicators, and the title all shrink with the terminal,
// and the connectors do not. The check also records the counterpart — a row
// seated under its own parent states no project, because the connectors have
// already answered and the group states its identity once.
func assertUXHierarchyReadsByDepth(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	uxSelectMode(t, driver, view, chrome.ModeBacklog)
	obs := driver.Observe(ObserveOptions{})
	if len(obs.LayoutIssues) > 0 {
		t.Fatalf("Backlog: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	// Each generation has a unique first word that survives title truncation
	// beside the aligned project column; the indent is what is under test.
	generations := []string{"Coordinate", "Stage", "Capture"}
	indents := make([]int, 0, len(generations))
	for depth, generation := range generations {
		line, ok := uxLineContaining(obs.PlainScreen, generation)
		if !ok {
			t.Fatalf("Backlog does not draw %q\n%s", generation, obs.PlainScreen)
		}
		// Depth is read off the connector rather than off the title, because the
		// title's own offset moves with the indicators and the project label a
		// row happens to spend, while the connector is the tree itself.
		indents = append(indents, ansi.StringWidth(line)-ansi.StringWidth(strings.TrimLeft(line, " │")))
		if depth > 0 && strings.Contains(line, "[ux-") {
			t.Fatalf("the row for %q names its project although it is drawn under its own parent: %q",
				generation, line)
		}
	}
	for i := 1; i < len(indents); i++ {
		if indents[i] <= indents[i-1] {
			t.Fatalf("generation %d is indented %d cells, no deeper than its parent at %d\n%s",
				i+1, indents[i], indents[i-1], obs.PlainScreen)
		}
	}
	uxSelectMode(t, driver, view, chrome.ModeAll)
}

// uxSelectMode presses Tab until the Outliner stands in want, which is how a
// user reaches a mode. Reading the mode off the model rather than off the title
// keeps the walk honest at the width where the title is compacted.
func uxSelectMode(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, want chrome.OutlinerModeID) {
	t.Helper()
	for range len(chrome.OutlinerModes) {
		if view.PrimaryMode == want {
			return
		}
		if err := driver.SendKey("tab"); err != nil {
			t.Fatalf("cycle to the %s mode: %v", want, err)
		}
	}
	t.Fatalf("Tab never reached the %s mode; it stopped in %s", want, view.PrimaryMode)
}

// assertUXSelectionPresentation begins a visual selection, which is the one
// gesture that both takes width from every row and closes Peek.
func assertUXSelectionPresentation(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceReviewTitle)
	if err := driver.SendKey("b"); err != nil {
		t.Fatalf("open Peek: %v", err)
	}
	if !view.PeekDockVisible() {
		t.Fatal("b did not open the Peek dock")
	}
	// `v` is a per-row toggle, so two rows are two presses with a move between.
	for _, key := range []string{"v", "j", "v"} {
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("selection key %q: %v", key, err)
		}
	}
	if view.PeekDockVisible() {
		t.Fatal("entering a visual selection left the Peek dock open")
	}
	obs := driver.Observe(ObserveOptions{})
	if len(obs.LayoutIssues) > 0 {
		t.Fatalf("visual selection: %+v\n%s", obs.LayoutIssues, obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "selected") {
		t.Fatalf("the selection summary is not on screen\n%s", obs.PlainScreen)
	}
	if err := driver.SendKey("esc"); err != nil {
		t.Fatalf("drop the selection: %v", err)
	}
}

// assertUXPeekGeometryIsStable opens the dock over rows whose documents have
// nothing in common — a task carrying every metadata field, a quiet Waiting
// task, and a folded project summary — and requires the dock to occupy exactly
// the same screen rows each time. That is the whole of what "a dock, not a
// block under the row" means to a reader.
func assertUXPeekGeometryIsStable(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	type dockFrame struct {
		what     string
		divider  int
		position int
	}
	frames := make([]dockFrame, 0, 3)
	record := func(what string) {
		t.Helper()
		if err := driver.SendKey("b"); err != nil {
			t.Fatalf("open Peek over the %s: %v", what, err)
		}
		if !view.PeekDockVisible() {
			t.Fatalf("b did not open the Peek dock over the %s", what)
		}
		dockRows := view.PeekDockRows()
		if lines := view.PeekDockLines(); len(lines) != dockRows {
			t.Fatalf("the dock over the %s drew %d rows, want %d", what, len(lines), dockRows)
		}
		obs := driver.Observe(ObserveOptions{})
		if len(obs.LayoutIssues) > 0 {
			t.Fatalf("Peek over the %s: %+v\n%s", what, obs.LayoutIssues, obs.PlainScreen)
		}
		screen := strings.Split(obs.PlainScreen, "\n")
		frame := dockFrame{what: what}
		var drawn bool
		frame.divider, frame.position, drawn = uxPeekDockSpan(screen, view)
		if !drawn {
			t.Fatalf("the dock over the %s is not drawn whole\n%s", what, obs.PlainScreen)
		}
		if !strings.Contains(screen[frame.divider], "─") {
			t.Fatalf("the dock over the %s has no divider %d rows above its position row\n%s",
				what, dockRows-1, obs.PlainScreen)
		}
		frames = append(frames, frame)
		if err := driver.SendKey("b"); err != nil {
			t.Fatalf("close Peek over the %s: %v", what, err)
		}
	}

	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	record("task carrying every field")
	seatCursorOnTaskTitled(t, view, uxAcceptanceWaitingTitle)
	record("quiet Waiting task")
	onProjectHeaderCursor(t, view, func() { record("folded project summary") })
	// The empty projection is the fourth cursor, and it is the one the other
	// three cannot stand in for: it is the only frame with no list to hold the
	// dock down, so a dock placed after the content rather than at the position
	// the reservation paid for shows up here and nowhere else.
	onEmptyProjectionCursor(t, driver, view, func() { record("empty projection") })

	for _, frame := range frames[1:] {
		if frame.divider != frames[0].divider || frame.position != frames[0].position {
			t.Fatalf("the dock over the %s sits at rows %d..%d while the dock over the %s sits at %d..%d",
				frame.what, frame.divider, frame.position,
				frames[0].what, frames[0].divider, frames[0].position)
		}
	}
}

// onEmptyProjectionCursor puts the view in the one frame with no list under the
// dock — a single project scoped to a mode it files nothing in — seats the cursor
// on the focused project header that frame leads with, runs inspect, and returns
// to the base scope the push captured. Next is that mode here: the fixture files
// no planning membership and Next promotes no pinned lane, so the projection is
// the header and the explanation under it. The scope is pushed through the same
// call `P` commits, because reaching an empty projection is this check's setup
// and the dock's geometry is the behavior it measures.
func onEmptyProjectionCursor(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, inspect func()) {
	t.Helper()
	view.PushProjectScope(uxAcceptanceProjectIDNamed(t, view, uxAcceptancePlatformName))
	uxSelectMode(t, driver, view, chrome.ModeNext)
	if view.HasContentRows() {
		t.Fatalf("the scoped %s projection still draws content rows, so it is not the empty branch", chrome.ModeNext)
	}
	seatCursorOnRow(t, view, "focused project header", func(row outliner.OutlinerRow) bool {
		return row.IsFocusedProjectHeader
	})
	// The explanation is the other half of the requirement: a dock that reached
	// its position by pushing the empty state off the screen would answer the
	// geometry question and lose the sentence. The sentence may wrap at the
	// narrow sizes, so both sides are compared as whitespace-collapsed words.
	words := func(text string) string { return strings.Join(strings.Fields(text), " ") }
	if obs := driver.Observe(ObserveOptions{}); !strings.Contains(words(obs.PlainScreen), words(view.OutlinerEmptyMessage())) {
		t.Fatalf("the empty projection dropped its explanation %q\n%s", view.OutlinerEmptyMessage(), obs.PlainScreen)
	}
	inspect()
	// Popping restores the base frame the push captured, which is the mode as
	// well as the scope, so the walk continues from where it stepped aside.
	if !view.PopProjectScope() {
		t.Fatal("the scoped projection would not return to the base scope")
	}
	uxSelectMode(t, driver, view, chrome.ModeAll)
}

// uxAcceptanceProjectIDNamed resolves a fixture project by the name the fixture
// spells, so the caller names a project rather than an identity the fixture mints.
func uxAcceptanceProjectIDNamed(t *testing.T, view *outliner.OutlinerModel, name string) string {
	t.Helper()
	for _, project := range view.Population.Projects {
		if project != nil && project.Name == name {
			return project.ID
		}
	}
	t.Fatalf("the fixture carries no project named %q", name)
	return ""
}

// assertUXPeekCarriesTheWholeTitle is the counterpart every clipped row is owed.
// TruncateToVisualWidth has no inverse, so the surface that needs the whole
// text renders the whole text: Peek wraps the title rather than eliding it, at
// every width including the one where the row cannot show half of it.
func assertUXPeekCarriesTheWholeTitle(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, size [2]int) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	row := view.Rows[view.Cursor]
	body := view.PeekDocumentBodyForRow(row)
	if len(body) == 0 {
		t.Fatal("the overdue row owns no Peek document")
	}
	stripped := make([]string, 0, len(body))
	for _, line := range body {
		stripped = append(stripped, strings.TrimSpace(ansi.Strip(line)))
	}
	joined := strings.Join(strings.Fields(strings.Join(stripped, " ")), " ")
	if !strings.Contains(joined, uxAcceptanceOverdueTitle) {
		t.Fatalf("Peek does not carry the whole title\n%s", strings.Join(stripped, "\n"))
	}
	obs := driver.Observe(ObserveOptions{})
	if width := size[0]; width <= 100 && strings.Contains(obs.PlainScreen, uxAcceptanceOverdueTitle) {
		// A wide terminal is allowed to hold this title whole — that is the
		// title fitter doing its job. A narrow one is not, and if it ever does,
		// the recovery above has stopped measuring anything and the fixture owes
		// the review a longer title.
		t.Fatalf("the %d-column row holds the whole title, so it no longer measures Peek's recovery\n%s",
			width, obs.PlainScreen)
	}
}
