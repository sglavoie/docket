package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAStartDateDimsARowUntilItsDayThenPutsItInToday(t *testing.T) {
	h := project(t).wide()
	h.press("o", `"Renew passport @@fri"`, "enter", `"Far off @@+60d"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ○ Renew passport @@fri",
		"▌   ○ Far off @@Jan 14 2027",
	)
	if !strings.Contains(h.m.View().Content, darkPalette.dim+"Renew"+sgrReset) {
		t.Fatal("a row that cannot start yet is not dimmed")
	}
	h.press("/", `"@@later"`, "tab")
	h.want(
		"▌ ○ Renew passport @@fri · Project",
		"  ○ Far off @@Jan 14 2027 · Project",
	)
	h.press("esc")

	// On the day, with no key pressed, it is in Today and reads as live. Today
	// names the day already, so the row does not say it again.
	h.clock = h.clock.Add(5 * 24 * time.Hour)
	h.tick()
	h.wantSide(
		"Inbox",
		"Today 1",
		"Waiting",
		"Stale",
		"▌ Projects",
		"Project 5d",
	)
	if strings.Contains(h.m.View().Content, darkPalette.dim+"Renew"+sgrReset) {
		t.Fatal("the row is still dimmed on its start day")
	}
	h.press("f", `"today"`, "enter")
	h.want("▌ ○ Renew passport · Project")

	// Weeks on, the untouched rows are stale; the one still waiting for its
	// day is not.
	h.clock = h.clock.Add(20 * 24 * time.Hour)
	h.press("f", `"stale"`, "enter")
	h.want(
		h.edge("▌ ○ First · Project", "25d"),
		h.edge("  ○ Second · Project", "25d"),
		h.edge("  ○ Renew passport @@Nov 20 · Project", "25d"),
	)
	h.press("-", `"@@"`, "enter", "A")
	h.want(
		"▌ ○ First",
		h.edge("  ○ Second · Project", "25d"),
		h.edge("  ○ Renew passport @@Nov 20 · Project", "25d"),
	)
}

func TestARepeatingRowIsMovedOnNotClosed(t *testing.T) {
	h := project(t).wide()
	h.press("o", `"Water plants *3d"`, "enter", `"Pay rent @12-01 @@11-28 *monthly"`, "enter", `"Backup *weekly $doing"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ○ Water plants *3d",
		"    ○ Pay rent @Dec 1 @@Nov 28 *1m",
		"▌   ● Backup *1w",
	)
	// Closing each moves it to its next day: the deadline when it has one,
	// else the day it comes back; and it is open again.
	h.press("x")
	if h.m.msg != "repeats: next Nov 22" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("k", "x", "k", "$", `"done"`, "enter")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ○ Water plants @@wed *3d",
		"    ○ Pay rent @Jan 1 2027 @@Dec 28 *1m",
		"    ○ Backup @@Nov 22 *1w",
	)
	h.press("u", "u", "u")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ○ Water plants *3d",
		"    ○ Pay rent @Dec 1 @@Nov 28 *1m",
		"▌   ● Backup *1w",
	)

	// Stamps set and clear the interval, and the filter finds it.
	h.press("-", `"*"`, "enter", "x", "k", "k", "*", `"2w"`, "enter", "/", `"*"`, "tab")
	h.want(
		"▌ ○ Water plants *2w · Project",
		"  ○ Pay rent @Dec 1 @@Nov 28 *1m · Project",
	)
}

func TestAFilterCanBeSavedAndReopened(t *testing.T) {
	h := work(t)
	h.press(":", `"save bugs"`, "enter")
	if h.m.msg != "filter an outline first (/), then save it" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("/", `"#bug -$done"`, "tab", "enter", ":", `"save open bugs"`, "enter")
	if h.m.msg != "saved: /open bugs is in the sidebar, and F opens it by name" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("esc", "x", "F", `"/op"`)
	if h.bottom() != "Filters › /op   /open bugs" {
		t.Fatalf("picker = %q", h.bottom())
	}
	h.press("enter")
	if h.header() != "Projects  · 2 to do  / #bug -$done  (flat)" {
		t.Fatalf("header = %q", h.header())
	}
	h.want(
		"▌ ○ Fix login ! #bug · Alpha",
		"  ○ Refactor auth #bug · Alpha",
	)

	// It outlives the session, and can be forgotten.
	again := reopen(t, h)
	again.press("F", `"/open"`, "enter")
	again.want(
		"▌ ○ Fix login ! #bug · Alpha",
		"  ○ Refactor auth #bug · Alpha",
	)
	again.press(":", `"forget open bugs"`, "enter", "F", `"/open"`, "enter")
	if again.m.msg != "nothing matches /open" {
		t.Fatalf("msg = %q", again.m.msg)
	}
}

func TestReviewListsProjectsLongestUntouchedFirst(t *testing.T) {
	h := two(t)
	h.clock = h.clock.Add(10 * 24 * time.Hour)
	h.press("A", `" v2"`, "esc") // Beta is touched; Alpha is not
	h.clock = h.clock.Add(2 * 24 * time.Hour)
	h.press(":", `"review"`, "enter")
	if h.header() != "Review  · 2 projects" {
		t.Fatalf("header = %q", h.header())
	}
	h.want(
		h.edge("▌   Alpha", "2 to do  12d"),
		h.edge("    Beta v2", "∅   2d"),
	)
	// Keeping is moving on; parking and closing are stamps; Enter looks in.
	h.press("enter")
	h.want(
		"▌ ○ One @Nov 16",
		"  ◔ Two",
	)
	h.press("esc", "$", `"parked"`, "enter", "j", "$", `"done"`, "enter")
	h.want(
		"  ◌ Alpha +2",
		"▌ ✓ Beta v2",
	)
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting",
		"Stale",
		"Projects",
		"1 parked project",
	)
}

// Review reads a project's age as the sidebar draws it: nothing for one
// touched today, and otherwise on the gradient against the oldest project,
// so the project left alone longest is the one drawn red.
func TestReviewDrawsAgesAsTheSidebarDoes(t *testing.T) {
	h := two(t)
	h.clock = h.clock.Add(15 * 24 * time.Hour)
	h.press("A", `" v2"`, "esc") // Beta is touched today; Alpha is not
	h.press(":", `"review"`, "enter")
	h.want(
		h.edge("▌   Alpha", "2 to do  15d"),
		strings.TrimRight(h.edge("    Beta v2", "∅     "), " "),
	)
	if raw := h.m.View().Content; !strings.Contains(raw, ageStyle(darkPalette, 1, 1)+"15d") {
		t.Fatalf("the oldest age is not red:\n%q", raw)
	}
	h.clock = h.clock.Add(3 * 24 * time.Hour)
	h.press(":", `"review"`, "enter")
	raw := h.m.View().Content
	if !strings.Contains(raw, ageStyle(darkPalette, 1, 1)+"18d") {
		t.Fatalf("the oldest age is not red:\n%q", raw)
	}
	if !strings.Contains(raw, ageStyle(darkPalette, 3, 18)+"3d") {
		t.Fatalf("a younger age is not drawn against the oldest:\n%q", raw)
	}
}

// Waiting and Stale draw a row's age against the oldest row shown.
func TestWaitingAndStaleDrawAgesAgainstTheOldest(t *testing.T) {
	h := two(t) // Two is waiting
	h.clock = h.clock.Add(3 * 24 * time.Hour)
	h.press("f", `"waiting"`, "enter")
	if raw := h.m.View().Content; !strings.Contains(raw, ageStyle(darkPalette, 1, 1)+"3d") {
		t.Fatalf("the only wait, and so the longest, is not red:\n%q", raw)
	}
	h.clock = h.clock.Add(12 * 24 * time.Hour)
	// One has sat open and untouched as long.
	h.press("f", `"stale"`, "enter")
	h.want(h.edge("▌ ○ One @Nov 16 · Alpha", "15d"))
	if raw := h.m.View().Content; !strings.Contains(raw, ageStyle(darkPalette, 1, 1)+"15d") {
		t.Fatalf("the oldest stale age is not red:\n%q", raw)
	}
}

// An age runs smoothly from the text colour, for one just begun, through
// pale yellow, yellow and orange to red for the oldest, in both palettes.
func TestAgeGradient(t *testing.T) {
	rgb := func(c [3]float64) string {
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", int(c[0]), int(c[1]), int(c[2]))
	}
	for _, p := range []palette{darkPalette, lightPalette} {
		if got := ageStyle(p, 0, 40); got != p.text {
			t.Errorf("a new age is %q, not the text colour %q", got, p.text)
		}
		if got := ageStyle(p, 0, 0); got != p.text {
			t.Errorf("with nothing old, an age is %q, not the text colour", got)
		}
		for i, c := range p.ages {
			if got := ageStyle(p, int64(i*10), 40); got != rgb(c) {
				t.Errorf("stop %d is %q, want %q", i, got, rgb(c))
			}
		}
		// Between two stops it is between them, a step at a time.
		var prev [3]int
		for d := int64(0); d <= 400; d++ {
			var c [3]int
			fmt.Sscanf(ageStyle(p, d, 400), "\x1b[38;2;%d;%d;%dm", &c[0], &c[1], &c[2])
			for k := range c {
				if d > 0 && (c[k]-prev[k] > 3 || prev[k]-c[k] > 3) {
					t.Fatalf("age %d/400 jumps from %v to %v", d, prev, c)
				}
			}
			prev = c
		}
	}
}
