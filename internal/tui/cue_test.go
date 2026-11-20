package tui

import (
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
