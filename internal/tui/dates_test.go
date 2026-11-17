package tui

import (
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
		"▌ ○ First · 25d · Project",
		"  ○ Second · 25d · Project",
		"  ○ Renew passport @@Nov 20 · 25d · Project",
	)
	h.press("-", `"@@"`, "enter", "A")
	h.want(
		"▌ ○ First",
		"  ○ Second · 25d · Project",
		"  ○ Renew passport @@Nov 20 · 25d · Project",
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
	if h.m.msg != "saved: /open bugs is in the sidebar, and f opens it by name" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("esc", "x", "f", `"/op"`)
	if h.bottom() != "Go to › /op   /open bugs" {
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
	again.press("f", `"/open"`, "enter")
	again.want(
		"▌ ○ Fix login ! #bug · Alpha",
		"  ○ Refactor auth #bug · Alpha",
	)
	again.press(":", `"forget open bugs"`, "enter", "f", `"/open"`, "enter")
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
		"▌ · Alpha · 12d · 2 to do",
		"  · Beta v2 · 2d · ∅",
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
		"parked 1",
	)
}

// Review reads a project's age as the sidebar draws it: nothing for one
// touched today, dim while it is young, and in the text colour past the
// Stale rule, so the project left alone is the one the eye finds.
func TestReviewDrawsAgesAsTheSidebarDoes(t *testing.T) {
	h := two(t)
	h.clock = h.clock.Add(15 * 24 * time.Hour)
	h.press("A", `" v2"`, "esc") // Beta is touched today; Alpha is not
	h.press(":", `"review"`, "enter")
	h.want(
		"▌ · Alpha · 15d · 2 to do",
		"  · Beta v2 · ∅",
	)
	if raw := h.m.View().Content; !strings.Contains(raw, darkPalette.text+"· 15d") {
		t.Fatalf("a stale age is not in the text colour:\n%q", raw)
	}
	h.clock = h.clock.Add(3 * 24 * time.Hour)
	h.press(":", `"review"`, "enter")
	if raw := h.m.View().Content; !strings.Contains(raw, darkPalette.dim+"· 3d") {
		t.Fatalf("a recent age is not dim:\n%q", raw)
	}
}
