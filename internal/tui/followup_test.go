package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/sigil"
)

// A waiting row that repeats, with a start day and no deadline, is a
// follow-up: from its start it comes back to Today every interval while it
// waits, nothing is written as the days pass, and closing it closes it for
// good rather than moving it on.
func TestWaitingFollowUpComesBackEveryInterval(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tChase Ann $waiting *3d @@2026-11-16\n\tPay rent $waiting *1m @2026-11-16 @@2026-11-16")
	chase, rent := "", ""
	project := h.m.tree.Children("")[len(h.m.tree.Children(""))-1]
	for _, id := range h.m.tree.Children(project) {
		switch sigil.FromFields(h.m.tree.Row(id).Fields).Text {
		case "Chase Ann":
			chase = id
		case "Pay rent":
			rent = id
		}
	}
	if chase == "" || rent == "" {
		t.Fatal("rows not pasted")
	}
	gen := h.m.tree.Gen()
	inToday := func(day int) (bool, bool) {
		h.clock = time.Date(2026, 11, day, 12, 0, 0, 0, time.UTC)
		h.m.scan()
		return h.m.inToday[chase], h.m.inToday[rent]
	}
	for day, want := range map[int]bool{15: false, 16: true, 17: false, 18: false, 19: true, 20: false, 22: true, 25: true} {
		follow, deadline := inToday(day)
		if follow != want {
			t.Errorf("Nov %d: follow-up in Today = %v, want %v", day, follow, want)
		}
		// A row with a deadline is a repeating row that waits: its start is
		// the day it names.
		if deadline != (day == 16) {
			t.Errorf("Nov %d: waiting row with a deadline in Today = %v", day, deadline)
		}
	}
	if h.m.tree.Gen() != gen {
		t.Fatal("the days passing wrote to the outline")
	}
	// Between follow-ups the row names the next one.
	inToday(20)
	if got := sigil.FromFields(h.m.tree.Row(chase).Fields).StartOn(h.m.today()); got != "2026-11-22" {
		t.Fatalf("next follow-up = %s", got)
	}
	// The answer came: closing it closes it, with its start kept as typed.
	inToday(22)
	h.press("f", `"Chase Ann"`, "enter", "x")
	f := h.m.tree.Row(chase).Fields
	if f[sigil.FieldState] != sigil.Done || f[sigil.FieldStart] != "2026-11-16" {
		t.Fatalf("closing a follow-up left state %q, start %q", f[sigil.FieldState], f[sigil.FieldStart])
	}
	// A repeating row that waits with a deadline still moves on when closed.
	h.press("f", `"Pay rent"`, "enter", "x")
	f = h.m.tree.Row(rent).Fields
	if f[sigil.FieldState] != sigil.Open || f[sigil.FieldDue] != "2026-12-16" {
		t.Fatalf("a repeating waiting row with a deadline left state %q, due %q", f[sigil.FieldState], f[sigil.FieldDue])
	}
}

// The filter reads a follow-up's start as it stands, so @@ and $today agree
// with Today.
func TestFollowUpFilterReadsStartAsItStands(t *testing.T) {
	l := sigil.Line{State: sigil.Waiting, Every: "1w", Start: "2026-11-02"}
	today := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	if got := l.StartOn(today); got != "2026-11-16" {
		t.Fatalf("StartOn = %s", got)
	}
	l.Start = "2026-11-08"
	if got := l.StartOn(today); got != "2026-11-15" {
		t.Fatalf("StartOn on the day = %s", got)
	}
	l.State = sigil.Open
	if got := l.StartOn(today); got != "2026-11-08" {
		t.Fatalf("an open row's start moved: %s", got)
	}
}
