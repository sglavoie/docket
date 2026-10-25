package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

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

	for _, fragment := range []string{"No check-in", "Set Up Check-Ins"} {
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
