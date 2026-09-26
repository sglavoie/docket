package tui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// These are the states the daily-use refinement is about, written down while
// they still behave the old way. Each one is an expectation a named later
// session deliberately replaces, so each test says which session owns it and
// what that session is expected to change it to. They are contracts about the
// arrangement of the screen — which row carries what, where a segment starts,
// how many rows a thing costs, what a width does to it — and never whole-screen
// snapshots, because a snapshot of a state eleven sessions are about to move is
// a snapshot that fails for every reason at once and names none of them.
//
// A test here failing after a daily-use session is not automatically a
// regression. It is that session's own signal that it reached the behavior it
// was aimed at, and the session that reaches it owns rewriting the contract in
// the same change. docs/tui-testing/ux-daily-use-regression-map.md is the index
// of who owns what.
//
// A rewritten one stays here rather than moving out, and stops being a baseline
// when it does: it then states the behavior its session reached and is an
// ordinary contract over it. The map records which session did the rewriting, so
// the two kinds stay tellable apart without reading every doc comment.
//
// Nothing here asserts the initial cursor position by accident. Two tests below
// are *about* it, and every other one seats the cursor itself through its
// frame's staging function, because session 11 moves where a launch lands and a
// contract that silently depended on today's landing would fail for a reason it
// never meant to state.

// uxDailyUseMatrixSizes is the review's floor, its two working sizes, and its
// widest: the same four the exported frames are taken at.
var uxDailyUseMatrixSizes = uxPresentationMatrixSizes

// uxDailyUseDailySize is the size this plan optimizes for.
var uxDailyUseDailySize = [2]int{100, 35}

// uxDailyUseSizeName spells one size for a subtest name.
func uxDailyUseSizeName(size [2]int) string {
	return fmt.Sprintf("%dx%d", size[0], size[1])
}

// TestUXDailyUseProjectHeaderNamesEachGroup is this contract after the layout
// plan restored the expanded project heading. Project identity is the
// `[slug] (count)` header that opens each group, so a task row spends no column
// on its project and four projects with four different name lengths still give
// their four root tasks one title column.
//
// Both halves are asserted, because either alone passes on an arrangement this
// is meant to refuse: equal title starts alone would still hold if the project
// were simply never named, and a header alone would hold beside a row that
// repeats the slug after its title.
//
// Columns are measured with lipgloss rather than by byte offset: the tree
// connectors and the state badge are multi-byte, so a byte index into the line
// is not the column the terminal draws the fragment in.
func TestUXDailyUseProjectHeaderNamesEachGroup(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, _ := uxDailyUseStagedDriver(t, "project-label-widths", size[0], size[1])
			screen := driver.Observe(ObserveOptions{}).PlainScreen
			lines := strings.Split(screen, "\n")

			starts := map[string]int{}
			for _, title := range uxDailyUseMultiProjectTitles {
				index := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, title) })
				if index < 0 {
					t.Fatalf("no screen row carries %q:\n%s", title, screen)
				}
				line := lines[index]
				if strings.Contains(line, "[") {
					t.Fatalf("the row for %q names its project beside the title:\n%s", title, line)
				}
				if index == 0 || !strings.Contains(lines[index-1], "[") {
					t.Fatalf("no project header opens the group holding %q:\n%s", title, screen)
				}
				starts[title] = lipgloss.Width(line[:strings.Index(line, title)])
			}

			if distinct := uxDailyUseDistinct(starts); len(distinct) != 1 {
				t.Fatalf("the titles start in %d different columns: %v", len(distinct), starts)
			}
		})
	}
}

// uxDailyUseMultiProjectTitles is the `multi-project` fixture's four tasks, one
// per project. The fixture is the only reviewed catalog whose project names have
// four different lengths, which is what makes the row's arithmetic observable.
var uxDailyUseMultiProjectTitles = []string{
	uxDailyUseAlphaTitle,
	"Beta rollout checklist",
	"Renew passport",
	"Finish the distributed systems book",
}

// uxDailyUseDistinct is the set of column positions a map of them holds.
func uxDailyUseDistinct(starts map[string]int) map[int]bool {
	distinct := map[int]bool{}
	for _, start := range starts {
		distinct[start] = true
	}
	return distinct
}

// uxDailyUseScreenLine is the one screen line carrying a fragment. It fails
// rather than returning nothing, because every caller below is asking about the
// arrangement of a row it has already staged and a missing row means the staging
// did not reach the state, not that the arrangement is wrong.
func uxDailyUseScreenLine(t *testing.T, screen, fragment string) string {
	t.Helper()
	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, fragment) {
			return line
		}
	}
	t.Fatalf("no screen row carries %q:\n%s", fragment, screen)
	return ""
}

// TestUXDailyUseCursorRowWearsOneBandUnderItsContent is this contract after
// session 3 rewrote it. The cursor row now carries a background across its whole
// width, and the claim is about what that costs: the band goes *under* the row,
// so every foreground the row earned is still on the line, and the one mark that
// is not a color survives a terminal that paints none.
//
// One task's row is captured twice — once as the cursor row and once with the
// cursor a row below it — and the difference between the two captures is what
// every assertion reads. The frame carries a base background on every cell, so
// "the cursor row sets a background" is true of every row on screen and would be
// a contract about the compositor; comparing one row against itself is a
// contract about the cursor.
//
// The foregrounds are checked as a set taken off the unfocused capture rather
// than named one at a time, because the row's own signals are what it happens to
// draw at this size and this fixture, and a named list would quietly stop
// covering one the moment the row dropped it. The per-signal claim — state,
// priority, deadline, review, dependencies, and a search match, each legible on
// the band in both themes — is stated at the row level in
// outliner_cursor_highlight_test.go, where all six can be put on one task; so
// are the flash and the visual selection, which are gestures rather than frames.
func TestUXDailyUseCursorRowWearsOneBandUnderItsContent(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureMultiProject, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	seatCursorOnTaskTitled(t, view, uxDailyUseAlphaTitle)
	focused := uxDailyUseScreenLine(t, driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, uxDailyUseAlphaTitle)

	uxClarityPress(t, driver, uxClarityKeys("j")...)
	unfocused := uxDailyUseScreenLine(t, driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, uxDailyUseAlphaTitle)

	if ansi.Strip(focused) == focused {
		t.Fatalf("the row carries no styling at all, so the capture is not the styled frame:\n%q", focused)
	}

	focusedContent := uxDailyUseRowContent(t, focused, uxDailyUseAlphaTitle)
	unfocusedContent := uxDailyUseRowContent(t, unfocused, uxDailyUseAlphaTitle)
	band := backgroundParams(t, theme.ColorCursorRowBg)
	if !strings.Contains(focusedContent, band) {
		t.Fatalf("the cursor row's content wears no band:\n%q", focusedContent)
	}
	if strings.Contains(unfocusedContent, band) {
		t.Fatalf("a row the cursor left still wears the band:\n%q", unfocusedContent)
	}

	for _, foreground := range uxDailyUseForegrounds(t, unfocusedContent) {
		if !strings.Contains(focusedContent, foreground) {
			t.Fatalf("the band swallowed the %s the row draws when unfocused:\nfocused   %q\nunfocused %q",
				foreground, focusedContent, unfocusedContent)
		}
	}

	mono := uxClarityMonochrome(t, focused)
	if !strings.Contains(mono, "\x1b[7m") {
		t.Fatalf("the cursor row keeps no mark on a terminal without color:\n%q", mono)
	}
	if other := uxClarityMonochrome(t, unfocused); strings.Contains(other, "\x1b[7m") {
		t.Fatalf("a row the cursor left is inverted too, so the mark says nothing:\n%q", other)
	}
}

// uxDailyUseForegroundPattern matches one truecolor foreground parameter run
// wherever it sits in an SGR sequence, since a piece that also arms a background
// spells both in one sequence.
var uxDailyUseForegroundPattern = regexp.MustCompile(`38;2;\d+;\d+;\d+`)

// uxDailyUseForegrounds is the set of foreground colors a run of styled text
// paints, in first-seen order so a failure names them the way the row draws them.
func uxDailyUseForegrounds(t *testing.T, styled string) []string {
	t.Helper()
	var found []string
	seen := map[string]bool{}
	for _, match := range uxDailyUseForegroundPattern.FindAllString(styled, -1) {
		if seen[match] {
			continue
		}
		seen[match] = true
		found = append(found, match)
	}
	if len(found) == 0 {
		t.Fatalf("the row paints no foreground at all, so nothing is being held to the band:\n%q", styled)
	}
	return found
}

// uxDailyUseRowContent is the styled bytes of a task row between its gutter and
// the end of its title: everything the cursor is not supposed to touch, with the
// first column and the trailing fill — the two parts it is — left outside.
func uxDailyUseRowContent(t *testing.T, line, title string) string {
	t.Helper()
	start := strings.Index(line, uxDailyUseRowGutter)
	end := strings.Index(line, title)
	if start < 0 || end < start {
		t.Fatalf("the row carries no gutter before %q:\n%q", title, line)
	}
	return line[start : end+len(title)]
}

// uxDailyUseRowGutter is the tree connector every `multi-project` task row draws,
// each of them being an only child of its project.
const uxDailyUseRowGutter = "└── "

// TestUXDailyUseSelectionCountsAreStatedOnce is this contract after session 4
// rewrote it. A visual selection used to be counted in two places at once — the
// context row under the title and the selection line the status channel held
// sticky — and the context row is now the whole of it.
//
// The frame it reads is the one where the counts are load-bearing rather than
// decorative: every selected task folded out of view, so nothing on screen
// carries a checkbox and the summary is the only thing that knows the gesture
// has five targets.
//
// Four claims, and each of them fails on its own. The summary appears exactly
// once on screen, so a second statement anywhere is a finding. The channel that
// used to carry the second one says nothing about a selection, which is what
// keeps it free for the feedback a bulk gesture is about to write there. The
// hidden count survives into both spellings the row can draw, the compact one
// included, because that is the number a bulk gesture must not be surprised by
// and shedding it is exactly what a narrow terminal would otherwise do. And `Q`
// still carries all three numbers, since the visible half is written down
// nowhere else.
func TestUXDailyUseSelectionCountsAreStatedOnce(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, view := uxDailyUseStagedDriver(t, "hidden-selection", size[0], size[1])
			counts := view.SelectionCounts()
			if counts.Total == 0 || counts.Hidden != counts.Total {
				t.Fatalf("the staged selection is not the hidden one: %+v", counts)
			}

			screen := driver.Observe(ObserveOptions{}).PlainScreen
			if stated := strings.Count(screen, "MULTI-SELECT"); stated != 1 {
				t.Fatalf("the selection is stated %d times, want exactly one:\n%s", stated, screen)
			}
			summary := uxDailyUseScreenLine(t, screen, "MULTI-SELECT")
			if !strings.Contains(summary, fmt.Sprintf("(%d hidden)", counts.Hidden)) {
				t.Fatalf("the summary drops the hidden half of %+v:\n%s", counts, summary)
			}

			// The context row is above the list, so the row carrying the summary is
			// the one directly under the title rather than anything the shell pins at
			// the bottom.
			rows := strings.Split(screen, "\n")
			if len(rows) < 2 || !strings.Contains(rows[1], "MULTI-SELECT") {
				t.Fatalf("the summary is not the context row under the title:\n%s", screen)
			}
			if footer := uxClarityFooter(driver); strings.Contains(footer, "Visual selection") {
				t.Fatalf("the status channel still states the selection:\n%s", footer)
			}
			if view.StatusMessage != "" {
				t.Fatalf("entering a selection wrote the status line: %q", view.StatusMessage)
			}

			// Both spellings are asked for directly, because whether this terminal is
			// narrow enough to compact the row is the renderer's decision and the
			// claim is about the compact form itself.
			entry, ok := uxDailyUseSelectionEntry(view, size[0])
			if !ok {
				t.Fatal("the live axes carry no selection entry")
			}
			hidden := fmt.Sprintf("(%d hidden)", counts.Hidden)
			if !strings.Contains(ansi.Strip(entry.Row), hidden) ||
				!strings.Contains(ansi.Strip(entry.RowCompact), hidden) {
				t.Fatalf("a spelling sheds the hidden count: full %q compact %q",
					ansi.Strip(entry.Row), ansi.Strip(entry.RowCompact))
			}

			uxClarityPress(t, driver, uxClarityKeys("Q")...)
			readout := driver.Observe(ObserveOptions{}).PlainScreen
			breakdown := fmt.Sprintf("%d total · %d visible · %d hidden", counts.Total, counts.Visible, counts.Hidden)
			if !strings.Contains(readout, breakdown) {
				t.Fatalf("the readout does not carry %q:\n%s", breakdown, readout)
			}
		})
	}
}

// uxDailyUseSelectionEntry is the selection axis as the context row and the
// readout both read it, taken from the model's own description rather than
// recovered from a rendered row.
func uxDailyUseSelectionEntry(view *outliner.OutlinerModel, width int) (outliner.ViewContextEntry, bool) {
	for _, entry := range view.BuildViewContextEntries(width) {
		if entry.Label == "selection" {
			return entry, true
		}
	}
	return outliner.ViewContextEntry{}, false
}

// TestUXDailyUseWorkflowChordDividesPlanFromStatus is this contract after
// session 5 rewrote it. The chord's ten destinations are two kinds of act, and
// the which-key row says so: `n next` and `t today` commit a task to a planning
// collection and stand under Plan, and the eight that set a lifecycle state
// stand under Status. Plan leads, because two entries at the front of the run
// are two the narrow row never sheds.
//
// The two widest sizes are read because they are where every destination fits;
// at the floor the run is shortened from the right, so a missing entry there is
// the footer's fitter rather than the grammar. The division is asserted as
// position rather than as presence — a heading that fell after the entries it
// opens would pass a containment check and tell a reader the opposite thing.
func TestUXDailyUseWorkflowChordDividesPlanFromStatus(t *testing.T) {
	for _, size := range [][2]int{{100, 35}, {120, 40}} {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, _ := uxDailyUseStagedDriver(t, "workflow-chord", size[0], size[1])
			footer := uxClarityFooter(driver)

			for _, entry := range uxDailyUseWorkflowEntries {
				if !strings.Contains(footer, entry) {
					t.Fatalf("the workflow chord does not offer %q:\n%s", entry, footer)
				}
			}

			planAt := strings.Index(footer, "Plan: n next")
			statusAt := strings.Index(footer, "Status: i inbox")
			if planAt < 0 || statusAt < 0 {
				t.Fatalf("the chord does not head its two divisions (plan=%d status=%d):\n%s",
					planAt, statusAt, footer)
			}
			if planAt > statusAt {
				t.Fatalf("Status leads Plan in the workflow row:\n%s", footer)
			}
			// Every planning entry sits inside Plan's run, so nothing between the
			// two headings is a lifecycle state and nothing after Status is a plan.
			if todayAt := strings.Index(footer, "t today"); todayAt < planAt || todayAt > statusAt {
				t.Fatalf("`t today` sits outside the Plan division at %d:\n%s", todayAt, footer)
			}
			for _, state := range []string{"i inbox", "b backlog", "g doing"} {
				if at := strings.Index(footer, state); at < statusAt {
					t.Fatalf("%q sits above the Status heading at %d:\n%s", state, at, footer)
				}
			}
		})
	}
}

// uxDailyUseWorkflowEntries is every destination the workflow chord offers, in
// the order the row draws them: the two planning collections first, then the
// eight lifecycle states.
var uxDailyUseWorkflowEntries = []string{
	"n next", "t today", "i inbox", "b backlog", "g doing",
	"w waiting", "p paused", "m someday", "d done", "c canceled",
}

// TestUXDailyUseCapturePanelIsSizedByItsContent is this contract after session
// 7 rewrote it. The capture panel is no longer a fixed share of the frame: a
// blank capture occupies only the rows it needs — its divider, the destination
// summary, and the one line being typed — and the divider rises by one row for
// each line added to the draft. It stops at the fourteen-row ceiling, and the
// four rows the host is owed are still there underneath when it does.
//
// The measurement is the screen row the panel's own divider lands on, taken
// repeatedly on one driver. Reading it off the frame rather than off the panel's
// arithmetic is deliberate: the rule is about what the user sees the panel cost,
// and the growth is asserted as a difference between two of those rows, so a
// footer that wraps at a narrow width shifts both measurements equally and the
// claim stays about the draft.
//
// The predecessor contract recorded the opposite — a panel holding a three-line
// draft occupied exactly the rows an empty one did — and named this session as
// its owner. Session 6 was its precondition: growth is only safe because a
// hosted budget is a loan the outline answers by keeping the window it was
// reading, rather than a resize that drops it.
func TestUXDailyUseCapturePanelIsSizedByItsContent(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, _ := uxDailyUseDriver(t, FixtureNarrow, size[0], size[1])
			uxClarityPress(t, driver, uxClarityKeys("a")...)
			empty := uxDailyUseCaptureDividerRow(t, driver)

			uxClarityPress(t, driver, uxDailyUseDraftActions()...)
			screen := driver.Observe(ObserveOptions{}).PlainScreen
			for _, line := range uxDailyUseDraftLines {
				if !strings.Contains(screen, line) {
					t.Fatalf("the draft line %q is not on screen, so the panel was not filled:\n%s", line, screen)
				}
			}

			full := uxDailyUseCaptureDividerRow(t, driver)
			if grew := empty - full; grew != len(uxDailyUseDraftLines) {
				t.Fatalf("a %d-line draft raised the divider from row %d to row %d, want one row per line",
					len(uxDailyUseDraftLines), empty, full)
			}

			// Past the ceiling the panel stops taking rows, and the draft is still
			// whole: the input scrolls inside the height it was given, so the line
			// the cursor is on is the one on screen.
			uxClarityPress(t, driver, uxDailyUseOverflowDraftActions(1, 20)...)
			capped := uxDailyUseCaptureDividerRow(t, driver)
			if capped >= full {
				t.Fatalf("a twenty-line draft did not grow the panel past row %d (divider at %d)", full, capped)
			}
			uxClarityPress(t, driver, uxDailyUseOverflowDraftActions(21, 10)...)
			settled := uxDailyUseCaptureDividerRow(t, driver)
			if settled != capped {
				t.Fatalf("the panel kept growing past its ceiling: divider moved from row %d to row %d", capped, settled)
			}
			if settled < uxDailyUseCaptureHostFloorRows {
				t.Fatalf("the capped panel's divider sits at row %d, above the %d rows the host is owed",
					settled, uxDailyUseCaptureHostFloorRows)
			}
			if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "overflow 30") {
				t.Fatalf("the line being typed is not on screen, so growth lost the cursor:\n%s", screen)
			}
		})
	}
}

// uxDailyUseOverflowDraftActions types count further draft lines, numbered from
// first so the last one typed is identifiable on screen.
func uxDailyUseOverflowDraftActions(first, count int) []UITestAction {
	actions := make([]UITestAction, 0, count*2)
	for i := first; i < first+count; i++ {
		actions = append(actions,
			UITestAction{Type: UITestActionKey, Key: "shift+enter"},
			UITestAction{Type: UITestActionText, Text: fmt.Sprintf("overflow %d", i)},
		)
	}
	return actions
}

// uxDailyUseCaptureHostFloorRows is the four rows the panel leaves the
// presentation underneath, read here as screen rows above the divider. The
// panel's own arithmetic states the same floor; this is what it looks like.
const uxDailyUseCaptureHostFloorRows = 4

// uxDailyUseCaptureDividerRow is which screen row the capture panel's labeled
// divider lands on, which is the top of the panel and so the measure of its
// height in a bottom-pinned layout.
func uxDailyUseCaptureDividerRow(t *testing.T, driver *UITestDriver) int {
	t.Helper()
	screen := driver.Observe(ObserveOptions{}).PlainScreen
	for index, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, uxDailyUseCaptureDivider) {
			return index
		}
	}
	t.Fatalf("the capture panel is not open; no row carries %q:\n%s", uxDailyUseCaptureDivider, screen)
	return -1
}

// uxDailyUseCaptureDivider is the panel's own labeled divider, which is the one
// row of it that is present at every width and with any draft. It names the act
// and not the destination: the route row under it is what resolves the project.
const uxDailyUseCaptureDivider = "── Capture ─"

// TestUXDailyUseHostedPanelResizeKeepsTheScrollWindow is what session 6 reached.
// The rows a bounded panel borrows are a loan rather than a resize, so opening
// capture over the outline covers the bottom of the window the user was reading
// and closing it hands that same window back — the cursor, the rows, the folds
// and the data are all unchanged, and so is the position.
//
// It reads the release fixture, the only reviewed catalog with enough rows for a
// window to be anywhere but the top, and it asserts the round trip rather than
// the opening alone. Opening alone proves nothing: the render recomputes a
// window that keeps the cursor visible, and at a shorter height a different
// offset is the correct answer. Coming back to the original height and landing
// on the original offset is the preservation.
//
// The loan itself is `route.HostedBudgetView`, and `internal/tui/outliner`'s own
// tests hold its ladder — the top task row held by qualified identity, the
// cursor's screen row behind it, the raw offset behind that — while a terminal
// resize still drops the window the way `SetSize` always has.
func TestUXDailyUseHostedPanelResizeKeepsTheScrollWindow(t *testing.T) {
	driver, view := uxDailyUseStagedDriver(t, "dense-scrolled", uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	// Off the last row, so the window is not pinned to the end of the document:
	// a cursor the clamp has only one legal answer for cannot record a loss.
	for range uxDailyUseScrollLift {
		uxClarityPress(t, driver, uxClarityKeys("k")...)
	}
	driver.Observe(ObserveOptions{})

	cursor, before := view.Cursor, view.ViewScrollOffset
	if before == 0 {
		t.Fatalf("the window is still at the top after %d rows, so there is no position to keep", len(view.Rows))
	}

	uxClarityPress(t, driver, uxClarityKeys("a")...)
	driver.Observe(ObserveOptions{})
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	driver.Observe(ObserveOptions{})

	if view.Cursor != cursor {
		t.Fatalf("the panel moved the cursor from row %d to row %d, so the offset is not what this asserts",
			cursor, view.Cursor)
	}
	if after := view.ViewScrollOffset; after != before {
		t.Fatalf("the window came back to offset %d, want the %d it was reading before the panel opened",
			after, before)
	}
}

// uxDailyUseScrollLift is how far off the last row the cursor is taken before
// the round trip. Ten rows is enough that the window has room to settle
// differently and few enough that one `G` still reaches the end from anywhere.
const uxDailyUseScrollLift = 10

// TestUXDailyUseTagPickerLeadsWithAppliedValues is session 8's rewrite of the
// baseline that recorded the opposite: the tags-and-context picker used to open
// on the fixed semantic vocabulary, none of it applied, with the values the task
// actually carries six rows below it.
//
// It now states the three things that order is worth: the applied values lead,
// a value checked while the picker is open does not jump up to join them, and
// neither does a query rearrange what leads. The second is the freeze — the
// order is computed once, out of the membership the picker opened with, so the
// row a reader is looking at stays where they found it.
func TestUXDailyUseTagPickerLeadsWithAppliedValues(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureUXAcceptance, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("#")...)

	opened := uxDailyUsePickerRows(t, driver)
	applied := uxDailyUsePickerRowsMarked(opened, "[x]")
	unapplied := uxDailyUsePickerRowsMarked(opened, "[ ]")
	if len(applied) == 0 || len(unapplied) == 0 {
		t.Fatalf("the picker shows %d applied and %d unapplied rows; the state under test was not reached",
			len(applied), len(unapplied))
	}
	if last, first := applied[len(applied)-1], unapplied[0]; last.index > first.index {
		t.Fatalf("applied %q is drawn below unapplied %q:\n%s",
			last.value, first.value, uxDailyUsePickerOrder(opened))
	}

	// The jump lands on the SEMANTIC section, which is unapplied by definition
	// here, so what is toggled is a row the freeze has to leave where it is.
	uxClarityPress(t, driver, uxClarityKeys("s")...)
	uxClarityPress(t, driver, uxClarityKeys("space")...)
	toggled := uxDailyUsePickerRows(t, driver)
	if uxDailyUsePickerOrder(toggled) != uxDailyUsePickerOrder(opened) {
		t.Fatalf("a toggle reordered the open picker:\nbefore\n%s\nafter\n%s",
			uxDailyUsePickerOrder(opened), uxDailyUsePickerOrder(toggled))
	}
	checked := uxDailyUsePickerRowsMarked(toggled, "[x]")
	if len(checked) != len(applied)+1 {
		t.Fatalf("the toggle checked %d rows, want the %d applied rows plus one:\n%s",
			len(checked), len(applied), uxDailyUsePickerOrder(toggled))
	}

	// What leads is what the picker opened applied, not what is checked now: the
	// row toggled above is checked and still sits in the vocabulary it was drawn
	// in, which is the freeze, so the query is asked about the promoted values.
	promoted := make(map[string]bool, len(applied))
	for _, row := range applied {
		promoted[row.value] = true
	}
	uxClarityPress(t, driver, uxClarityKeys("/")...)
	uxClarityPress(t, driver, uxClarityKeys("i")...)
	queried := uxDailyUsePickerRows(t, driver)
	matchedPromoted, matchedRest := 0, 0
	for _, row := range queried {
		if !promoted[row.value] {
			matchedRest++
			continue
		}
		matchedPromoted++
		// Ranking reorders the rows a query matched, and a promoted row is not
		// ranked back down into the vocabulary underneath it.
		if matchedRest > 0 {
			t.Fatalf("under a query promoted %q is drawn below an unpromoted row:\n%s",
				row.value, uxDailyUsePickerOrder(queried))
		}
	}
	if matchedPromoted == 0 || matchedRest == 0 {
		t.Fatalf("the query left %d promoted and %d other rows; it narrowed too far to say anything:\n%s",
			matchedPromoted, matchedRest, uxDailyUsePickerOrder(queried))
	}

	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	if cleared := uxDailyUsePickerRows(t, driver); uxDailyUsePickerOrder(cleared) != uxDailyUsePickerOrder(toggled) {
		t.Fatalf("clearing the query rebuilt the order:\nbefore\n%s\nafter\n%s",
			uxDailyUsePickerOrder(toggled), uxDailyUsePickerOrder(cleared))
	}
}

// TestUXDailyUseBulkTagPickerLeadsWithMixedValues is session 8's rewrite of the
// bulk half. It stays a separate contract from the single-task one because the
// two reach the picker through different membership computations: universally
// applied and partially applied values both have to lead, and the difference
// between them has to survive the promotion.
//
// The clear-context row is the counterexample the bulk case needs. Every target
// without a context agrees on it, so it reads as mixed exactly like a value the
// selection partly carries — and it is not a value at all, so it stays in
// CONTEXTS rather than leading a list of what the tasks are tagged with.
//
// Session 8 also owns not changing this anywhere else: the filter vocabularies,
// the project-tag picker, and the dependency picker share this infrastructure
// and keep their own order.
func TestUXDailyUseBulkTagPickerLeadsWithMixedValues(t *testing.T) {
	driver, _ := uxDailyUseStagedDriver(t, "bulk-tag-picker", uxDailyUseDailySize[0], uxDailyUseDailySize[1])

	rows := uxDailyUsePickerRows(t, driver)
	unapplied := uxDailyUsePickerRowsMarked(rows, "[ ]")
	mixed := uxDailyUsePickerRowsMarked(rows, "[-]")
	if len(mixed) == 0 || len(unapplied) == 0 {
		t.Fatalf("the picker shows %d mixed and %d unapplied rows; the state under test was not reached",
			len(mixed), len(unapplied))
	}

	clear := mixed[len(mixed)-1]
	if !strings.HasPrefix(clear.value, "No context") {
		t.Fatalf("the last mixed row is %q, want the clear-context row every untagged target agrees on:\n%s",
			clear.value, uxDailyUsePickerOrder(rows))
	}
	if clear.index != rows[len(rows)-1].index {
		t.Fatalf("the clear-context row was promoted out of CONTEXTS:\n%s", uxDailyUsePickerOrder(rows))
	}
	for _, row := range mixed[:len(mixed)-1] {
		if row.index > unapplied[0].index {
			t.Fatalf("partially applied %q is drawn below unapplied %q:\n%s",
				row.value, unapplied[0].value, uxDailyUsePickerOrder(rows))
		}
	}
}

// uxDailyUsePickerRow is one checkbox row of the overlay: where it is drawn,
// which of the three states it carries, and the text beside it.
type uxDailyUsePickerRow struct {
	index int
	mark  string
	value string
}

// uxDailyUsePickerRows is the picker's checkbox rows in the order they are
// drawn, from one observation, so any two of them are always comparable.
//
// Only what is inside the picker's own box is read. The outline underneath is
// still rendered and, under a visual selection, every one of its task rows draws
// a checkbox of its own in the gutter beside the box — so a scan of whole screen
// lines answers about the selection as often as about the picker.
func uxDailyUsePickerRows(t *testing.T, driver *UITestDriver) []uxDailyUsePickerRow {
	t.Helper()
	rows := make([]uxDailyUsePickerRow, 0, 16)
	for index, line := range strings.Split(driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
		inside, ok := uxDailyUseInsidePickerBox(line)
		if !ok {
			continue
		}
		for _, mark := range []string{"[x]", "[-]", "[ ]"} {
			at := strings.Index(inside, mark+" ")
			if at < 0 {
				continue
			}
			rows = append(rows, uxDailyUsePickerRow{
				index: index,
				mark:  mark,
				value: strings.TrimSpace(inside[at+len(mark):]),
			})
			break
		}
	}
	return rows
}

// uxDailyUsePickerRowsMarked narrows those rows to one checkbox state.
func uxDailyUsePickerRowsMarked(rows []uxDailyUsePickerRow, mark string) []uxDailyUsePickerRow {
	marked := make([]uxDailyUsePickerRow, 0, len(rows))
	for _, row := range rows {
		if row.mark == mark {
			marked = append(marked, row)
		}
	}
	return marked
}

// uxDailyUsePickerOrder is the sequence of values alone, which is what a freeze
// is a claim about: the checkbox beside a row is free to change without the row
// moving, so the marks are deliberately not part of it.
func uxDailyUsePickerOrder(rows []uxDailyUsePickerRow) string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.value)
	}
	return strings.Join(values, "\n")
}

// uxDailyUseInsidePickerBox is the part of a screen line the overlay's own
// borders enclose, and whether the line crosses the box at all.
func uxDailyUseInsidePickerBox(line string) (string, bool) {
	const border = "│"
	open := strings.Index(line, border)
	close := strings.LastIndex(line, border)
	if open < 0 || close <= open {
		return "", false
	}
	return line[open+len(border) : close], true
}

// TestUXDailyUseStateChangeNotesTheMissingCheckIn records what moving a task to
// Doing does now: it commits, the footer says the task carries no check-in and
// names the route that would give it one, and nothing is mounted. The only keys
// pressed are the workflow chord and its destination, so the list still has the
// keyboard when the frame settles.
//
// The contract names the visible sentence and the absence of the editor's own
// text rather than the presentation stack, because what a user experiences is
// the interruption or its absence: an implementation that kept a value step open
// without drawing it would be a different bug, and one that drew the note from a
// cursor read rather than from committed evidence would name the wrong task.
//
// The explicit route is asserted beside it, since replacing an offer must not
// cost the editor itself: the same command still raises it over the same row.
func TestUXDailyUseStateChangeNotesTheMissingCheckIn(t *testing.T) {
	driver, _ := uxDailyUseStagedDriver(t, "check-in-offer", uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	screen := driver.Observe(ObserveOptions{}).PlainScreen

	for _, fragment := range []string{"No check-in", "Set up check-ins"} {
		if !strings.Contains(screen, fragment) {
			t.Fatalf("a state change no longer notes the missing check-in; %q is not on screen:\n%s", fragment, screen)
		}
	}
	for _, fragment := range []string{"Next check-in:", "Esc skips", "Optional check-in"} {
		if strings.Contains(screen, fragment) {
			t.Fatalf("a state change still raises the check-in editor; %q is on screen:\n%s", fragment, screen)
		}
	}
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionCommand, Command: "Set Check-In"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if explicit := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(explicit, "Next check-in:") {
		t.Fatalf("the explicit route no longer reaches the editor:\n%s", explicit)
	}
}

// TestUXDailyUseLaunchSeatsTheCursorOnTheFirstTask states where a fresh launch
// lands: on the first task row the settled projection draws, whatever chrome
// stands above it, so the first key a user presses acts on work rather than on a
// grouping.
//
// All three reviewed multi-row fixtures are read, because the claim is about the
// launch rule rather than about one catalog's first section. Each is asserted
// twice: the cursor is on a task, and it is on the *first* one, which is what
// separates the rule from a landing that merely happened to clear the header.
//
// A launch folds All's Backlog and Someday lanes and nothing else, so a fixture
// whose work all sits in those two lanes draws no task row and lands on the
// first row, the first folded header, which is the rule's own fallback. A section holding nothing is never
// emitted. What can stand over the first task is a section header, a project
// group summary, or a focused-project header, and the rule reads past all three
// by asking each row what it is. The
// asynchronous halves of the rule — a superseded read, a lens still loading its
// own projects, a keystroke arriving first — are in app_launch_focus_test.go,
// where they can be driven one message at a time.
func TestUXDailyUseLaunchSeatsTheCursorOnTheFirstTask(t *testing.T) {
	for _, fixture := range []string{FixtureMultiProject, FixtureUXAcceptance, FixtureNarrow} {
		t.Run(fixture, func(t *testing.T) {
			_, view := uxDailyUseDriver(t, fixture, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
			if len(view.Rows) == 0 {
				t.Fatal("the fixture drew no rows, so there is no landing to record")
			}
			wanted := -1
			for index, row := range view.Rows {
				if row.IsTaskRow() && row.Task != nil {
					wanted = index
					break
				}
			}
			if wanted < 0 {
				if !view.Folds.CollapsedSections["_state:"+string(models.StateGroupBacklog)] {
					t.Fatal("the fixture drew no task row and no folded Backlog lane hides one")
				}
				wanted = 0
			}
			if view.Cursor != wanted {
				t.Fatalf("a fresh launch seated the cursor on row %d, not the first task row %d", view.Cursor, wanted)
			}
		})
	}
}

// TestUXDailyUseLaunchGuidanceNamesTheDailyActs records what the footer offers
// on that landing now that the launch seats the cursor on work. It is asserted
// as one contiguous run rather than a set of memberships, because the claim is
// about order as much as presence: the six acts a task row is read for lead the
// resting footer, in ladder order, and the axes it is changed on follow them.
//
// The shed half is the other side of the same claim. `Enter zoom` and `s
// sections` are gone because the cursor is no longer on a header, and `Space
// actions` — rank 10, the first entry of the third tier — is what the fitter
// refuses at this width, so the row states where the daily size ends rather than
// only what it reached. A contract that only listed what is present would show
// neither.
//
// Session 12 owns this, and session 11 reached it first by moving the landing.
func TestUXDailyUseLaunchGuidanceNamesTheDailyActs(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureMultiProject, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	// This fixture's work all sits in the Backlog lane a launch folds, so the
	// landing is that header; opening it and stepping down reaches the first task.
	sortPressKeys(t, driver, "l")
	for range len(view.Rows) {
		if row := view.Rows[view.Cursor]; row.IsTaskRow() && row.Task != nil {
			break
		}
		sortPressKeys(t, driver, "j")
	}
	if row := view.Rows[view.Cursor]; !row.IsTaskRow() || row.Task == nil {
		t.Fatalf("opening the Backlog lane left the cursor off a task row: %+v", row)
	}
	footer := uxClarityFooter(driver)

	leading := "a capture · w workflow · e edit · / search · b peek · ? help"
	if !strings.Contains(footer, leading) {
		t.Fatalf("the resting footer does not lead with the acts a task row is read for (%q):\n%s",
			leading, footer)
	}
	axes := leading + " · @ date · ! priority · # tags/context"
	if !strings.Contains(footer, axes) {
		t.Fatalf("the resting footer does not follow those acts with the axes a row is changed on (%q):\n%s",
			axes, footer)
	}
	for _, shed := range []string{"Enter zoom", "s sections", "Space actions"} {
		if strings.Contains(footer, shed) {
			t.Fatalf("the resting footer still names %q, so this is not the row the daily size draws:\n%s",
				shed, footer)
		}
	}
}
