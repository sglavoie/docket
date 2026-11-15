package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/store"
)

// flaky is a backend whose writes can be made to fail.
type flaky struct {
	Backend
	fail bool
}

func (f *flaky) Commit(ops []outline.Op, now int64) error {
	if f.fail {
		return errors.New("disk full")
	}
	return f.Backend.Commit(ops, now)
}

func TestAFailedSaveChangesNothing(t *testing.T) {
	h := work(t)
	disk := &flaky{Backend: h.m.backend, fail: true}
	h.m.backend = disk
	before := h.rows()
	steps := len(h.m.undo)

	for _, keys := range [][]string{
		{"x"},
		{"o", `"Lost"`, "esc"},
		{"e", `" more"`, "esc"},
		{"d", "d"},
		{"<"},
		{"a", `"Lost too"`, "enter", "enter"},
		{"u"},
	} {
		h.press(keys...)
		if !strings.HasPrefix(h.m.msg, "not saved, nothing changed: disk full") && keys[0] != "a" {
			t.Fatalf("%v: msg = %q", keys, h.m.msg)
		}
		if h.rows() != before {
			t.Fatalf("%v changed the screen:\n%s", keys, h.rows())
		}
	}
	if len(h.m.undo) != steps || len(h.m.tree.Children(inboxID)) != 0 {
		t.Fatal("a failed save left something behind")
	}

	disk.fail = false
	h.press("x")
	h.want(
		"  ▾ Beta",
		"▌   ✓ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func reopen(t *testing.T, h *harness) *harness {
	t.Helper()
	h.st.Close()
	st, err := store.Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return resume(t, h.path, st, h.clock).wide()
}

func TestUndoAndRedoSurviveARestart(t *testing.T) {
	h := work(t)
	h.press("x", "j", "j", "d", "d", "u")
	again := reopen(t, h)
	again.press("u")
	again.want(
		"  ▾ Beta",
		"▌   ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	again.press("ctrl+r", "ctrl+r")
	again.want(
		"  ▾ Beta",
		"▌   ✓ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestAPlaceRemembersItsCursor(t *testing.T) {
	h := work(t)
	h.press("p", `"alp"`, "enter", "j", "j")
	h.want(
		"  ○ Fix login ! #bug",
		"  ○ Write docs @fri #docs",
		"▌ ○ Refactor auth #bug",
	)
	h.press("p", `"proj"`, "enter", "p", `"alp"`, "enter")
	h.want(
		"  ○ Fix login ! #bug",
		"  ○ Write docs @fri #docs",
		"▌ ○ Refactor auth #bug",
	)
	// Projects remembers its own.
	h.press("p", `"proj"`, "enter")
	h.want(
		"  ▾ Beta",
		"▌   ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestTheScreenFollowsTheClockWithoutAKey(t *testing.T) {
	h := two(t)
	h.clock = h.clock.Add(15 * 24 * time.Hour)
	if _, cmd := h.m.Update(tickMsg{}); cmd == nil {
		t.Fatal("the tick does not rearm")
	}
	h.wantSide(
		"Inbox",
		"Today",
		"Waiting 1",
		"Stale 1",
		"▌ Projects",
		"Beta ∅",
		"Alpha ! 15d",
	)
	h.want(
		"▌   Beta",
		"  ▾ Alpha",
		"    ○ One @Nov 16",
		"    ◔ Two",
	)
}

func TestEditingMovesByWord(t *testing.T) {
	h := work(t)
	h.press("o", `"one two three"`, "alt+left", "alt+left", `"X"`)
	if got := string(h.m.edit.buf); got != "one Xtwo three" {
		t.Fatalf("buf = %q", got)
	}
	h.press("alt+backspace", "alt+right", `"!"`, "alt+b", "alt+b", "alt+f", `"?"`)
	if got := string(h.m.edit.buf); got != "one? two! three" {
		t.Fatalf("buf = %q", got)
	}
	// Alt with a letter is a command, never text.
	h.press("alt+x", "esc")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"▌   ○ one? two! three",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestPickerArrowsWalkTheMatches(t *testing.T) {
	h := work(t)
	h.press("p")
	if h.bottom() != "Go to ›    Inbox · Today · Waiting · Stale · Projects · Beta · Alpha" {
		t.Fatalf("picker = %q", h.bottom())
	}
	h.press("up", "down", "down")
	if h.bottom() != "Go to ›    ‹ Waiting · Stale · Projects · Beta · Alpha" {
		t.Fatalf("picker = %q", h.bottom())
	}
	h.press("enter")
	if h.header() != "Waiting" {
		t.Fatalf("header = %q", h.header())
	}
	// Typing starts over from the best match, and the walk stops at the end.
	h.press("p", "down", `"a"`, "down", "down", "down", "down", "down", "enter")
	if h.header() != "Beta" {
		t.Fatalf("header = %q", h.header())
	}
}

func TestClosedRowsCanBeLeftOut(t *testing.T) {
	h := work(t)
	h.press("x", ":", `"closed"`, "enter")
	if h.m.msg != "closed rows hidden" || h.header() != "Projects  · closed hidden" {
		t.Fatalf("msg = %q, header = %q", h.m.msg, h.header())
	}
	h.want(
		"  ▾ Beta",
		"▌ ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	// A row closed here stays until the place is left.
	h.press("j", "x", "j")
	h.want(
		"  ▾ Beta",
		"  ▾ Alpha",
		"    ✓ Fix login ! #bug",
		"▌   ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	h.press("p", `"beta"`, "enter")
	h.want("  Nothing open here. :closed shows the closed rows.")
	h.press("p", `"proj"`, "enter")
	h.want(
		"  ▾ Beta",
		"  ▾ Alpha",
		"▌   ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)

	// Lists and filters name what they show, so they still show it; and
	// the setting outlives the session.
	h.press("/", `"fix"`, "enter")
	h.want(
		"▌ ▾ Beta",
		"    ✓ Fix cache #bug",
		"  ▾ Alpha",
		"    ✓ Fix login ! #bug",
	)
	again := reopen(t, h)
	again.want(
		"▌ ▾ Beta",
		"  ▾ Alpha",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	again.press(":", `"closed"`, "enter")
	again.want(
		"▌ ▾ Beta",
		"    ✓ Fix cache #bug",
		"  ▾ Alpha",
		"    ✓ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}
