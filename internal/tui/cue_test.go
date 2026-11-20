package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// doingProject is one project holding a doing row with an estimate and a
// doing row without one, both gone doing at the harness's noon.
func doingProject(t *testing.T) *harness {
	h := start(t).wide()
	h.press("o", `"Project"`, "enter", "tab", `"Write $doing #est=30m"`, "enter", `"Read $doing"`, "esc")
	return h
}

// rowLine is the drawn line holding text.
func (h *harness) rowLine(text string) string {
	h.t.Helper()
	for _, l := range strings.Split(h.rows(), "\n") {
		if strings.Contains(l, text) {
			return l
		}
	}
	h.t.Fatalf("no row %q in:\n%s", text, h.rows())
	return ""
}

func TestDoingRowSaysHowLong(t *testing.T) {
	h := doingProject(t)
	// Under a minute in hand, it says nothing yet.
	if got := h.rowLine("Write"); strings.Contains(got, "·") {
		t.Fatalf("a row just gone doing says %q", got)
	}
	// The minute turning is drawn without a key.
	h.clock = h.clock.Add(20 * time.Minute)
	h.tick()
	if got := h.rowLine("Write"); !strings.HasSuffix(got, "#est=30m · 20m") {
		t.Fatalf("doing row after 20 minutes = %q", got)
	}
	raw := h.m.View().Content
	if !strings.Contains(raw, darkPalette.dim+"· 20m") || strings.Contains(raw, darkPalette.warn+"· 20m") {
		t.Fatalf("an age within its estimate is not dim:\n%q", raw)
	}
	h.clock = h.clock.Add(time.Minute)
	h.tick()
	if got := h.rowLine("Write"); !strings.HasSuffix(got, "· 21m") {
		t.Fatalf("the minute turning did not redraw: %q", got)
	}
	// Past its estimate, the duration is in the warning colour; a row with
	// no estimate stays dim however long it runs.
	h.clock = h.clock.Add(79 * time.Minute)
	h.tick()
	if got := h.rowLine("Write"); !strings.HasSuffix(got, "· 1h40m") {
		t.Fatalf("doing row after 1h40m = %q", got)
	}
	raw = h.m.View().Content
	if !strings.Contains(raw, darkPalette.warn+"· 1h40m") {
		t.Fatalf("an age past its estimate is not in the warning colour:\n%q", raw)
	}
	if got := h.rowLine("Read"); !strings.HasSuffix(got, "Read · 1h40m") || strings.Count(raw, darkPalette.warn+"· 1h40m") != 1 {
		t.Fatalf("a doing row with no estimate = %q", got)
	}
	// In a filter and in a list it is drawn as in the outline.
	h.press("k", "t", "/", `"$doing"`, "enter")
	if got := h.rowLine("Read"); !strings.Contains(got, "· 1h40m") {
		t.Fatalf("a filter's doing row = %q", got)
	}
	h.press("esc", "f", `"Today"`, "enter")
	if got := h.rowLine("Write"); !strings.Contains(got, "· 1h40m") {
		t.Fatalf("Today's doing row = %q", got)
	}
	// On a later day it says since when, not 19 hours.
	h.clock = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	h.tick()
	if got := h.rowLine("Write"); !strings.Contains(got, "· since Nov 15") {
		t.Fatalf("a row doing since an earlier day = %q", got)
	}
	// A closed or parked row says nothing of it.
	h.press("f", `"Project"`, "enter")
	h.press("gg", "j", "x", "j", "$", `"parked"`, "enter")
	if got := h.rows(); strings.Contains(got, "since") || strings.Contains(got, "· 1h") {
		t.Fatalf("a closed or parked row says how long:\n%s", got)
	}
}

func TestClosingSaysWhatItTook(t *testing.T) {
	h := doingProject(t)
	h.clock = h.clock.Add(100 * time.Minute)
	h.tick()
	// With an estimate, beside it; before the day's tally.
	h.press("k", "x")
	h.wantStatus("done in 1h40m · est 30m · 1 done today")
	// Undo says nothing of it.
	h.press("u")
	if got := h.status(); strings.Contains(got, "done in") {
		t.Fatalf("undo said %q", got)
	}
	// Without one, only the duration.
	h.press("j", "x")
	h.wantStatus("done in 1h40m · 1 done today")
	h.press("u")

	// A selection says only the tally.
	h.press("gg", "j", "V", "j", "x")
	if got := h.status(); !strings.HasPrefix(got, "2 done today ") {
		t.Fatalf("closing a selection said %q", got)
	}
	h.press("esc", "u")

	// A row gone doing on an earlier day says since when.
	h.clock = time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	h.tick()
	h.press("gg", "j", "x")
	h.wantStatus("done · doing since Nov 15 · 1 done today")
	// A row that was not doing says only the tally.
	h.press("gg", "o", `"Fresh"`, "esc", "x")
	h.wantStatus("2 done today")
}

// doneOn pastes n rows already done, as rows closed daysAgo days before
// the harness's day: a pasted row's changed is when it was made.
func (h *harness) doneOn(daysAgo, n int) {
	h.t.Helper()
	now := h.clock
	h.clock = now.AddDate(0, 0, -daysAgo)
	var rows []string
	for i := range n {
		rows = append(rows, fmt.Sprintf("Done %d-%d $done", daysAgo, i))
	}
	h.pasteText(strings.Join(rows, "\n"))
	h.clock = now
	h.tick()
}

func TestTodayNamesTheUsualDay(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Pick 1 $open\nPick 2 $open\nPick 3 $open\nPick 4 $open\nPick 5 $open")
	// Four days in the 28 before today did anything; a day before them
	// is out of the reckoning, so nothing is said yet.
	h.doneOn(29, 5)
	h.doneOn(1, 2)
	h.doneOn(2, 2)
	h.doneOn(3, 4)
	h.doneOn(4, 4)
	h.press("gg", "t", "j", "t", "f", `"Today"`, "enter")
	if got := h.header(); got != "Today  · 2 to do" {
		t.Fatalf("too little history: %q", got)
	}
	// A fifth day: the median of 1, 2, 2, 4, 4.
	h.press("f", `"Projects"`, "enter")
	h.doneOn(28, 1)
	h.press("f", `"Today"`, "enter")
	if got := h.header(); got != "Today  · 2 to do · usually 2 a day" {
		t.Fatalf("five days: %q", got)
	}
	if raw := h.raw(); !strings.Contains(raw, darkPalette.dim+"usually 2 a day") {
		t.Fatalf("a day within the usual is not dim:\n%q", raw)
	}
	// More picked than a day usually holds is said in the warning colour.
	h.press("f", `"Projects"`, "enter", "gg", "j", "j", "j", "t", "j", "t", "j", "t", "f", `"Today"`, "enter")
	if got := h.header(); got != "Today  · 5 to do · usually 2 a day" {
		t.Fatalf("over: %q", got)
	}
	if raw := h.raw(); !strings.Contains(raw, darkPalette.warn+"usually 2 a day") {
		t.Fatalf("a day over the usual is not in the warning colour:\n%q", raw)
	}
	// Closing picks brings it back under.
	h.press("gg", "x", "j", "x", "j", "x")
	if got := h.header(); got != "Today  · 2 to do · 3 done · usually 2 a day" {
		t.Fatalf("under again: %q", got)
	}
	if raw := h.raw(); !strings.Contains(raw, darkPalette.dim+"usually 2 a day") {
		t.Fatalf("a day back under the usual is not dim:\n%q", raw)
	}
	// The day turning reads today's three in, and the oldest day out.
	h.clock = h.clock.AddDate(0, 0, 1)
	h.tick()
	h.press("f", `"Today"`, "enter")
	if got := h.header(); got != "Today  · 2 to do · usually 3 a day" {
		t.Fatalf("the next day: %q", got)
	}
}

func TestSecondDoingRowIsCounted(t *testing.T) {
	h := project(t).wide()
	h.press("o", `"Third"`, "esc")
	// One row doing says nothing of it.
	h.press("gg", "j", "$", `"doing"`, "enter")
	if got := h.status(); strings.Contains(got, "rows doing") {
		t.Fatalf("one row doing said %q", got)
	}
	// A second one says how many are.
	h.press("j", "$", `"doing"`, "enter")
	h.wantStatus("2 rows doing: /$doing lists them")
	if got := h.rows(); !strings.Contains(got, "Second") || !strings.Contains(got, "Third") {
		t.Fatalf("a row moved:\n%s", got)
	}
	// Undo says only what it undid.
	h.press("u")
	if got := h.status(); strings.Contains(got, "rows doing") {
		t.Fatalf("undo said %q", got)
	}
	// After a left-off note, and from an edited line too.
	h.press("R", `"Next: the tests"`, "enter", "$", `"doing"`, "enter")
	h.wantStatus("left off: Next: the tests · 2 rows doing: /$doing lists them")
	h.press("j", "A", `" $doing"`, "esc")
	h.wantStatus("3 rows doing: /$doing lists them")
	// A change that sets no row doing says nothing of the rows in hand.
	h.press("k", "#", `"home"`, "enter")
	if got := h.status(); strings.Contains(got, "rows doing") {
		t.Fatalf("a tag said %q", got)
	}
}

func TestCarriedPickNamesASmallerStep(t *testing.T) {
	h := start(t).wide()
	day := h.clock
	h.clock = day.AddDate(0, 0, -3)
	h.press("o", `"Project"`, "enter", "tab", `"Big"`, "enter", `"Split"`, "enter", "tab", `"A step $open"`, "esc")
	h.press("gg", "j", "t", "j", "t")
	// Carried two days, it says nothing.
	h.clock = day.AddDate(0, 0, -1)
	h.tick()
	h.press("f", `"Today"`, "enter", "gg")
	if h.on() != "Big" || h.status() != "" {
		t.Fatalf("two days on %q said %q", h.on(), h.status())
	}
	// Three days, it names a smaller first step.
	h.clock = day
	h.tick()
	h.press("f", `"Today"`, "enter", "gg")
	h.wantStatus("picked 3 days ago: Enter, then o writes a smaller first step")
	// A pick with a task under it has its first step.
	h.press("j")
	if h.on() != "Split" || h.status() != "" {
		t.Fatalf("a pick with a task under it, %q, said %q", h.on(), h.status())
	}
	// Out of Today, it says nothing.
	h.press("f", `"Projects"`, "enter", "gg", "j")
	if h.on() != "Big" || h.status() != "" {
		t.Fatalf("the outline's %q said %q", h.on(), h.status())
	}
	// Where its work was left off is said first.
	h.press("R", `"Next: call them"`, "enter", "f", `"Today"`, "enter", "gg")
	h.wantStatus("left off: Next: call them")
}
