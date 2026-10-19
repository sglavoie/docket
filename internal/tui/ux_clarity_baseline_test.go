package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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
			for _, control := range []string{"Enter create", "Esc keep draft"} {
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
// They are the ladder's leading three rather than a spread across it, because
// the daily size sheds the tail: a rank the resting footer never reaches would
// make every absence below vacuous.
var uxClarityOutlinerHostRanks = []string{"a capture", "w workflow", "e edit"}

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
	// discard question out of the shell's guidance contract. Its answer line is
	// the footer's, since "no" here keeps editing rather than canceling.
	for _, required := range []string{"Discard unsaved changes?", "Y discard · N/Esc keep editing"} {
		if !strings.Contains(plain, required) {
			t.Fatalf("the confirmation box does not draw %q:\n%s", required, plain)
		}
	}
	// The composer is still painted underneath rather than replaced, and its
	// divider is the cheapest proof that the panel still owns its rows.
	if !strings.Contains(plain, "── Capture ─") {
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
