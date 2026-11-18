package tui

import "testing"

// Quitting leaves one line saying what the day finished and what Today
// still holds, and nothing on a day with neither.
func TestFarewell(t *testing.T) {
	h := start(t).wide()
	if got := h.m.Farewell(); got != "" {
		t.Fatalf("an untouched outline says %q", got)
	}
	h.pasteText("Project\n\tWrite $open\n\tRead $open\n\tCall $open")
	h.press("g", "g", "j", "t", "j", "t", "j", "x")
	if got := h.m.Farewell(); got != "1 done today · 2 still in Today" {
		t.Fatalf("farewell = %q", got)
	}
	h.press("k", "x", "k", "x")
	if got := h.m.Farewell(); got != "3 done today" {
		t.Fatalf("farewell with Today closed = %q", got)
	}
	h.press("d", "d")
	if got := h.m.Farewell(); got != "2 done today" {
		t.Fatalf("a deleted row is still counted: %q", got)
	}
}

// An empty Today names the saved /next as well as t, since what could be
// done is where a day's picks come from; without /next saved it names t alone.
func TestEmptyTodayNamesNext(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tWrite $open")
	h.press("f", `"Today"`, "enter")
	h.want("  Nothing picked for today. Press t on a row to pick it.")
	h.press("f", `"Project"`, "enter", "/", `"$next"`, "enter", ":", `"save next"`, "enter", "esc")
	h.press("f", `"Today"`, "enter")
	h.want("  Nothing picked for today. t picks a row; /next lists what can be done.")
}
