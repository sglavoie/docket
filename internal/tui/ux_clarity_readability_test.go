package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestUXClarityCombinedContextIsReadableAtEverySize states session 5's outcome
// for the imbalance the baseline recorded: every context axis used to be joined
// into the one title row, fitted to the terminal, and cut mid-value with no
// counterpart anywhere on screen.
//
// Identity now keeps the first row on its own and the axes take a second one
// under it, which is counted once against the list. That row still cannot hold
// everything at the floor, so what it cannot draw it drops whole and names — and
// the readout it points at is where the dropped value is readable, at every
// reviewed size.
//
// Session 5 owns this; it is the rewritten form of
// TestUXClarityCombinedContextExceedsTheOneRowHeader.
func TestUXClarityCombinedContextIsReadableAtEverySize(t *testing.T) {
	widest := uxClarityMatrixSizes[len(uxClarityMatrixSizes)-1]
	narrowest := uxClarityMatrixSizes[0]

	_, wideView := uxClarityStagedDriver(t, "combined-context", widest[0], widest[1])
	// The axes are named by the model rather than by this test, so the contract
	// cannot drift from the view's own vocabulary.
	entries := wideView.BuildViewContextEntries(widest[0])
	if len(entries) < 3 {
		t.Fatalf("the staged state carries %d context axes, which is not the combination under test: %+v",
			len(entries), entries)
	}

	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			driver, view := uxClarityStagedDriver(t, "combined-context", size[0], size[1])
			title := view.BuildOutlinerTitleLine(size[0])
			context := view.BuildOutlinerContextRow(size[0])
			if got := lipgloss.Height(title) + lipgloss.Height(context); got != 2 {
				t.Fatalf("the header is %d rows tall at %dx%d; with context live it is exactly two",
					got, size[0], size[1])
			}
			for name, row := range map[string]string{"title": title, "context": context} {
				if got := ansi.StringWidth(ansi.Strip(row)); got > size[0] {
					t.Fatalf("the %s row is %d cells in a %d-cell terminal", name, got, size[0])
				}
			}

			// Whatever the row could not draw, it says so and names where the whole
			// of it is: a shortened value with no forwarding address is the failure
			// this session removes.
			plainContext := ansi.Strip(context)
			readout := uxClarityReadoutText(t, driver)
			for _, entry := range entries {
				if strings.Contains(plainContext, ansi.Strip(entry.Row)) {
					continue
				}
				// The match count is a search badge, shed whole before the query and
				// stated again on the readout's query row, so the query drawn without
				// it is the entry drawn rather than a dropped one.
				if entry.Label == "query" && strings.Contains(plainContext, ansi.Strip(entry.RowCompact)) {
					if !strings.Contains(readout, "match") {
						t.Fatalf("the context row shed the match count and the readout does not state it:\n%s", readout)
					}
					continue
				}
				if !strings.Contains(plainContext, "details") {
					t.Fatalf("the context row dropped %q without naming the readout: %q", entry.Label, plainContext)
				}
				if !strings.Contains(readout, entry.Value) {
					t.Fatalf("axis %q is on neither the context row nor the readout:\ncontext: %q\nreadout:\n%s",
						entry.Label, plainContext, readout)
				}
			}
		})
	}

	// The two ends of the matrix are the two behaviors: the widest draws every
	// axis outright, and the floor is where dropping and forwarding start.
	_, narrowView := uxClarityStagedDriver(t, "combined-context", narrowest[0], narrowest[1])
	if got := ansi.Strip(narrowView.BuildOutlinerContextRow(narrowest[0])); !strings.Contains(got, "more") {
		t.Fatalf("the context row at %dx%d fits every axis, so the floor is not where it runs out: %q",
			narrowest[0], narrowest[1], got)
	}
	wide := ansi.Strip(wideView.BuildOutlinerContextRow(widest[0]))
	for _, entry := range entries {
		if !strings.Contains(wide, ansi.Strip(entry.Row)) {
			t.Fatalf("the widest reviewed terminal already drops %q: %q", entry.Label, wide)
		}
	}
}

// uxClarityReadoutText opens the query readout and pages it to the end,
// returning everything it drew along the way. The readout is bounded, so
// "readable in Q" is a claim about a document a reader can reach rather than
// about one frame of it.
func uxClarityReadoutText(t *testing.T, driver *UITestDriver) string {
	t.Helper()
	uxClarityPress(t, driver, uxClarityKeys("Q")...)
	defer uxClarityPress(t, driver, uxClarityKeys("esc")...)

	var seen strings.Builder
	previous := ""
	for range 20 {
		frame := driver.Observe(ObserveOptions{}).PlainScreen
		seen.WriteString(frame)
		seen.WriteString("\n")
		if frame == previous {
			break
		}
		previous = frame
		uxClarityPress(t, driver, uxClarityKeys("pgdown")...)
	}
	return seen.String()
}

// TestUXClarityPeekDockIsAShareOfTheFrame states session 7's outcome for the
// geometry the baseline recorded as a constant. The dock used to cost the same
// seven rows at the app's floor as at the widest reviewed terminal, so a tall
// terminal read a long document five lines at a time while a short one gave up
// the same rows a tall one could spare. It is now a share of the frame —
// clamp(floor(H/3), 7, 12) — and the rows it costs therefore differ by height.
//
// Capture raised over it is where the arithmetic runs out, so that is the frame
// every size is measured in: the dock shrinks to fit the panel and is still
// drawn whole, the list still keeps rows, and closing the panel gives the
// resting dock back with Peek still open on the same row.
//
// Session 7 owns this; it is the rewritten form of
// TestUXClarityCaptureOverPeekKeepsAFixedDock. The daily-use plan's own session
// 7 later replaced the typing half: a content-sized capture panel grows with its
// draft, so the assertion is the direction of that cost rather than its absence.
func TestUXClarityPeekDockIsAShareOfTheFrame(t *testing.T) {
	docks := make(map[[2]int]int, len(uxClarityMatrixSizes))
	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			driver, view := uxClarityStagedDriver(t, "capture-over-peek", size[0], size[1])
			obs := driver.Observe(ObserveOptions{})
			if len(obs.LayoutIssues) > 0 {
				t.Fatalf("capture over Peek at %dx%d: %+v\n%s", size[0], size[1], obs.LayoutIssues, obs.PlainScreen)
			}
			if !view.PeekDockVisible() {
				t.Fatal("the dock closed when capture opened")
			}
			if !strings.Contains(obs.PlainScreen, "── Capture ─") {
				t.Fatalf("the capture panel is not drawn over the dock:\n%s", obs.PlainScreen)
			}
			// Every row the dock drew is on screen, its position row last, which
			// is what proves the whole dock is there rather than clipped into it.
			if _, _, drawn := uxPeekDockSpan(strings.Split(obs.PlainScreen, "\n"), view); !drawn {
				t.Fatalf("the dock's position line is missing at %dx%d:\n%s", size[0], size[1], obs.PlainScreen)
			}
			// Under the panel the dock may be shorter than the frame alone would
			// give it, and it may never be shorter than the three rows a displayed
			// dock owes: one content line, the divider, and the position line.
			underCapture := view.PeekDockRows()
			if underCapture != len(view.PeekDockLines()) {
				t.Fatalf("the dock reports %d rows and drew %d", underCapture, len(view.PeekDockLines()))
			}
			if underCapture < 3 {
				t.Fatalf("the dock under capture is %d rows, want at least three", underCapture)
			}

			// Typing is a growth rather than a resize, and the daily-use plan's
			// own session 7 is why: the panel measures the draft, the preview, and
			// the suggestion list it holds, so a title, an autocompleting sigil,
			// and a refused one may each cost the dock a row. What holds through
			// all of them is that the dock only ever yields rows to the panel, is
			// still drawn whole, and never falls under the three a displayed dock
			// owes.
			for _, key := range []string{"R", "e", "v", "i", "e", "w", " ", "#"} {
				uxClarityPress(t, driver, uxClarityKeys(key)...)
				got := view.PeekDockRows()
				if drawn := len(view.PeekDockLines()); got != drawn {
					t.Fatalf("after %q the dock reports %d rows and drew %d", key, got, drawn)
				}
				if got < 3 {
					t.Fatalf("typing %q left the dock %d rows, want at least three", key, got)
				}
				if got > underCapture {
					t.Fatalf("typing %q grew the dock to %d rows from the %d the open left it",
						key, got, underCapture)
				}
			}
			after := driver.Observe(ObserveOptions{})
			if len(after.LayoutIssues) > 0 {
				t.Fatalf("typing under capture at %dx%d: %+v\n%s",
					size[0], size[1], after.LayoutIssues, after.PlainScreen)
			}

			// Closing capture restores the presentation and the Peek intent, and
			// the dock comes back to the height the frame alone gives it.
			uxClarityPress(t, driver, uxClarityKeys("esc")...)
			if prompt := driver.Observe(ObserveOptions{}); strings.Contains(prompt.PlainScreen, "Discard") {
				uxClarityPress(t, driver, uxClarityKeys("y")...)
			}
			if !view.PeekDockVisible() {
				t.Fatal("closing capture closed the Peek dock with it")
			}
			resting := view.PeekDockRows()
			if resting < underCapture {
				t.Fatalf("the resting dock is %d rows and the one under capture was %d", resting, underCapture)
			}
			docks[size] = resting
		})
	}

	if len(docks) != len(uxClarityMatrixSizes) {
		t.Fatalf("measured %d of %d sizes", len(docks), len(uxClarityMatrixSizes))
	}
	// The share is what the rewrite bought, so the reviewed extremes must differ:
	// the floor keeps the seven rows a readable page needs and the tallest
	// terminal spends more of itself on the document.
	shortest, tallest := docks[uxClarityMatrixSizes[0]], docks[uxClarityMatrixSizes[len(uxClarityMatrixSizes)-1]]
	if tallest <= shortest {
		t.Fatalf("the dock costs %d rows at the floor and %d at the tallest reviewed terminal, want it to grow with the frame",
			shortest, tallest)
	}
	// And it is monotone in height, which is what makes a resize one deliberate
	// move rather than a number a reader cannot predict.
	for index := 1; index < len(uxClarityMatrixSizes); index++ {
		previous, current := uxClarityMatrixSizes[index-1], uxClarityMatrixSizes[index]
		if docks[current] < docks[previous] {
			t.Fatalf("the dock costs %d rows at %s and %d at the taller %s",
				docks[previous], uxClaritySizeName(previous), docks[current], uxClaritySizeName(current))
		}
	}
}

// TestUXClarityRoutineSuccessIsQuiet is the contract session 2 replaced the
// baseline with. An ordinary, expected, undramatic success — copying a title —
// used to arm the same twenty-frame divider animation a storage failure does.
// Now it reports and stops there: the footer says what happened, and the motion
// is kept for the messages that are asking to be noticed.
//
// Both halves are asserted from the same staged frame through real key presses,
// because the risk the split introduces is not that the success stays loud but
// that the warning goes quiet with it.
func TestUXClarityRoutineSuccessIsQuiet(t *testing.T) {
	driver, view := uxClarityStagedDriver(t, "launch", uxClarityDailySize[0], uxClarityDailySize[1])
	app := driver.App()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	if app.feedback.flash.seq != 0 {
		t.Fatalf("the launch frame is already flashing at generation %d", app.feedback.flash.seq)
	}

	uxClarityPress(t, driver, uxClarityKeys("y")...)

	if !strings.Contains(uxClarityFooter(driver), "Copied title to clipboard") {
		t.Fatalf("y did not report a routine success:\n%s", uxClarityFooter(driver))
	}
	if app.feedback.severity != flashSuccess {
		t.Fatalf("recorded severity = %v, want flashSuccess; the severity is what the split reads", app.feedback.severity)
	}
	if app.feedback.flash.active() {
		t.Fatalf("a routine copy animated the divider: %+v", app.feedback.flash)
	}
	if app.feedback.flash.seq != 0 {
		t.Fatalf("a routine copy spent flash generation %d", app.feedback.flash.seq)
	}

	// The abandoned chord is the warning that reaches the footer on the same
	// path, from the same view, by the same kind of keystroke — so it is the one
	// that proves the divider still moves for the messages that earn it.
	uxClarityPress(t, driver, uxClarityKeys("w", "q")...)

	if !strings.Contains(uxClarityFooter(driver), "wq is not a workflow action") {
		t.Fatalf("wq did not report the abandoned chord:\n%s", uxClarityFooter(driver))
	}
	if got := app.feedback.flash.severity; got != flashWarning {
		t.Fatalf("flash severity = %v, want flashWarning", got)
	}
	if app.feedback.flash.frame != 0 {
		t.Fatalf("flash frame = %d, want the first of %d", app.feedback.flash.frame, footerFlashFrames)
	}

	// And the quiet line that follows takes the warning's animation down with
	// it, rather than leaving a wash and a moving glyph under text nobody needs
	// alerting to.
	warned := app.feedback.flash.seq
	uxClarityPress(t, driver, uxClarityKeys("y")...)

	if app.feedback.flash.active() {
		t.Fatalf("the superseded warning kept animating under a routine success: %+v", app.feedback.flash)
	}
	if app.feedback.flashSeq <= warned {
		t.Fatalf("flash generation = %d, want it past the retired %d", app.feedback.flashSeq, warned)
	}
}

// uxClaritySizeName is the subtest name a reviewed size gets, so a failure names
// the terminal it happened in the same way the exported frames do.
func uxClaritySizeName(size [2]int) string {
	return fmt.Sprintf("%dx%d", size[0], size[1])
}
