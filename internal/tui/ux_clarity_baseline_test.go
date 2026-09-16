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

// TestUXClarityFeedbackStandsBesideTheCaptureControls states session 4's outcome
// for the imbalance the baseline recorded: a routine success message used to
// displace the capture panel's controls, so copying a title and then opening
// capture left the user in a text field whose keys were named nowhere on screen.
// The footer now says both — what just happened on its own row, and what the
// keys still do on the row under it.
//
// The order is asserted rather than the mere presence of the two, because the
// controls are what stays: putting feedback above them keeps commit and cancel
// on the terminal's last row whether a message is standing or not, so they do
// not move under a message arriving and expiring.
//
// Every size in the matrix is read, the 60-column floor included, because that
// is where the footer has the least room to hold two things at once and where a
// composition that only fits one would show it first.
//
// Session 4 owns this; it is the rewritten form of
// TestUXClarityFeedbackTakesTheCaptureControlsRow.
func TestUXClarityFeedbackStandsBesideTheCaptureControls(t *testing.T) {
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
			// halves are named, because a footer that kept one of them would leave
			// the other act unreachable by name.
			for _, control := range []string{"Enter create", "Esc cancel"} {
				if !strings.Contains(footer, control) {
					t.Fatalf("the status displaced %q, leaving the open draft's control unnamed:\n%s", control, footer)
				}
			}
			if got := strings.Count(footer, "Copied title to clipboard"); got != 1 {
				t.Fatalf("the status is on %d rows, want exactly one:\n%s", got, footer)
			}

			status := uxClarityFooterRowIndex(t, footer, "Copied title to clipboard")
			controls := uxClarityFooterRowIndex(t, footer, "Enter create")
			if status >= controls {
				t.Fatalf("the status is on row %d and the controls on row %d; controls keep the last row:\n%s",
					status, controls, footer)
			}
		})
	}
}

// uxClarityFooterRowIndex is which row of a rendered footer carries a fragment.
// The contracts here are about arrangement, so more than one of them needs to
// ask where something landed rather than only whether it is there.
func uxClarityFooterRowIndex(t *testing.T, footer, fragment string) int {
	t.Helper()
	for index, line := range strings.Split(footer, "\n") {
		if strings.Contains(line, fragment) {
			return index
		}
	}
	t.Fatalf("footer does not carry %q:\n%s", fragment, footer)
	return -1
}

// uxClarityOutlinerHostRanks are the host's own footer ranks, the text that
// proves the Outliner is still the layer being answered for. They are named here
// once because more than one contract below asks whether they are on screen.
var uxClarityOutlinerHostRanks = []string{"w workflow", "Space actions", "a capture"}

// TestUXClarityPickerOwnsGuidanceAndKeepsHostRows states session 3's outcome for
// the imbalance the baseline recorded: an overlay that owns every key is now the
// only layer publishing instructions, and the rows the host's ranks occupied are
// still reserved underneath it. The picker is the shell-mounted value picker
// over a task row.
//
// Both halves are asserted against the same size's resting frame, because "the
// space is retained" is a claim about a difference: a footer that shed the text
// and the rows together would satisfy the absence alone. Closing the picker is
// the third half — the host gets its ranks back exactly as they were, which is
// what makes the suppression a loan rather than a write.
//
// Session 3 owns this; it is the rewritten form of
// TestUXClarityPickerLeavesHostGuidanceOnScreen.
func TestUXClarityPickerOwnsGuidanceAndKeepsHostRows(t *testing.T) {
	width, height := uxClarityDailySize[0], uxClarityDailySize[1]

	// The resting reading is taken from a second launch at the same size with
	// the same cursor, rather than from this driver before the press, so the
	// frame the staged assertions run against is the staged one throughout.
	resting := uxClarityRestingFooter(t, width, height)

	driver, _ := uxClarityStagedDriver(t, "picker-over-outline", width, height)
	obs := driver.Observe(ObserveOptions{})
	plain := obs.PlainScreen

	pickerGuidance := "Enter apply · Esc back"
	if !strings.Contains(plain, pickerGuidance) {
		t.Fatalf("the picker does not publish %q, so the state under test was not reached:\n%s", pickerGuidance, plain)
	}

	// The host's ranks are read out of the footer specifically: the picker's own
	// instructions are inside its box, and the contract is that the row below the
	// box no longer answers for a surface no key can reach.
	footer := uxClarityFooter(driver)
	for _, rank := range uxClarityOutlinerHostRanks {
		if strings.Contains(footer, rank) {
			t.Fatalf("the footer still names the host rank %q while the picker owns every key:\n%s", rank, footer)
		}
	}
	if strings.TrimSpace(footer) == "" && lipgloss.Height(footer) < 2 {
		t.Fatalf("the footer collapsed entirely rather than reserving its rows:\n%q", footer)
	}
	if got, want := lipgloss.Height(footer), lipgloss.Height(resting); got != want {
		t.Fatalf("the footer is %d rows with the picker open and %d rows at rest; the space is not retained", got, want)
	}

	// Closing hands the rows back with their text, byte for byte against the
	// resting frame the same launch produces.
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	if got := uxClarityFooter(driver); got != resting {
		t.Fatalf("closing the picker did not restore the host footer:\ngot:\n%s\nwant:\n%s", got, resting)
	}
}

// uxClarityRestingFooter launches the fixture at one size, seats the cursor
// where every staged frame seats it, and returns the footer before anything is
// opened over it.
func uxClarityRestingFooter(t *testing.T, width, height int) string {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, width, height)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	return uxClarityFooter(driver)
}

// TestUXClarityDiscardPromptStandsOnTheOpenComposer states the nesting: a
// confirmation over a draft-bearing panel over the outline is three
// presentations deep, the box and the footer both answer for the innermost one,
// and the two underneath keep drawing their rows without speaking.
//
// This is the case the ownership rule is written against, because the live
// instruction set belongs to neither of the presentations that own rows. It is
// also the one prompt the shell does not treat as a confirmation owner: the
// composer owns this question and answers for it through its own guidance, so
// the footer keeps publishing — what it publishes is the prompt's exits rather
// than the draft's controls.
//
// Session 3 owns this; the assertions on what is absent are its outcome.
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
	// Two presentations deep, neither of the ones underneath is speaking: the
	// composer's controls are gone above and the outline's ranks never reach the
	// footer at all.
	for _, rank := range uxClarityOutlinerHostRanks {
		if strings.Contains(footer, rank) {
			t.Fatalf("the footer names the outline rank %q under a prompt two presentations above it:\n%s", rank, footer)
		}
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
