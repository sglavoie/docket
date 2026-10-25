package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/outliner"
)

// uxDailyUseIntegratedCombinedState is one pair of occupants raised together and
// the reason the pair is worth raising. Each is opened with public keys from the
// resting Outliner, so the state under test is one a user reaches.
type uxDailyUseIntegratedCombinedState struct {
	name  string
	why   string
	stage func(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel)
	// require names text the frame has to still carry once both occupants stand,
	// which is what separates "it fits" from "it fits because something was
	// dropped".
	require []string
}

// uxDailyUseIntegratedCombinedStates are the four combinations that are a
// single frame: two of this plan's changes standing at once over the same rows.
// The fifth the plan named — resizing while editing — is a round trip rather
// than a frame, so it has its own test below.
var uxDailyUseIntegratedCombinedStates = []uxDailyUseIntegratedCombinedState{
	{
		name:  "capture-over-peek",
		why:   "a content-sized panel and a dock sized as a share of the same frame",
		stage: uxDailyUseIntegratedStageCaptureOverPeek,
		// The panel, the dock's own position row, and the guidance capture owes.
		require: []string{uxDailyUseCaptureDivider, "Enter create"},
	},
	{
		name:    "validation-and-guidance",
		why:     "a refusal the panel has to keep on screen beside the keys that answer it",
		stage:   uxDailyUseIntegratedStageValidationAndGuidance,
		require: []string{"$ready", "Esc keep draft"},
	},
	{
		name:    "selection-and-filters",
		why:     "a persistent summary read against a projection a facet has narrowed",
		stage:   uxDailyUseIntegratedStageSelectionAndFilters,
		require: []string{"selected"},
	},
	{
		name:    "long-project-names-and-urgency",
		why:     "a long project heading and a deadline indicator sharing one frame",
		stage:   uxDailyUseIntegratedStageLongNamesAndUrgency,
		require: []string{"[ux-"},
	},
}

// TestUXDailyUseIntegratedCombinedStatesKeepTheirRows raises each pair at every
// reviewed size and requires the frame to hold both.
//
// A pair is not the sum of its halves here: the panel's rows are a loan against
// the same content height the dock takes its share of, the summary rides the
// context row a facet also writes to, and the project column is measured over a
// projection a filter just shortened. Every one of those is one authority being
// read by two occupants, and a frame that drops an occupant rather than shrinking
// it is the failure the pass exists to find.
func TestUXDailyUseIntegratedCombinedStatesKeepTheirRows(t *testing.T) {
	for _, state := range uxDailyUseIntegratedCombinedStates {
		for _, size := range uxDailyUseMatrixSizes {
			t.Run(fmt.Sprintf("%s-%s", state.name, uxDailyUseSizeName(size)), func(t *testing.T) {
				driver, view := uxDailyUseDriver(t, FixtureUXAcceptance, size[0], size[1])
				state.stage(t, driver, view)

				obs := driver.Observe(ObserveOptions{})
				assertUXScreenFitsItsTerminal(t, obs, size)
				screen := obs.PlainScreen + "\n" + uxClarityFooter(driver)
				for _, required := range state.require {
					if !strings.Contains(screen, required) {
						t.Fatalf("%s (%s) dropped %q at %s\n%s",
							state.name, state.why, required, uxDailyUseSizeName(size), obs.PlainScreen)
					}
				}
			})
		}
	}
}

// uxDailyUseIntegratedResizeDraft is the text the resize round trip is measured
// against. It is longer than the narrowest input row so that at least one
// reviewed width has to wrap it, which is what makes the panel's demand change
// with the terminal rather than stay constant through the walk.
const uxDailyUseIntegratedResizeDraft = "Reconcile the vendor invoices against the quarterly ledger"

// TestUXDailyUseIntegratedResizingWhileEditingKeepsTheDraft walks a live draft
// through every reviewed size and back to the one it started in.
//
// Resizing is where a borrowed row is most easily lost: the panel's demand is
// re-measured from the stored draft at the new width, the host is re-sized once
// per update rather than per frame, and `SetSize` deliberately drops the window
// the outline was holding. So the round trip asks the two things a user would:
// the draft is still there and still editable at every stop, and returning to
// the starting size returns the starting frame byte for byte rather than a frame
// that merely looks similar.
func TestUXDailyUseIntegratedResizingWhileEditingKeepsTheDraft(t *testing.T) {
	start := uxDailyUseDailySize
	driver, _ := uxDailyUseDriver(t, FixtureUXAcceptance, start[0], start[1])
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "a"},
		UITestAction{Type: UITestActionText, Text: uxDailyUseIntegratedResizeDraft},
	)
	before := driver.Render()

	for _, size := range uxDailyUseMatrixSizes {
		driver.Resize(size[0], size[1])
		obs := driver.Observe(ObserveOptions{})
		assertUXScreenFitsItsTerminal(t, obs, size)
		if !strings.Contains(obs.PlainScreen, uxDailyUseCaptureDivider) {
			t.Fatalf("resizing to %s closed the capture panel\n%s", uxDailyUseSizeName(size), obs.PlainScreen)
		}
		// The draft is read back from the screen rather than from the model, since
		// a draft the model holds and the panel stops drawing is the same loss to
		// whoever is typing it. Narrow rows wrap it, so the words are matched
		// individually.
		for _, word := range strings.Fields(uxDailyUseIntegratedResizeDraft) {
			if !strings.Contains(obs.PlainScreen, word) {
				t.Fatalf("the draft lost %q at %s\n%s", word, uxDailyUseSizeName(size), obs.PlainScreen)
			}
		}
	}

	driver.Resize(start[0], start[1])
	if after := driver.Render(); after != before {
		t.Fatalf("the round trip did not restore the frame it started in\nbefore:\n%s\nafter:\n%s",
			ansi.Strip(before), ansi.Strip(after))
	}

	// And the draft the walk preserved still commits, which is the half a frame
	// comparison cannot see.
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "enter"},
		UITestAction{Type: UITestActionWait},
	)
	view := driver.App().ActiveOutliner()
	if view == nil {
		t.Fatal("committing the resized draft left no outline")
	}
	uxDailyUseIntegratedRequireStored(t, view, uxDailyUseIntegratedResizeDraft)
}

// uxDailyUseIntegratedStageCaptureOverPeek opens the dock and then capture over
// it, which is the one frame where a share and a loan are taken from the same
// rows. The dock yields rows to the panel and stays whole: it may shrink, and it
// may not be clipped away or lifted off the bottom of the list.
func uxDailyUseIntegratedStageCaptureOverPeek(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("b")...)
	if !view.PeekDockVisible() {
		t.Fatal("`b` did not open the dock")
	}
	resting := view.PeekDockRows()
	uxClarityPress(t, driver, uxClarityKeys("a")...)
	uxClarityPress(t, driver, UITestAction{Type: UITestActionText, Text: "one"})

	under := view.PeekDockRows()
	switch {
	case under > resting:
		t.Fatalf("capture grew the dock from %d rows to %d", resting, under)
	case under > 0 && under < 3:
		t.Fatalf("the dock is %d rows under capture, which is fewer than a displayed dock owes", under)
	}
	if under > 0 && len(view.PeekDockLines()) != under {
		t.Fatalf("the dock reports %d rows and drew %d", under, len(view.PeekDockLines()))
	}
}

// uxDailyUseIntegratedStageValidationAndGuidance types a draft the grammar
// recognizes and cannot honor. The panel gives the input its first row
// unconditionally and yields the rest to refusals, so the message and the keys
// that answer it are readable together at every width — which is the whole
// reason a refusal is a panel row rather than a toast.
func uxDailyUseIntegratedStageValidationAndGuidance(t *testing.T, driver *UITestDriver, _ *outliner.OutlinerModel) {
	t.Helper()
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "a"},
		UITestAction{Type: UITestActionText, Text: "Retire the staging bastion $ready "},
		UITestAction{Type: UITestActionKey, Key: "enter"},
	)
}

// uxDailyUseIntegratedStageSelectionAndFilters narrows the projection with a
// facet and then selects inside it, so the summary is read against rows a filter
// is already hiding. The context row carries both axes, which is where the two
// changes meet.
func uxDailyUseIntegratedStageSelectionAndFilters(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "/"},
		UITestAction{Type: UITestActionText, Text: "the"},
		UITestAction{Type: UITestActionKey, Key: "enter"},
	)
	uxDailyUseIntegratedRequireTaskCursor(t, view, "a narrowed projection")
	uxClarityPress(t, driver, uxClarityKeys("v", "j", "v")...)
	if view.SelectionCounts().Total < 2 {
		t.Fatalf("the selection inside the narrowed projection holds %d tasks", view.SelectionCounts().Total)
	}
}

// uxDailyUseIntegratedStageLongNamesAndUrgency seats the cursor on the overdue
// row, which is the fixture's longest title in the project whose slug is longest.
// The row therefore has to spend its width on a deadline indicator, a project
// field, and a title at once, and the column is measured over a projection where
// every other row is shorter.
func uxDailyUseIntegratedStageLongNamesAndUrgency(t *testing.T, _ *UITestDriver, view *outliner.OutlinerModel) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
}
