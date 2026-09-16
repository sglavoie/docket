package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// These are the combined states the UX clarity work is about, written down while
// they still behave the old way. Each one is an expectation a named later session
// deliberately replaces, so each test says which session owns it and what that
// session is expected to change it to. They are contracts about the arrangement
// of the screen — which row carries what, how many rows a thing costs, what a
// width does to it — and never whole-screen snapshots, because a snapshot of a
// state six sessions are about to move is a snapshot that fails for every reason
// at once and names none of them.
//
// A test here failing after a clarity session is not automatically a regression.
// It is the session's own signal that it reached the behavior it was aimed at,
// and the session that reaches it owns rewriting the contract in the same change.
// docs/tui-testing/ux-clarity-regression-map.md is the index of who owns what.
//
// A rewritten one stays here rather than moving out, and stops being a baseline
// when it does: it now states the behavior its session reached and is an
// ordinary contract over it. The map records which session did the rewriting, so
// the two kinds stay tellable apart without reading every doc comment.

// uxClarityMatrixSizes is the review's floor, its two working sizes, and its
// widest; the same four the exported frames are taken at. The combined-state
// contracts below read geometry at more than one of them, because every
// expectation here is about what happens when there is not enough room.
var uxClarityMatrixSizes = uxPresentationMatrixSizes

// uxClarityDailySize is the size the plan optimizes for.
var uxClarityDailySize = [2]int{100, 35}

// TestUXClarityFeedbackTakesTheCaptureControlsRow records that a routine success
// message displaces the capture panel's controls rather than standing beside
// them: today the footer has one occupant and status outranks contextual
// guidance, so copying a title and then opening capture leaves the user in a
// text field whose keys are no longer named anywhere on screen.
//
// Session 4 owns this. It separates feedback from controls, after which the
// controls are expected to survive the message.
func TestUXClarityFeedbackTakesTheCaptureControlsRow(t *testing.T) {
	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			driver, _ := uxClarityStagedDriver(t, "feedback-over-guidance", size[0], size[1])
			obs := driver.Observe(ObserveOptions{})
			footer := uxClarityFooter(driver)

			if !slices.Contains(obs.ViewStack, "TaskComposer") {
				t.Fatalf("the composer is not open, so the state under test was not reached: %q", obs.ViewStack)
			}
			if !strings.Contains(footer, "Copied title to clipboard") {
				t.Fatalf("the footer does not carry the copy status:\n%s", footer)
			}
			// The capture controls are the guidance the composer publishes. Both
			// halves are named, because a footer that kept one of them would be a
			// different arrangement than the one this test is recording.
			for _, control := range []string{"Enter create", "Esc cancel"} {
				if strings.Contains(footer, control) {
					t.Fatalf("the footer still names %q beside the status, which is session 4's outcome, not today's:\n%s",
						control, footer)
				}
			}
		})
	}
}

// TestUXClarityPickerLeavesHostGuidanceOnScreen records the opposite imbalance:
// an overlay that owns every key still lets the host underneath publish its own
// ranks, so two instruction sets are readable at once and only one of them is
// live. The picker is the shell-mounted value picker over a task row.
//
// Session 3 owns this. It gives the active interaction sole ownership of
// guidance while keeping the measured space, after which the host's ranks are
// expected to be absent and the rows they occupied still reserved.
func TestUXClarityPickerLeavesHostGuidanceOnScreen(t *testing.T) {
	driver, _ := uxClarityStagedDriver(t, "picker-over-outline", uxClarityDailySize[0], uxClarityDailySize[1])
	obs := driver.Observe(ObserveOptions{})
	plain := obs.PlainScreen

	pickerGuidance := "Enter apply · Esc back"
	if !strings.Contains(plain, pickerGuidance) {
		t.Fatalf("the picker does not publish %q, so the state under test was not reached:\n%s", pickerGuidance, plain)
	}
	// The host's ranks are read out of the footer specifically: the picker's own
	// instructions are inside its box, and the point of the contract is that the
	// row below the box is still answering for a surface that cannot be reached.
	footer := uxClarityFooter(driver)
	hostRanks := []string{"w workflow", "Space actions", "a capture"}
	for _, rank := range hostRanks {
		if !strings.Contains(footer, rank) {
			t.Fatalf("the host footer no longer names %q, which is session 3's outcome, not today's:\n%s", rank, footer)
		}
	}
}

// TestUXClarityDiscardPromptStandsOnTheOpenComposer records the nesting: a
// confirmation over a draft-bearing panel over the outline is three
// presentations deep, the box and the footer both answer for the innermost one,
// and the two underneath keep drawing. It is the case a rule about one active
// instruction set has to be written against, because here the instruction set
// that is live belongs to neither of the presentations that own rows.
//
// Session 3 owns this. The footer's ownership is expected to survive; the host
// guidance underneath is what changes.
func TestUXClarityDiscardPromptStandsOnTheOpenComposer(t *testing.T) {
	driver, _ := uxClarityStagedDriver(t, "nested-discard-prompt", uxClarityDailySize[0], uxClarityDailySize[1])
	obs := driver.Observe(ObserveOptions{})
	plain := obs.PlainScreen

	if len(obs.ViewStack) < 2 {
		t.Fatalf("the view stack is not nested: %q", obs.ViewStack)
	}
	if !slices.Contains(obs.ViewStack, "TaskComposer") {
		t.Fatalf("the composer left the stack under the prompt: %q", obs.ViewStack)
	}
	// The prompt draws its own question in its own box, which is what keeps the
	// discard question out of the shell's guidance contract.
	for _, required := range []string{"Discard unsaved changes?", "y confirm · n/Esc cancel"} {
		if !strings.Contains(plain, required) {
			t.Fatalf("the confirmation box does not draw %q:\n%s", required, plain)
		}
	}
	// The composer is still painted underneath rather than replaced, and its
	// divider is the cheapest proof that the panel still owns its rows.
	if !strings.Contains(plain, "Capture → Unfiled") {
		t.Fatalf("the composer host stopped drawing under the prompt:\n%s", plain)
	}
	footer := uxClarityFooter(driver)
	if !strings.Contains(footer, "Y discard") || !strings.Contains(footer, "N/Esc keep editing") {
		t.Fatalf("the footer does not answer for the prompt:\n%s", footer)
	}
	if strings.Contains(footer, "Enter create") {
		t.Fatalf("the footer names the composer's own controls beneath a prompt that owns every key:\n%s", footer)
	}
}

// TestUXClarityCombinedContextExceedsTheOneRowHeader records what the single
// header row does when every context axis is true at once: the segments are
// joined into one line, the line is fitted to the terminal, and the axes past
// the fit are dropped with no counterpart anywhere on screen. At the widest
// reviewed size all of them fit; at the floor the line truncates mid-value, and
// what it cut is readable nowhere.
//
// Session 5 owns this. It adds an optional second context row and scrollable
// details under `Q`, after which the header is still counted once and the
// dropped axes are expected to have somewhere to go.
func TestUXClarityCombinedContextExceedsTheOneRowHeader(t *testing.T) {
	widest := uxClarityMatrixSizes[len(uxClarityMatrixSizes)-1]
	narrowest := uxClarityMatrixSizes[0]

	_, wideView := uxClarityStagedDriver(t, "combined-context", widest[0], widest[1])
	// The axes are named by the model rather than by this test, so the contract
	// cannot drift from the view's own vocabulary; the styling is dropped because
	// what is under test is whether the text reaches the screen at all.
	segments := make([]string, 0, 4)
	for _, segment := range wideView.BuildOutlinerInteractionContext(widest[0]) {
		segments = append(segments, ansi.Strip(segment))
	}
	if len(segments) < 3 {
		t.Fatalf("the staged state carries %d context segments, which is not the combination under test: %q",
			len(segments), segments)
	}

	// The header stays one row and inside its terminal at every reviewed size,
	// which is the measurement session 5 has to keep counting once.
	dropped := make(map[[2]int][]string, len(uxClarityMatrixSizes))
	for _, size := range uxClarityMatrixSizes {
		t.Run(uxClaritySizeName(size), func(t *testing.T) {
			driver, view := uxClarityStagedDriver(t, "combined-context", size[0], size[1])
			header := view.BuildOutlinerTitleLine(size[0])
			if got := lipgloss.Height(header); got != 1 {
				t.Fatalf("the header is %d rows tall at %dx%d; today it is exactly one", got, size[0], size[1])
			}
			if got := ansi.StringWidth(ansi.Strip(header)); got > size[0] {
				t.Fatalf("the header is %d cells in a %d-cell terminal", got, size[0])
			}
			// Every axis is still true, and the header is the only place any of
			// them is spelled, so whichever ones do not fit are on screen nowhere.
			obs := driver.Observe(ObserveOptions{})
			missing := make([]string, 0, len(segments))
			for _, segment := range segments {
				if !strings.Contains(obs.PlainScreen, segment) {
					missing = append(missing, segment)
				}
			}
			dropped[size] = missing
		})
	}

	if got := dropped[widest]; len(got) > 0 {
		t.Fatalf("the widest reviewed terminal already drops %q, so the combination is not the one recorded here", got)
	}
	if got := dropped[narrowest]; len(got) == 0 {
		t.Fatalf("every context axis survives %dx%d, which is session 5's outcome, not today's",
			narrowest[0], narrowest[1])
	}
	_, narrowView := uxClarityStagedDriver(t, "combined-context", narrowest[0], narrowest[1])
	narrow := ansi.Strip(narrowView.BuildOutlinerTitleLine(narrowest[0]))
	if !strings.Contains(narrow, "…") {
		t.Fatalf("the header at %dx%d drops axes without marking the cut: %q",
			narrowest[0], narrowest[1], narrow)
	}
}

// TestUXClarityCaptureOverPeekKeepsAFixedDock records Peek's geometry as a
// constant: the dock costs the same rows at the app's floor as it does at the
// widest reviewed terminal, so every row the terminal does not have is taken
// out of the list instead. Raising capture over it is where that arithmetic
// runs out, and the floor is where it runs out first.
//
// Session 7 owns this. It makes the dock responsive — clamp(floor(H/3), 7, 12),
// eleven rows at 100×35, and a three-row displayed minimum under pressure —
// after which these row counts are expected to differ by height.
func TestUXClarityCaptureOverPeekKeepsAFixedDock(t *testing.T) {
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
			if !strings.Contains(obs.PlainScreen, "Capture → Unfiled") {
				t.Fatalf("the capture panel is not drawn over the dock:\n%s", obs.PlainScreen)
			}
			// The position line is drawn for every document, so it is the one row
			// that proves the whole dock is on screen rather than clipped into it.
			if !uxPeekPositionLine.MatchString(obs.PlainScreen) {
				t.Fatalf("the dock's position line is missing at %dx%d:\n%s", size[0], size[1], obs.PlainScreen)
			}
			docks[size] = len(view.PeekDockLines())
		})
	}

	if len(docks) != len(uxClarityMatrixSizes) {
		t.Fatalf("measured %d of %d sizes", len(docks), len(uxClarityMatrixSizes))
	}
	first := docks[uxClarityMatrixSizes[0]]
	for size, rows := range docks {
		if rows != first {
			t.Fatalf("the dock costs %d rows at %dx%d and %d rows at %dx%d, so it is already responsive; that is session 7's outcome, not today's",
				rows, size[0], size[1], first, uxClarityMatrixSizes[0][0], uxClarityMatrixSizes[0][1])
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
