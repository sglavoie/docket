package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// lit turns the light on finished rows on for one test.
func lit(t *testing.T) {
	closeFlashFor = time.Millisecond
	t.Cleanup(func() { closeFlashFor = 0 })
}

func TestFinishingARowLightsIt(t *testing.T) {
	lit(t)
	h := project(t).wide()
	flash := h.m.colors().flash
	if strings.Contains(h.raw(), flash) {
		t.Fatal("a row is lit before anything is finished")
	}
	h.press("x")
	if !h.m.flashed[h.m.current()] || !strings.Contains(h.raw(), flash) {
		t.Fatalf("a row closed as done is not lit:\n%q", h.raw())
	}
	// The light goes out on its own.
	h.m.Update(flashMsg{h.m.flashGen})
	if strings.Contains(h.raw(), flash) {
		t.Fatal("the light stayed on")
	}
	// Undo, reopening and canceling light nothing.
	h.press("u")
	if len(h.m.flashed) > 0 {
		t.Fatal("an undo lit a row")
	}
	h.press("$", `"canceled"`, "enter")
	if len(h.m.flashed) > 0 {
		t.Fatal("canceling lit a row")
	}
	// A selection finished at once lights every row of it.
	h.press("$", `"open"`, "enter", "gg", "j", "v", "v", "x")
	if len(h.m.flashed) != 2 {
		t.Fatalf("lit %d rows of two", len(h.m.flashed))
	}
}

func TestAFinishedDayIsSaidInTheAccent(t *testing.T) {
	h := project(t).wide()
	h.press("gg", "j", "t", "f", `"Today"`, "enter")
	h.press("x")
	if got := h.header(); got != "Today  · all 1 done" {
		t.Fatalf("header = %q", got)
	}
	if raw := h.raw(); !strings.Contains(raw, darkPalette.accent+"all 1 done") {
		t.Fatalf("a finished day is not in the accent:\n%q", raw)
	}
	h.press("x")
	if raw := h.raw(); strings.Contains(raw, darkPalette.accent+"1 to do") {
		t.Fatalf("a day still to do is in the accent:\n%q", raw)
	}
}

func TestTodayDrawsTheWeek(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Pick $open")
	h.doneOn(1, 4)
	h.doneOn(2, 2)
	h.doneOn(4, 1)
	h.doneOn(10, 1)
	h.press("gg", "t", "f", `"Today"`, "enter")
	// Four days in the reckoning are too few for a usual day, so the week
	// is not drawn either.
	if got := h.header(); strings.Contains(got, "7d") {
		t.Fatalf("too little history: %q", got)
	}
	h.press("f", `"Projects"`, "enter")
	h.doneOn(20, 1)
	h.press("f", `"Today"`, "enter")
	// Six days ago, five, four, three, two, yesterday, today: a blank is a
	// day that finished nothing, and the busiest day is the full bar.
	if got := h.header(); !strings.HasSuffix(got, "usually 1 a day · 7d   ▂ ▄█") {
		t.Fatalf("the week: %q", got)
	}
	// Today's bar grows as the day is finished, in the text colour.
	h.press("x")
	if got := h.header(); !strings.HasSuffix(got, "7d   ▂ ▄█▂") {
		t.Fatalf("today's bar: %q", got)
	}
	if raw := h.raw(); !strings.Contains(raw, darkPalette.dim+"7d   ▂ ▄█"+sgrReset) {
		t.Fatalf("today's bar is not in the text colour:\n%q", raw)
	}
	// A narrow line gives the week up before anything else.
	h.m.Update(tea.WindowSizeMsg{Width: 76, Height: 16})
	if got := h.header(); strings.Contains(got, "7d") || !strings.Contains(got, "usually 1 a day") {
		t.Fatalf("narrow: %q", got)
	}
}

func TestComingBackReadsTheWorkInHand(t *testing.T) {
	h := project(t).wide()
	h.m.Update(tea.WindowSizeMsg{Width: 160, Height: 16})
	away := func(d time.Duration) string {
		h.m.Update(tea.BlurMsg{})
		h.clock = h.clock.Add(d)
		h.m.Update(tea.FocusMsg{})
		return h.status()
	}
	// Nothing in hand names where to start.
	if got := away(25 * time.Minute); got != "back after 25m · start here: First · /suggest lists more" {
		t.Fatalf("nothing doing: %q", got)
	}
	h.press("$", `"doing"`, "enter", "R", `"Next: the tests"`, "enter")
	// A glance away says nothing.
	if got := away(5 * time.Minute); strings.Contains(got, "back after") {
		t.Fatalf("five minutes away said %q", got)
	}
	if got := away(2*time.Hour + 10*time.Minute); got != "back after 2h10m · doing: Second · left off: Next: the tests" {
		t.Fatalf("one doing: %q", got)
	}
	// The next key takes it away.
	h.press("j")
	if got := h.status(); strings.Contains(got, "back after") {
		t.Fatalf("the read-back outlived a key: %q", got)
	}
	h.press("gg", "j", "$", `"doing"`, "enter")
	if got := away(26 * time.Hour); got != "back after 1d · 2 rows doing: /$doing lists them" {
		t.Fatalf("two doing: %q", got)
	}
	// Focus with no blur before it, as at launch, says nothing.
	h.press("j")
	h.m.Update(tea.FocusMsg{})
	if got := h.status(); strings.Contains(got, "back after") {
		t.Fatalf("a focus with no blur said %q", got)
	}
}

func TestStartHereNamesOneRow(t *testing.T) {
	h := start(t).wide()
	h.m.Update(tea.WindowSizeMsg{Width: 160, Height: 16})
	h.pasteText("Project\n\tLater $open\n\tAsked $waiting @+1d\n\tPicked $open\n\tTax report $open @+2d")
	// The soonest near delivery still open comes first; one waiting on
	// someone else is passed over.
	if got := h.m.startHere(false); got != "start here: Tax report, due Tue" {
		t.Fatalf("a delivery: %q", got)
	}
	h.press("G", "x")
	h.press("k", "t")
	if got := h.m.startHere(false); got != "start here: Picked, first in Today" {
		t.Fatalf("a pick: %q", got)
	}
	h.press("x")
	if got := h.m.startHere(false); got != "start here: Later · /suggest lists more" {
		t.Fatalf("a suggestion: %q", got)
	}
	h.press("gg", "j", "x")
	if got := h.m.startHere(false); got != "" {
		t.Fatalf("nothing open named %q", got)
	}
}

func TestLaunchAfterDaysAwaySaysWhatSlipped(t *testing.T) {
	h := project(t).wide()
	launch := func() string {
		tree, err := h.st.Load()
		if err != nil {
			t.Fatal(err)
		}
		h.m = Launch(tree, h.st, nil, h.now)
		h.m.Update(tea.WindowSizeMsg{Width: 180, Height: 16})
		return h.status()
	}
	h.press("gg", "j", "t")
	if got := launch(); got != "start here: First, first in Today" {
		t.Fatalf("first launch: %q", got)
	}
	// The next morning is not time away.
	h.clock = h.clock.AddDate(0, 0, 1)
	if got := launch(); strings.Contains(got, "away") {
		t.Fatalf("the next day said %q", got)
	}
	h.clock = h.clock.AddDate(0, 0, 6)
	if got := launch(); got != "away 6 days · 1 pick carried · 1 slipping: /slipping lists them · start here: First, first in Today" {
		t.Fatalf("after six days: %q", got)
	}
	// Opened again the same day, it has been seen.
	if got := launch(); strings.Contains(got, "away") {
		t.Fatalf("a second launch said %q", got)
	}
	// A session left open notes each day it is open.
	h.clock = h.clock.AddDate(0, 0, 3)
	h.tick()
	if got := launch(); strings.Contains(got, "away") {
		t.Fatalf("a session open through the days said %q", got)
	}
}

func TestStaleNamesTheWayToSetItDown(t *testing.T) {
	h := project(t).wide()
	h.clock = h.clock.AddDate(0, 0, 15)
	h.tick()
	h.press("f", `"Stale"`, "enter")
	h.wantStatus(staleHint)
	h.press(":", `"all"`, "enter", "$", `"p"`, "enter")
	if got := h.rows(); strings.Count(got, "◌") != 2 {
		t.Fatalf("not every row was parked:\n%s", got)
	}
	h.press("esc", "u")
	if got := h.rows(); strings.Count(got, "◌") != 0 {
		t.Fatalf("u did not bring them back:\n%s", got)
	}
	// Anything the row has to say comes first.
	h.press("gg", "R", `"Next: call"`, "enter", "j", "k")
	if got := h.status(); !strings.HasPrefix(got, "left off: Next: call") {
		t.Fatalf("status = %q", got)
	}
}
