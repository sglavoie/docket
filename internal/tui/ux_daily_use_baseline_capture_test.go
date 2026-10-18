package tui

import (
	"fmt"
	"strings"
	"testing"
)

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

			if !strings.Contains(screen, "3 tasks · Line 3") {
				t.Fatalf("multiline capture lost its current-line summary:\n%s", screen)
			}
			// The draft lands in one project at the default destination, which the
			// route row already says, so neither the destination counts nor a Route
			// group spends a row repeating it.
			for _, repeated := range []string{"To: ", "(default)"} {
				if strings.Contains(screen, repeated) {
					t.Fatalf("multiline capture repeats its route row with %q:\n%s", repeated, screen)
				}
			}
			full := uxDailyUseCaptureDividerRow(t, driver)
			if grew := empty - full; grew != len(uxDailyUseDraftLines)+1 {
				t.Fatalf("a %d-line draft raised the divider from row %d to row %d, want draft rows plus the count and current-line preview",
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
