package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// This file is the daily-use refinement's integrated acceptance pass. Sessions 2
// through 12 each changed one occupant of the frame and proved that occupant
// correct on its own, and the `ux_daily_use_baseline*` test files record each of
// those outcomes against the expectation it replaced. Nothing here
// restates one of them.
//
// What is here is the part no single session could see, because it lives between
// two changes that are each right on their own: a project column measured over
// the whole projection while a panel underneath borrows the window the column is
// read in; a footer note published by a commit that also moved the row the
// cursor was on; a selection summary counted once while a filter hides half its
// members. Those relationships are only true of the finished product, so this is
// the only pass that can ask about them.
//
// The vocabulary is public keys throughout, for the reason the plan gave: a
// gesture a test can make has to be a gesture a user can make, so an acceptance
// pass that reached past the key grammar would be accepting something nobody can
// drive.

// uxDailyUseIntegratedCapture is the task the loop creates. It is typed rather
// than built into a fixture because every committed scenario over these catalogs
// reads their row populations, and a row added to one would rewrite contracts
// this session is supposed to be confirming.
const uxDailyUseIntegratedCapture = "Read the overnight deploy report"

// uxDailyUseIntegratedEditSuffix is appended by the inline row editor, so the
// loop's edit step changes a committed title through the same `e` buffer a user
// would open rather than through a service call.
const uxDailyUseIntegratedEditSuffix = " and file the findings"

// uxDailyUseIntegratedLeadingActs is the head of the one footer ladder, in
// order. Session 12 sized that head to `chrome.MinTerminalWidth` exactly, so it
// is also the run every reviewed size has to keep; this pass reads it wherever a
// task row is resting, which is what a user does between every two steps below.
const uxDailyUseIntegratedLeadingActs = "a capture · w workflow · e edit · / search · b peek · ? help"

// TestUXDailyUseIntegratedDailyLoopHoldsEveryChangeTogether drives one workday
// through the public grammar — capture, triage, plan, start, edit, schedule,
// complete, undo — and reads the plan's own changes at each seam rather than at
// the end.
//
// The seams are the point. Capture commits a row into the projection the
// outline is scrolled in; the state change publishes a footer sentence about a
// task the same transaction may have moved to another lane; the inline editor
// takes its width from the row geometry a project column decided; undo restores
// a task the selection and the cursor have both already left. Each of those is
// two sessions' work meeting, and none of them is asserted anywhere else.
func TestUXDailyUseIntegratedDailyLoopHoldsEveryChangeTogether(t *testing.T) {
	size := uxDailyUseDailySize
	driver, view := uxDailyUseDriver(t, FixtureUXAcceptance, size[0], size[1])

	// Launch is the first step of the loop and not a preamble to it: the cursor
	// is already on work, so the next gesture is an act rather than a navigation.
	uxDailyUseIntegratedRequireTaskCursor(t, view, "launch")
	uxDailyUseIntegratedRequireLeadingActs(t, driver, "launch")

	// Capture. The panel is a bounded loan over the live list, so the assertion
	// is about both: the host is still drawn underneath, and the row the draft
	// names reaches the projection when it commits.
	uxClarityPress(t, driver, uxClarityKeys("a")...)
	panelFrame := driver.Observe(ObserveOptions{})
	if !strings.Contains(panelFrame.PlainScreen, uxDailyUseCaptureDivider) {
		t.Fatalf("`a` drew no capture panel\n%s", panelFrame.PlainScreen)
	}
	if !strings.Contains(panelFrame.PlainScreen, "Active Projects · All") {
		t.Fatalf("the capture panel replaced the list it was opened over\n%s", panelFrame.PlainScreen)
	}
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionText, Text: uxDailyUseIntegratedCapture},
		UITestAction{Type: UITestActionKey, Key: "enter"},
		UITestAction{Type: UITestActionWait},
	)
	uxDailyUseIntegratedRequireRow(t, driver, view, uxDailyUseIntegratedCapture)

	// Triage. The capture lands in the Inbox; the workflow chord is what moves it
	// on, and `w` is rank 2 of the head the launch footer just proved.
	seatCursorOnTaskTitled(t, view, uxDailyUseIntegratedCapture)
	uxDailyUseIntegratedRequireState(t, view, uxDailyUseIntegratedCapture, "inbox")
	uxClarityPress(t, driver, uxClarityKeys("w", "b")...)
	uxClarityPress(t, driver, uxClarityKeys()...)
	uxDailyUseIntegratedSettle(t, driver)
	uxDailyUseIntegratedRequireState(t, view, uxDailyUseIntegratedCapture, "backlog")

	// Plan. Today is a collection rather than a tenth state, which is the whole
	// of what session 5's division says; the stored state has to stay Backlog.
	seatCursorOnTaskTitled(t, view, uxDailyUseIntegratedCapture)
	uxClarityPress(t, driver, uxClarityKeys("w", "t")...)
	uxDailyUseIntegratedSettle(t, driver)
	uxDailyUseIntegratedRequireState(t, view, uxDailyUseIntegratedCapture, "backlog")

	// Start. This is the seam sessions 9 and 10 left: entering Doing without a
	// schedule says so in the footer and opens nothing, so the list is still the
	// view and the next key still belongs to the row.
	seatCursorOnTaskTitled(t, view, uxDailyUseIntegratedCapture)
	uxClarityPress(t, driver, uxClarityKeys("w", "g")...)
	uxDailyUseIntegratedSettle(t, driver)
	uxDailyUseIntegratedRequireState(t, view, uxDailyUseIntegratedCapture, "doing")
	footer := uxClarityFooter(driver)
	if !strings.Contains(footer, "No check-in") || !strings.Contains(footer, "Set up check-ins") {
		t.Fatalf("entering Doing left no note naming the unscheduled task and its route:\n%s", footer)
	}
	for _, interrupted := range []string{"Set Check-In", "Esc skips setup"} {
		if strings.Contains(driver.Observe(ObserveOptions{}).PlainScreen, interrupted) {
			t.Fatalf("entering Doing raised %q over the list", interrupted)
		}
	}
	if got := driver.App().CurrentView(); got != presentation.ViewCrossProjectOutliner {
		t.Fatalf("entering Doing left the view in %q", got)
	}

	// Schedule, through the keyless entry that note names. The note promises a
	// route, and this is the step that spends it: the entry raises the setup pass
	// over the list it was read from, which is the only thing entering Doing was
	// allowed not to do on its own.
	uxClarityPress(t, driver, UITestAction{Type: UITestActionCommand, Command: "Set up check-ins"})
	setup := driver.Observe(ObserveOptions{})
	if !strings.Contains(setup.PlainScreen, "Set up check-ins") {
		t.Fatalf("the check-in entry the footer names opened nothing\n%s", setup.PlainScreen)
	}
	if !strings.Contains(setup.PlainScreen, "Next check-in:") {
		t.Fatalf("the setup pass drew no schedule draft\n%s", setup.PlainScreen)
	}
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	uxDailyUseIntegratedSettle(t, driver)

	// And the same schedule reached by naming one task, which is the route a
	// user takes when the pass is not what they wanted. This is the commit: the
	// stored schedule is what makes scheduling explicit rather than merely
	// offered.
	seatCursorOnTaskTitled(t, view, uxDailyUseIntegratedCapture)
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionCommand, Command: "Set Check-In"},
		UITestAction{Type: UITestActionText, Text: "today"},
		UITestAction{Type: UITestActionKey, Key: "ctrl+s"},
		UITestAction{Type: UITestActionWait},
	)
	uxDailyUseIntegratedSettle(t, driver)
	if scheduled := uxDailyUseIntegratedStoredTask(t, view, uxDailyUseIntegratedCapture); scheduled.NextCheckIn == nil {
		t.Fatal("saving the named check-in editor stored no schedule")
	}

	// Edit, in the row rather than in a form. The inline editor takes its indent
	// and its budget from the same geometry the project column is measured in, so
	// an edit that commits is also evidence the two agree about where a row ends.
	seatCursorOnTaskTitled(t, view, uxDailyUseIntegratedCapture)
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "e"},
		UITestAction{Type: UITestActionText, Text: uxDailyUseIntegratedEditSuffix},
		UITestAction{Type: UITestActionKey, Key: "enter"},
		UITestAction{Type: UITestActionWait},
	)
	uxDailyUseIntegratedSettle(t, driver)
	edited := uxDailyUseIntegratedCapture + uxDailyUseIntegratedEditSuffix
	uxDailyUseIntegratedRequireStored(t, view, edited)

	// Complete. `wd` is the one route to Done, and completing clears planning,
	// so the task leaves the lane the cursor is standing in.
	seatCursorOnTaskTitled(t, view, edited)
	uxClarityPress(t, driver, uxClarityKeys("w", "d")...)
	uxDailyUseIntegratedSettle(t, driver)
	uxDailyUseIntegratedRequireState(t, view, edited, "done")

	// Undo. The ledger is one workspace ledger over the whole loop, so the step
	// it takes back is the completion and not the edit that preceded it.
	uxClarityPress(t, driver, uxClarityKeys("u")...)
	uxDailyUseIntegratedSettle(t, driver)
	uxDailyUseIntegratedRequireState(t, view, edited, "doing")
	uxDailyUseIntegratedRequireStored(t, view, edited)

	// And the frame the loop ends in is the frame it started in: a task row under
	// the cursor, the same six acts under it, and nothing clipped or lost.
	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)
	uxDailyUseIntegratedRequireTaskCursor(t, view, "the end of the loop")
	uxDailyUseIntegratedRequireLeadingActs(t, driver, "the end of the loop")
}

// TestUXDailyUseIntegratedWorkspaceGesturesFitEveryReviewedSize exercises the
// gestures a day across several projects is made of — searching the whole
// workspace, reading a subtree by depth, holding a selection whose members are
// folded away, editing tags over that selection, moving a task between lanes,
// and scheduling one explicitly — and requires each of them to fit at every
// reviewed size in both palettes.
//
// Each gesture is checked for the thing this plan changed about it rather than
// for the thing it has always done: a search still reads across projects, a
// child row names no project, a hidden selection is stated once,
// the picker leads with what the selection already carries, and a relocation
// leaves the cursor on work.
func TestUXDailyUseIntegratedWorkspaceGesturesFitEveryReviewedSize(t *testing.T) {
	gestures := []struct {
		name string
		run  func(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, size [2]int)
	}{
		{"search", uxDailyUseIntegratedSearchesAcrossProjects},
		{"hierarchy", uxDailyUseIntegratedReadsHierarchyByDepth},
		{"hidden-selection-and-tags", uxDailyUseIntegratedEditsTagsOverAHiddenSelection},
		{"relocation", uxDailyUseIntegratedRelocatesATaskAndKeepsTheCursor},
		{"scheduling", uxDailyUseIntegratedSchedulesExplicitly},
	}
	defer theme.ApplyTheme(theme.ThemeConfig{})
	for _, gesture := range gestures {
		for _, variant := range uxDailyUseReviewVariants {
			for _, size := range uxDailyUseMatrixSizes {
				t.Run(fmt.Sprintf("%s-%s-%s", gesture.name, variant, uxDailyUseSizeName(size)), func(t *testing.T) {
					// One gesture is one launch. Sharing a driver would make the
					// fifth assertion depend on what the fourth left folded, which
					// is a coupling the product does not have.
					driver, view := uxDailyUseDriver(t, FixtureUXAcceptance, size[0], size[1])
					theme.ApplyTheme(theme.ThemeConfig{Name: variant})
					gesture.run(t, driver, view, size)
				})
			}
		}
	}
}

// uxDailyUseIntegratedSearchesAcrossProjects runs one query over the whole
// workspace and reads the result for the fact the ranked list has to keep: it
// is drawn from more than one project and still fits the terminal.
func uxDailyUseIntegratedSearchesAcrossProjects(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	size [2]int,
) {
	t.Helper()
	uxClarityPress(t, driver,
		UITestAction{Type: UITestActionKey, Key: "/"},
		UITestAction{Type: UITestActionText, Text: uxClaritySearchQuery},
		UITestAction{Type: UITestActionKey, Key: "enter"},
	)
	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)

	projects := map[string]bool{}
	for _, row := range view.Rows {
		if row.IsTaskRow() && row.Task != nil {
			projects[row.ProjectID] = true
		}
	}
	if len(projects) < 2 {
		t.Fatalf("the ranked result draws rows from %d project(s), so it is not a cross-project read", len(projects))
	}
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
}

// uxDailyUseIntegratedReadsHierarchyByDepth reads the indent ladder in Backlog,
// where all three generations share one group. Depth is the one thing a compact
// row may not spend: every other field shrinks with the terminal and the
// connectors do not, and a row seated under its own parent states no project
// because the connectors already answered.
func uxDailyUseIntegratedReadsHierarchyByDepth(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	size [2]int,
) {
	t.Helper()
	uxSelectMode(t, driver, view, chrome.ModeBacklog)
	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)

	generations := []string{uxAcceptanceParentTitle, uxAcceptanceChildTitle, uxAcceptanceGrandchildTitle}
	indents := make([]int, 0, len(generations))
	for depth, generation := range generations {
		// The unique first word survives truncation at the minimum width; the
		// indent is what is under test.
		lead := strings.Fields(generation)[0]
		line, ok := uxLineContaining(obs.PlainScreen, lead)
		if !ok {
			t.Fatalf("Backlog draws no row for %q\n%s", lead, obs.PlainScreen)
		}
		indents = append(indents, ansi.StringWidth(line)-ansi.StringWidth(strings.TrimLeft(line, " │")))
		if depth > 0 && strings.Contains(line, "[ux-") {
			t.Fatalf("the row for %q names its project although it is drawn under its own parent: %q", lead, line)
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

// uxDailyUseIntegratedEditsTagsOverAHiddenSelection takes a range selection,
// folds it out of view, and then edits its tags — which is the one order in
// which both of this plan's selection changes are under test at once.
//
// The fold is what makes the summary a question: with the members drawn, the
// checkboxes already say what the summary says, and only a hidden member makes
// the count the sole evidence. The picker opened over that same selection is
// then the one state where a tag is neither applied nor unapplied, so the
// promoted block has to lead with what the selection carries rather than sort a
// mixed value back among the vocabulary nobody has.
func uxDailyUseIntegratedEditsTagsOverAHiddenSelection(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	size [2]int,
) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceOverdueTitle)
	uxClarityPress(t, driver, uxClarityKeys("v", "V", "j", "j", "s")...)
	counts := view.SelectionCounts()
	if counts.Hidden == 0 {
		t.Fatalf("folding the sections hid no selected task: %+v", counts)
	}

	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)
	summary := fmt.Sprintf("%d selected", counts.Total)
	if !strings.Contains(obs.PlainScreen, summary) {
		t.Fatalf("the persistent summary does not state %q at %s\n%s",
			summary, uxDailyUseSizeName(size), obs.PlainScreen)
	}
	hidden := fmt.Sprintf("%d hidden", counts.Hidden)
	if !strings.Contains(obs.PlainScreen, hidden) {
		t.Fatalf("the summary sheds %q, which is the half a bulk gesture would be surprised by\n%s",
			hidden, obs.PlainScreen)
	}
	// Counted once: the string appears on exactly one screen row, whatever the
	// width does to the rest of that row.
	if rows := uxDailyUseIntegratedRowsContaining(obs.PlainScreen, summary); rows != 1 {
		t.Fatalf("%q is stated on %d rows, want exactly one\n%s", summary, rows, obs.PlainScreen)
	}

	uxClarityPress(t, driver, uxClarityKeys("#")...)
	picker := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, picker, size)

	rows := uxDailyUsePickerRows(t, driver)
	if len(rows) == 0 {
		t.Fatalf("`#` over the hidden selection drew no picker rows\n%s", picker.PlainScreen)
	}
	var lead int
	for _, row := range rows {
		if row.mark != "[x]" && row.mark != "[-]" {
			break
		}
		lead++
	}
	if lead == 0 {
		t.Fatalf("no applied or partially applied value leads the picker:\n%s", uxDailyUsePickerOrder(rows))
	}
}

// uxDailyUseIntegratedRelocatesATaskAndKeepsTheCursor moves one task to another
// lane and requires the cursor to land on work rather than on the vacated index.
// The task leaves the block it was read in, so the recovery sweep is what
// decides where the next keystroke goes — and the next keystroke is the reason
// the plan cares.
func uxDailyUseIntegratedRelocatesATaskAndKeepsTheCursor(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	size [2]int,
) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceDueTodayTitle)
	uxClarityPress(t, driver, uxClarityKeys("w", "p")...)
	uxDailyUseIntegratedSettle(t, driver)

	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)
	uxDailyUseIntegratedRequireState(t, view, uxAcceptanceDueTodayTitle, "paused")
	uxDailyUseIntegratedRequireTaskCursor(t, view, "a relocation")

	uxClarityPress(t, driver, uxClarityKeys("u")...)
	uxDailyUseIntegratedSettle(t, driver)
	uxDailyUseIntegratedRequireState(t, view, uxAcceptanceDueTodayTitle, "backlog")
}

// uxDailyUseIntegratedSchedulesExplicitly commits a deadline through the one
// date picker. Scheduling is explicit everywhere in the finished product, so
// this is the counterpart the retired automatic editor owes: a route the user
// takes deliberately has to actually reach a stored date, at every width.
//
// The preset key is what makes it one gesture rather than a browse: it
// dispatches ahead of the grammar that also spells `t`, and it commits the row
// it lands on rather than merely seating a cursor there.
func uxDailyUseIntegratedSchedulesExplicitly(
	t *testing.T,
	driver *UITestDriver,
	view *outliner.OutlinerModel,
	size [2]int,
) {
	t.Helper()
	seatCursorOnTaskTitled(t, view, uxAcceptanceInboxTitle)
	uxClarityPress(t, driver, uxClarityKeys("@")...)
	obs := driver.Observe(ObserveOptions{})
	assertUXScreenFitsItsTerminal(t, obs, size)
	// DEFER sits below the due presets and scrolls off the smallest size, so the
	// combined picker is recognized by the crossing its footer offers instead.
	if !strings.Contains(obs.PlainScreen, "DUE") {
		t.Fatalf("`@` opened a picker with no DUE section\n%s", obs.PlainScreen)
	}
	if !strings.Contains(obs.PlainScreen, "DEFER") && !strings.Contains(obs.PlainScreen, "Tab switch field") {
		t.Fatalf("`@` opened a picker offering no Defer section, so it is not the one date picker\n%s",
			obs.PlainScreen)
	}

	uxClarityPress(t, driver, uxClarityKeys("t")...)
	uxDailyUseIntegratedSettle(t, driver)
	stored := uxDailyUseIntegratedStoredTask(t, view, uxAcceptanceInboxTitle)
	if stored.DueDate == nil {
		t.Fatalf("the Today preset stored no deadline\n%s", driver.Observe(ObserveOptions{}).PlainScreen)
	}

	uxClarityPress(t, driver, uxClarityKeys("u")...)
	uxDailyUseIntegratedSettle(t, driver)
	if restored := uxDailyUseIntegratedStoredTask(t, view, uxAcceptanceInboxTitle); restored.DueDate != nil {
		t.Fatal("undo left the deadline the preset stored")
	}
}
