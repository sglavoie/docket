package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The harness's day is Sunday, November 15: a deadline through Wednesday
// is near, and one on Friday is not.

func TestCheckNoteIsWrittenAndReadBack(t *testing.T) {
	h := project(t).wide()
	h.press("C", `"totals,   names"`)
	h.wantStatus("Check › totals,   names   Enter keeps it · a blank line takes it off · Esc leaves it")
	h.press("enter")
	h.wantStatus("noted what to check before it is delivered")
	if got := h.m.tree.Row(h.m.current()).Fields[fieldCheck]; got != "totals, names" {
		t.Fatalf("check = %q", got)
	}
	// It is not drawn on the row, and a row with no near deadline says nothing.
	h.want("  ▾ Project", "    ○ First", "▌   ○ Second")
	h.press("k", "j")
	h.wantStatus("")
	// With its deadline near it is read back whenever the cursor is on it.
	h.press("@", `"fri"`, "enter", "k", "j")
	h.wantStatus("")
	h.press("@", `"wed"`, "enter", "k", "j")
	h.wantStatus("check: totals, names · due Wed, first step not written: Enter, then o")
	// Taking the row up says it once, after any left-off note.
	h.press("R", `"Next: the totals"`, "enter", "$", `"doing"`, "enter")
	h.wantStatus("left off: Next: the totals · check: totals, names")
	// Esc leaves it, a changed line changes it, a blank line takes it off.
	h.press("C", `" more"`, "esc")
	h.press("C", `", dates"`, "enter")
	h.wantStatus("check changed")
	h.press("C", "ctrl+u", "enter")
	h.wantStatus("check taken off")
	h.press("u", "k", "j")
	h.wantStatus("left off: Next: the totals · check: totals, names, dates")
	// It is one row's, so a selection refuses it.
	h.press("v", "C")
	if got := h.status(); !strings.HasPrefix(got, "that works on one row: press Esc to clear the selection") {
		t.Fatalf("a selection said %q", got)
	}
}

func TestNearDeliverySaysWhatItLacks(t *testing.T) {
	h := project(t).wide()
	// No check and no first step, the day said once.
	h.press("@", `"tom"`, "enter", "k", "j")
	h.wantStatus("due tomorrow, no check written: C writes one · first step not written: Enter, then o")
	// A row in hand has its first step under way.
	h.press("$", `"doing"`, "enter", "k", "j")
	h.wantStatus("due tomorrow, no check written: C writes one")
	// A task under it is its first step; one past its day is due today.
	h.press("$", `"open"`, "enter", "@", `"2026-11-01"`, "enter", "o", "tab", `"A step"`, "esc", "k")
	h.wantStatus("due today, no check written: C writes one")
	// A closed row is no delivery, and nor is a note.
	h.press("x", "j", "k")
	h.wantStatus("")
}

func TestTaskTooBigNamesASmallerStep(t *testing.T) {
	h := project(t).wide()
	h.press("A", `" #est=4h"`, "esc", "k", "A", `" #est=2h"`, "esc", "j")
	// Neither picked nor in hand, it says nothing.
	h.wantStatus("")
	h.press("t", "k", "t", "j", "k", "j")
	h.wantStatus("est 4h and no step under it: Enter, then o writes a smaller first step")
	// Two hours is a block.
	h.press("k")
	h.wantStatus("")
	// A row in hand is told the same, picked or not.
	h.press("j", "t", "$", `"doing"`, "enter", "k", "j")
	h.wantStatus("est 4h and no step under it: Enter, then o writes a smaller first step")
	// With a task under it, it has its first step.
	h.press("o", "tab", `"A step"`, "esc", "k")
	h.wantStatus("")
}

func TestTodayCountsItsUnsizedPicks(t *testing.T) {
	h := project(t).wide()
	h.press("o", `"Third #est=1h"`, "enter", `"Fourth"`, "enter", "tab", `"A step"`, "esc")
	h.press("gg", "j", "V", "G", "t", "f", `"Today"`, "enter")
	// First and Second have neither an estimate nor a step; the step has none.
	if got := h.header(); got != "Today  · 5 to do · 1h · 3 unsized" {
		t.Fatalf("header = %q", got)
	}
	// A closed pick is not counted, and an estimate sizes one.
	h.press("gg", "x", "j", "#", `"est=30m"`, "enter")
	if got := h.header(); got != "Today  · 4 to do · 1h30m · 1 done · 1 unsized" {
		t.Fatalf("header = %q", got)
	}
}

// deliveries is a project of three tasks, two of them due by Wednesday.
func deliveries(t *testing.T) *harness {
	h := start(t).wide()
	h.press("o", `"Project"`, "enter", "tab", `"Tax report @wed"`, "enter", `"Slides @tue"`, "enter", `"Far away @fri"`, "esc")
	return h
}

func TestTodayNamesCompetingDeliveries(t *testing.T) {
	h := deliveries(t)
	h.press("t", "f", `"Today"`, "enter")
	if got := h.header(); got != "Today  · 1 to do · 1 unsized · 2 due by Wed · first: Slides" {
		t.Fatalf("header = %q", got)
	}
	// It is the first thing a narrow line gives up.
	h.m.Update(tea.WindowSizeMsg{Width: 76, Height: 16})
	if got := h.header(); got != "Today  · 1 to do · 1 unsized" {
		t.Fatalf("narrow header = %q", got)
	}
	h.wide()
	// One delivery competes with nothing; a parked one is not to be ordered.
	h.press("f", `"Projects"`, "enter", "gg", "j", "$", `"parked"`, "enter", "f", `"Today"`, "enter")
	if got := h.header(); got != "Today  · 1 to do · 1 unsized" {
		t.Fatalf("one delivery left: %q", got)
	}
	// Nowhere else is it said.
	h.press("f", `"Projects"`, "enter", "u")
	if got := h.header(); strings.Contains(got, "due by") {
		t.Fatalf("Projects says %q", got)
	}
}

func TestStampingADeadlineSaysWhatItLandsAmong(t *testing.T) {
	h := deliveries(t)
	// A day past the near ones lands among nothing.
	h.press("@", `"sat"`, "enter")
	if got := h.status(); strings.Contains(got, "due by") {
		t.Fatalf("a far deadline said %q", got)
	}
	h.press("@", `"mon"`, "enter")
	h.wantStatus("2 others due by Wed: /deliveries lists them")
	// The same day again changes nothing, and undo says only what it undid.
	h.press("u")
	if got := h.status(); strings.Contains(got, "due by") {
		t.Fatalf("undo said %q", got)
	}
	// Typed on the row, it is said too.
	h.press("o", `"Fourth @tom"`, "esc")
	h.wantStatus("2 others due by Wed: /deliveries lists them")
	// The built-in filter lists them, soonest first.
	h.press("F", `"deliveries"`, "enter")
	h.want(
		"▌ ○ Fourth @tomorrow · Project",
		"  ○ Slides @tue · Project",
		"  ○ Tax report @wed · Project",
	)
}

func TestWorkPutDownWithNoNoteIsNamed(t *testing.T) {
	h := project(t).wide()
	h.press("o", `"Third"`, "esc")
	h.press("gg", "j", "$", `"doing"`, "enter", "j", "$", `"doing"`, "enter")
	h.wantStatus("2 rows doing · First has no left-off note: R writes one")
	// The quit line names what is left doing with nothing noted.
	if got := h.m.Farewell(); got != "2 left doing without a note" {
		t.Fatalf("farewell = %q", got)
	}
	h.press("j", "$", `"doing"`, "enter")
	h.wantStatus("3 rows doing: /$doing lists them · 2 have no left-off note: R writes one")
	// A row put down to wait or for later says it has no note.
	h.press("$", `"waiting"`, "enter")
	h.wantStatus("put down with no left-off note: R writes one")
	h.press("k", "R", `"Next: the tests"`, "enter", "$", `"open"`, "enter")
	if got := h.status(); strings.Contains(got, "left-off") {
		t.Fatalf("a row with a note said %q", got)
	}
	// Finishing a row puts nothing down.
	h.press("k", "x")
	if got := h.status(); strings.Contains(got, "left-off") {
		t.Fatalf("a finished row said %q", got)
	}
	if got := h.m.Farewell(); got != "1 done today" {
		t.Fatalf("farewell = %q", got)
	}
	h.press("u")
	if got := h.m.Farewell(); got != "First left doing without a note" {
		t.Fatalf("farewell = %q", got)
	}
}

func TestLaunchSaysWhatWasLeftDoing(t *testing.T) {
	h := project(t).wide()
	launch := func() string {
		tree, err := h.st.Load()
		if err != nil {
			t.Fatal(err)
		}
		h.m = Launch(tree, h.st, nil, h.now)
		h.wide()
		return h.status()
	}
	// Nothing in hand names where to start instead.
	if got := launch(); got != "start here: First · /suggest lists more" {
		t.Fatalf("nothing doing said %q", got)
	}
	h.press("G", "$", `"doing"`, "enter")
	if got := launch(); got != "left doing: Second" {
		t.Fatalf("launch said %q", got)
	}
	h.press("G", "R", `"Next: the tests"`, "enter")
	if got := launch(); got != "left doing: Second · left off: Next: the tests" {
		t.Fatalf("launch said %q", got)
	}
	h.press("gg", "j", "$", `"doing"`, "enter")
	if got := launch(); got != "2 rows left doing: /$doing lists them" {
		t.Fatalf("launch said %q", got)
	}
}

func TestUncheckedFindsDeliveriesWithNoCheck(t *testing.T) {
	h := deliveries(t)
	h.press("gg", "j", "C", `"totals"`, "enter", "j", "x")
	// Tax report has a check, Slides is done: only Far away is left.
	h.press("/", `"$unchecked"`)
	if got := h.status(); !strings.Contains(got, "1 match · in context · a task with a deadline and no check written") {
		t.Fatalf("read back = %q", got)
	}
	h.press("ctrl+t", "enter")
	h.want("▌ ○ Far away @fri · Project")
	if got := List(h.m.tree, h.st, "$unchecked", h.now); len(got) != 1 || !strings.HasPrefix(got[0], "Far away") {
		t.Fatalf("docket ls = %q", got)
	}
}
