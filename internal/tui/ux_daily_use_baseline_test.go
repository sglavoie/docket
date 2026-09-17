package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// TestUXDailyUseProjectLabelFollowsTheTitleInOneColumn is this contract after
// session 2 rewrote it. Project identity is a bracketed slug drawn *after* the
// title, in a column measured over the whole projection, so four projects with
// four different name lengths give their four tasks one title column and one
// label column instead of four of each.
//
// Both halves are asserted, because either alone passes on an arrangement this
// is meant to refuse. Equal title starts alone would still hold if the label had
// simply been deleted, and equal label starts alone would hold if the label had
// stayed in front of a title padded out to a common width — which is the shape
// the predecessor contract was written to catch.
//
// Columns are measured with lipgloss rather than by byte offset: the tree
// connectors and the state badge are multi-byte, so a byte index into the line
// is not the column the terminal draws the fragment in.
func TestUXDailyUseProjectLabelFollowsTheTitleInOneColumn(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, _ := uxDailyUseStagedDriver(t, "project-label-widths", size[0], size[1])
			screen := driver.Observe(ObserveOptions{}).PlainScreen

			starts := map[string]int{}
			labels := map[string]int{}
			for _, title := range uxDailyUseMultiProjectTitles {
				line := uxDailyUseScreenLine(t, screen, title)
				label := strings.Index(line, "[")
				start := strings.Index(line, title)
				if label < 0 {
					t.Fatalf("the row for %q names no project:\n%s", title, line)
				}
				if label < start {
					t.Fatalf("the project label precedes the title on %q, which session 2 retired:\n%s",
						title, line)
				}
				starts[title] = lipgloss.Width(line[:start])
				labels[title] = lipgloss.Width(line[:label])
			}

			if distinct := uxDailyUseDistinct(starts); len(distinct) != 1 {
				t.Fatalf("the titles start in %d different columns, so the label is still sized per row: %v",
					len(distinct), starts)
			}
			if distinct := uxDailyUseDistinct(labels); len(distinct) != 1 {
				t.Fatalf("the labels start in %d different columns, so they are segments rather than a column: %v",
					len(distinct), labels)
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

// TestUXDailyUseCursorRowLeavesItsContentUnstyled records what marks the cursor
// row today: the first column and the trailing fill, and nothing across the
// content between them. One task's row is captured twice — once as the cursor
// row and once with the cursor a row below it — and the styled bytes covering
// its gutter, checkbox, project label, and title are identical in both.
//
// A difference is what the contract is written against, rather than the absence
// of some particular escape. The frame carries a base background on every cell,
// so "the cursor row sets no background" is false of every row on screen and
// would be a contract about the compositor; comparing one row against itself is
// a contract about the cursor.
//
// Session 3 owns this. It gives the current task a subtle background through the
// shared theme and cursor-rendering seams, and rewrites this contract to state
// that the tint owns the row's background, that the foregrounds for state,
// priority, deadline, review, dependencies, and search matches survive on top of
// it, that a distinct marker remains in monochrome and during a cursor flash,
// and that focus stays tellable from membership in a visual selection.
func TestUXDailyUseCursorRowLeavesItsContentUnstyled(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureMultiProject, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	seatCursorOnTaskTitled(t, view, uxDailyUseAlphaTitle)
	focused := uxDailyUseScreenLine(t, driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, uxDailyUseAlphaTitle)

	uxClarityPress(t, driver, uxClarityKeys("j")...)
	unfocused := uxDailyUseScreenLine(t, driver.Observe(ObserveOptions{IncludeANSI: true}).ANSIScreen, uxDailyUseAlphaTitle)

	if ansi.Strip(focused) == focused {
		t.Fatalf("the row carries no styling at all, so the capture is not the styled frame:\n%q", focused)
	}
	if focused == unfocused {
		t.Fatalf("the cursor changes nothing about the row it is on, so there is no marker to record:\n%q", focused)
	}

	focusedContent := uxDailyUseRowContent(t, focused, uxDailyUseAlphaTitle)
	unfocusedContent := uxDailyUseRowContent(t, unfocused, uxDailyUseAlphaTitle)
	if focusedContent != unfocusedContent {
		t.Fatalf("the cursor restyles the row's content, which is the tint session 3 introduces:\nfocused   %q\nunfocused %q",
			focusedContent, unfocusedContent)
	}
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

// TestUXDailyUseSelectionCountsAreStatedTwice records that a visual selection is
// counted in two places at once today: the status strip's own summary line and
// the footer's selection line. The frame it reads is the one where the counts
// are load-bearing rather than decorative — every selected task folded out of
// view, so nothing on screen carries a checkbox.
//
// Both statements are asserted, and so is the fact that they are two rows rather
// than one wrapped one, because the change session 9 makes is a removal: a
// contract that only checked the strip would keep passing after the footer line
// went, and one that only checked the footer would keep passing after the strip
// line did.
//
// Session 4 owns this. It makes the top context row the sole persistent
// summary, removes the selection write from the transient status channel, and
// keeps the full total/visible/hidden breakdown in `Q`. It rewrites this
// contract to say the summary appears exactly once, which row carries it, and
// that a nonzero hidden count survives into the compact form.
func TestUXDailyUseSelectionCountsAreStatedTwice(t *testing.T) {
	for _, size := range uxDailyUseMatrixSizes {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, view := uxDailyUseStagedDriver(t, "hidden-selection", size[0], size[1])
			counts := view.SelectionCounts()
			if counts.Total == 0 || counts.Hidden != counts.Total {
				t.Fatalf("the staged selection is not the hidden one: %+v", counts)
			}

			screen := driver.Observe(ObserveOptions{}).PlainScreen
			strip := uxDailyUseScreenLine(t, screen, "MULTI-SELECT:")
			if !strings.Contains(strip, fmt.Sprintf("%d selected", counts.Total)) ||
				!strings.Contains(strip, fmt.Sprintf("(%d hidden)", counts.Hidden)) {
				t.Fatalf("the strip summary does not carry %+v:\n%s", counts, strip)
			}

			footer := uxClarityFooter(driver)
			wanted := fmt.Sprintf("%d total · %d visible · %d hidden", counts.Total, counts.Visible, counts.Hidden)
			if !strings.Contains(footer, wanted) {
				t.Fatalf("the footer does not restate the counts as %q:\n%s", wanted, footer)
			}
			if strings.Contains(footer, "MULTI-SELECT:") {
				t.Fatalf("the strip summary has moved into the footer, so the two statements are one:\n%s", footer)
			}
		})
	}
}

// TestUXDailyUseWorkflowChordListsOneFlatRun records how the workflow chord
// presents its destinations today: one undivided run in which Today and Next,
// which are planning collections, sit between lifecycle states and read exactly
// like them.
//
// The two widest sizes are read because they are where every destination fits;
// at the floor the run is shortened from the right, so a missing entry there is
// the footer's fitter rather than the grammar. The claim is the absence of a
// division, so the entries are checked for order-neutral presence and the run
// for the absence of a heading.
//
// Session 5 owns this. It distinguishes Plan from Status in workflow guidance,
// capture, and Peek, and rewrites this contract to state where the division
// falls and which destinations sit on each side of it.
func TestUXDailyUseWorkflowChordListsOneFlatRun(t *testing.T) {
	for _, size := range [][2]int{{100, 35}, {120, 40}} {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, _ := uxDailyUseStagedDriver(t, "workflow-chord", size[0], size[1])
			footer := uxClarityFooter(driver)

			for _, entry := range uxDailyUseWorkflowEntries {
				if !strings.Contains(footer, entry) {
					t.Fatalf("the workflow chord does not offer %q:\n%s", entry, footer)
				}
			}
			for _, heading := range []string{"Plan", "Status", "plan", "status"} {
				if strings.Contains(footer, heading+":") {
					t.Fatalf("the chord already groups its destinations under %q, which session 5 introduces:\n%s",
						heading, footer)
				}
			}
		})
	}
}

// uxDailyUseWorkflowEntries is every destination the workflow chord offers. Two
// of them — `t today` and `n next` — are planning collections rather than
// lifecycle states, and their presence in this one list is the whole of what the
// contract above records.
var uxDailyUseWorkflowEntries = []string{
	"i inbox", "b backlog", "n next", "t today", "g doing",
	"w waiting", "p paused", "m someday", "d done", "c canceled",
}

// TestUXDailyUseCapturePanelHeightIgnoresItsDraft records that the capture
// panel's height is a pure function of the frame today: a panel holding a
// three-line draft occupies exactly the rows an empty one did, so a one-line
// capture spends the same screen a multi-line one does.
//
// The measurement is the screen row the panel's own divider lands on, taken
// before and after the draft is typed on one driver. Reading it off the frame
// rather than off the panel's arithmetic is deliberate: the rule this records is
// about what the user sees the panel cost, and a later session is free to reach
// the new behavior through different arithmetic.
//
// Session 7 owns this. It lets compact capture grow with its content, and
// rewrites this contract to state that a blank capture occupies only its
// necessary rows, that the divider rises as lines are added, and where it stops
// — the existing fourteen-row ceiling and four-row host reservation both stand.
// Session 6 is its precondition and reaches no part of this contract: the
// reason the rule currently reads the frame and nothing else is that
// `OutlinerModel.SetSize` drops the scroll window, and session 6 is what makes
// growing the panel safe.
func TestUXDailyUseCapturePanelHeightIgnoresItsDraft(t *testing.T) {
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
			if full != empty {
				t.Fatalf("the capture panel moved its divider from row %d to row %d, which is the growth session 6 introduces",
					empty, full)
			}
		})
	}
}

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
// row of it that is present at every width and with any draft.
const uxDailyUseCaptureDivider = "── Capture →"

// TestUXDailyUseHostedPanelResizeLosesTheScrollWindow records what opening and
// closing a bounded panel costs the outline underneath it today. The host is
// resized to the rows the panel leaves and resized back when it closes, and
// `OutlinerModel.SetSize` drops the scroll offset both times, so the window the
// user was reading is not the window they get back — even though the cursor, the
// rows, the folds, and the data are all unchanged.
//
// It reads the release fixture, the only reviewed catalog with enough rows for a
// window to be anywhere but the top, and it asserts the round trip rather than
// the opening alone. Opening alone proves nothing: the render recomputes a
// window that keeps the cursor visible, and at a shorter height a different
// offset is the correct answer. Coming back to the original height and landing
// on a different offset is the loss.
//
// Session 6 owns this. It separates a temporary host-budget change from a
// resize, preserves the selected task's qualified identity and the viewport
// anchor across successive height changes, and clamps only as far as keeping the
// cursor visible requires. It rewrites this contract to state that restoring the
// size restores the prior view wherever the unchanged data permits.
func TestUXDailyUseHostedPanelResizeLosesTheScrollWindow(t *testing.T) {
	driver, view := uxDailyUseStagedDriver(t, "dense-scrolled", uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	// Off the last row, so the window is not pinned to the end of the document:
	// a cursor the clamp has only one legal answer for cannot record a loss.
	for range uxDailyUseScrollLift {
		uxClarityPress(t, driver, uxClarityKeys("k")...)
	}
	driver.Observe(ObserveOptions{})

	cursor, before := view.Cursor, view.ViewScrollOffset
	if before == 0 {
		t.Fatalf("the window is still at the top after %d rows, so there is no position to lose", len(view.Rows))
	}

	uxClarityPress(t, driver, uxClarityKeys("a")...)
	driver.Observe(ObserveOptions{})
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	driver.Observe(ObserveOptions{})

	if view.Cursor != cursor {
		t.Fatalf("the panel moved the cursor from row %d to row %d, so the offset is not what changed",
			cursor, view.Cursor)
	}
	if after := view.ViewScrollOffset; after == before {
		t.Fatalf("the window came back to offset %d, which is the preservation session 6 introduces", after)
	}
}

// uxDailyUseScrollLift is how far off the last row the cursor is taken before
// the round trip. Ten rows is enough that the window has room to settle
// differently and few enough that one `G` still reaches the end from anywhere.
const uxDailyUseScrollLift = 10

// TestUXDailyUseTagPickerListsAppliedValuesLast records the order the
// tags-and-context picker opens in today: the fixed semantic vocabulary first,
// none of it applied, and the values the task actually carries below it — a
// context and two tags the user has to scroll past six irrelevant rows to reach.
//
// Session 8 owns this. It puts initially applied tags and context first and
// keeps the ordering stable while the picker is open, and rewrites this contract
// to state that the applied values lead, that an unapplied value checked while
// the picker is open does not jump to the front, and that the stability survives
// a query.
func TestUXDailyUseTagPickerListsAppliedValuesLast(t *testing.T) {
	driver, view := uxDailyUseDriver(t, FixtureUXAcceptance, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("#")...)

	applied, unapplied := uxDailyUsePickerRows(t, driver, "[x] ", "[ ] ")
	if len(applied) == 0 || len(unapplied) == 0 {
		t.Fatalf("the picker shows %d applied and %d unapplied rows; the state under test was not reached",
			len(applied), len(unapplied))
	}
	if applied[0] < unapplied[len(unapplied)-1] {
		t.Fatalf("an applied value already leads the picker, which is the order session 8 introduces: applied %v, unapplied %v",
			applied, unapplied)
	}
}

// TestUXDailyUseBulkTagPickerListsMixedValuesLast records the same order for the
// bulk case, where the values a selection partly carries are drawn as a third
// state rather than as checked. Today they sit below the same six unapplied
// semantic rows a single task's picker leads with.
//
// It is a separate contract from the single-task one because the bulk case is a
// separate requirement: universally applied and partially applied values both
// have to lead, together, without collapsing the difference between them. A
// single contract over both would let the session satisfy it by ordering one
// case and leaving the other, which is exactly the shape of the bug, since the
// two cases reach the picker through different membership computations.
//
// Session 8 owns this, as it owns the single-task order above. It also owns not
// changing it anywhere else: the filter vocabularies, the project-tag picker,
// and the dependency picker share this infrastructure and are out of scope.
func TestUXDailyUseBulkTagPickerListsMixedValuesLast(t *testing.T) {
	driver, _ := uxDailyUseStagedDriver(t, "bulk-tag-picker", uxDailyUseDailySize[0], uxDailyUseDailySize[1])

	mixed, unapplied := uxDailyUsePickerRows(t, driver, "[-] ", "[ ] ")
	if len(mixed) == 0 || len(unapplied) == 0 {
		t.Fatalf("the picker shows %d mixed and %d unapplied rows; the state under test was not reached",
			len(mixed), len(unapplied))
	}
	if mixed[0] < unapplied[len(unapplied)-1] {
		t.Fatalf("a partially applied value already leads the picker, which is the order session 7 introduces: mixed %v, unapplied %v",
			mixed, unapplied)
	}
}

// uxDailyUsePickerRows is the screen rows carrying each of two checkbox states,
// in the order they are drawn. Both lists come from one observation so the two
// are always comparable.
//
// Only what is inside the picker's own box is read. The outline underneath is
// still rendered and, under a visual selection, every one of its task rows draws
// a checkbox of its own in the gutter beside the box — so a scan of whole screen
// lines answers about the selection as often as about the picker.
func uxDailyUsePickerRows(t *testing.T, driver *UITestDriver, first, second string) ([]int, []int) {
	t.Helper()
	var firstRows, secondRows []int
	for index, line := range strings.Split(driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
		inside, ok := uxDailyUseInsidePickerBox(line)
		if !ok {
			continue
		}
		switch {
		case strings.Contains(inside, first):
			firstRows = append(firstRows, index)
		case strings.Contains(inside, second):
			secondRows = append(secondRows, index)
		}
	}
	return firstRows, secondRows
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

// TestUXDailyUseStateChangeRaisesTheCheckInEditor records that moving a task to
// Doing opens a check-in editor by itself today. Nothing asked for it: the only
// keys pressed are the workflow chord and its destination, and the editor that
// appears is a modal that has to be dismissed before the list takes another key.
//
// The contract names the editor's own text rather than the presentation stack,
// because what session 10 replaces is the interruption a user sees, and an
// implementation that kept a value step open without drawing it would be a
// different bug.
//
// Session 10 owns this. It replaces the automatic editor with feedback inviting
// explicit scheduling, and rewrites this contract to state that the state change
// commits without a modal, what the feedback says, and that the explicit route
// still reaches the same editor. Session 9 shares the frame: the asynchronous
// editor-failure safeguards this offer carries are what it re-homes.
func TestUXDailyUseStateChangeRaisesTheCheckInEditor(t *testing.T) {
	driver, _ := uxDailyUseStagedDriver(t, "check-in-offer", uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	screen := driver.Observe(ObserveOptions{}).PlainScreen

	for _, fragment := range []string{"Set Check-In", "Optional check-in", "Esc skips"} {
		if !strings.Contains(screen, fragment) {
			t.Fatalf("a state change no longer raises the check-in editor; %q is not on screen:\n%s", fragment, screen)
		}
	}
}

// TestUXDailyUseLaunchSeatsTheCursorOnASectionHeader records where a fresh
// launch lands today: row zero, which is a section header in every reviewed
// catalog, so the first key a user presses acts on a grouping rather than on
// work.
//
// All three reviewed multi-row fixtures are read, because the claim is about the
// launch rule rather than about one catalog's first section, and a rule that
// happened to land on a task in one of them would be a different rule.
//
// Session 11 owns this. It selects the first visible task on a fresh launch, and
// rewrites this contract to state that the cursor lands on a task row, that it
// still lands there when the first section is folded or empty, and that
// asynchronous project loading and a restored lens do not move it afterwards.
func TestUXDailyUseLaunchSeatsTheCursorOnASectionHeader(t *testing.T) {
	for _, fixture := range []string{FixtureMultiProject, FixtureUXAcceptance, FixtureNarrow} {
		t.Run(fixture, func(t *testing.T) {
			_, view := uxDailyUseDriver(t, fixture, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
			if view.Cursor != 0 {
				t.Fatalf("a fresh launch seated the cursor on row %d, not row 0", view.Cursor)
			}
			if len(view.Rows) == 0 {
				t.Fatal("the fixture drew no rows, so there is no landing to record")
			}
			if view.Rows[0].IsTaskRow() {
				t.Fatal("row 0 is already a task row, which is the landing session 11 introduces")
			}
		})
	}
}

// TestUXDailyUseLaunchGuidanceNamesNavigationOnly records what the footer offers
// on that landing: the structural keys a section header answers — zoom,
// collapse, the section fold, the mode cycle, project scope, search, and help —
// and none of the daily acts. Capture, the workflow chord, and edit are all
// absent, because none of them applies to the row the launch seated the cursor
// on.
//
// The two halves are asserted together on purpose. The absent keys are absent
// because of where the cursor is, so a session that moves the landing changes
// this footer without touching guidance at all, and a contract that only listed
// what is present would not show that.
//
// Session 12 owns this, and session 11 reaches it first. Session 12 prioritizes
// useful daily actions in contextual guidance and rewrites this contract to
// state which acts lead the resting footer and what the fitter drops first.
func TestUXDailyUseLaunchGuidanceNamesNavigationOnly(t *testing.T) {
	driver, _ := uxDailyUseDriver(t, FixtureMultiProject, uxDailyUseDailySize[0], uxDailyUseDailySize[1])
	footer := uxClarityFooter(driver)

	for _, rank := range []string{"Enter zoom", "s sections", "Tab next mode", "/ search", "? help"} {
		if !strings.Contains(footer, rank) {
			t.Fatalf("the resting footer does not name %q:\n%s", rank, footer)
		}
	}
	for _, act := range uxClarityOutlinerHostRanks {
		if strings.Contains(footer, act) {
			t.Fatalf("the resting footer already names the daily act %q, so the launch is not on a section header:\n%s",
				act, footer)
		}
	}
}
