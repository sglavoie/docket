package tui

import (
	"errors"
	"fmt"
	"slices"
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
	h.press("f", `"alp"`, "enter", "j", "j")
	h.want(
		"  ○ Fix login ! #bug",
		"  ○ Write docs @fri #docs",
		"▌ ○ Refactor auth #bug",
	)
	h.press("f", `"proj"`, "enter", "f", `"alp"`, "enter")
	h.want(
		"  ○ Fix login ! #bug",
		"  ○ Write docs @fri #docs",
		"▌ ○ Refactor auth #bug",
	)
	// Projects remembers its own.
	h.press("f", `"proj"`, "enter")
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
	h.press("f")
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
	h.press("f", "down", `"a"`, "down", "down", "down", "down", "down", "enter")
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
	h.press("f", `"beta"`, "enter")
	h.want("  Nothing open here. :closed shows the closed rows.")
	h.press("f", `"proj"`, "enter")
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

func TestHelpScrolls(t *testing.T) {
	m := &Model{width: 80, height: 10}
	m.openHelp()
	m.helpKey("j")
	if m.helpTop != 1 {
		t.Fatalf("j: top=%d", m.helpTop)
	}
	m.helpKey("ctrl+d")
	if m.helpTop != 1+m.helpPage()/2 {
		t.Fatalf("ctrl+d: top=%d", m.helpTop)
	}
	m.helpKey("G")
	if bottom := m.helpTop; bottom == 0 || !m.help {
		t.Fatalf("G: top=%d help=%v", bottom, m.help)
	}
	m.helpKey("g")
	m.helpKey("g")
	if m.helpTop != 0 {
		t.Fatalf("gg: top=%d", m.helpTop)
	}
	m.helpKey("ctrl+u")
	if m.helpTop != 0 {
		t.Fatalf("ctrl+u clamp: top=%d", m.helpTop)
	}
	m.helpKey("esc")
	if m.help {
		t.Fatal("esc should close help")
	}
}

func (f *flaky) SetFolds(changes map[string]bool) error {
	if f.fail {
		return errors.New("disk full")
	}
	return f.Backend.SetFolds(changes)
}

func TestFoldCommandsAndHalfPages(t *testing.T) {
	command := func(h *harness, text string) { h.press(":", `"`+text+`"`, "enter") }
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Alpha:\n\tBranch:\n\t\tLeaf #match\n\tOther\nBeta:\n\tOutside\n")
		return h
	}
	t.Run("levels and restart", func(t *testing.T) {
		h := fixture(t)
		h.press("g", "g", "j", "j")
		steps, generation := len(h.m.undo), h.m.tree.Gen()
		command(h, "fold 2")
		h.want("  ▾ Alpha:", "▌   ▸ Branch:", "    ○ Other", "  ▾ Beta:", "    ○ Outside")
		command(h, "fold 1")
		h.want("▌ ▸ Alpha:", "  ▸ Beta:")
		command(h, "unfold")
		h.want("▌ ▾ Alpha:", "    ▾ Branch:", "      ○ Leaf #match", "    ○ Other", "  ▾ Beta:", "    ○ Outside")
		h.press("j", "j")
		command(h, "fold")
		h.want("▌ ▸ Alpha:", "  ▸ Beta:")
		if len(h.m.folded) != 3 || len(h.m.undo) != steps || h.m.tree.Gen() != generation {
			t.Fatal("fold-all must cover hidden descendants without changing tree or undo")
		}
		again := reopen(t, h)
		again.want("▌ ▸ Alpha:", "  ▸ Beta:")
		command(again, "fold 2")
		again.want("▌ ▾ Alpha:", "    ▸ Branch:", "    ○ Other", "  ▾ Beta:", "    ○ Outside")
		command(again, "unfold")
		if len(again.m.folded) != 0 || len(again.m.vis) != 6 {
			t.Fatal("unfold did not restore every level")
		}
		again = reopen(t, again)
		if len(again.m.folded) != 0 || len(again.m.vis) != 6 {
			t.Fatal("unfold did not survive restart")
		}
	})
	t.Run("invalid and failed saves", func(t *testing.T) {
		h := fixture(t)
		before := h.rows()
		for _, bad := range []string{"nope", "0", "-1", "1.5", "999999999999999999999999999"} {
			command(h, "fold "+bad)
			if !strings.Contains(h.bottom(), bad) || h.rows() != before || len(h.m.folded) != 0 {
				t.Fatalf("invalid level %q changed state or was not named: %s", bad, h.bottom())
			}
		}
		h.m.backend = &flaky{Backend: h.m.backend, fail: true}
		command(h, "fold")
		if h.rows() != before || len(h.m.folded) != 0 || !strings.Contains(h.bottom(), "disk full") {
			t.Fatal("failed batch changed view state")
		}
	})
	t.Run("pane root and context filter", func(t *testing.T) {
		h := fixture(t)
		h.press("g", "g", "enter", "j")
		command(h, "fold")
		h.want("▌ ▸ Branch:", "  ○ Other")
		if len(h.m.folded) != 1 {
			t.Fatal("fold escaped the pane root")
		}
		command(h, "unfold")
		// A filter shows every match, so neither shape of it takes folds.
		for _, keys := range [][]string{{"/", `"#match"`, "enter"}, {"/", "tab", "enter"}} {
			h.press(keys...)
			before := h.rows()
			for _, cmd := range []string{"fold", "fold 2", "unfold"} {
				command(h, cmd)
				if !strings.Contains(h.bottom(), "nothing to fold") || h.rows() != before || len(h.m.folded) != 0 {
					t.Fatal("filter accepted folds")
				}
			}
		}
	})
	for _, place := range []string{"outline", "list", "filter", "flat filter"} {
		t.Run("paging "+place, func(t *testing.T) {
			h := start(t)
			h.pasteText("Alpha:\n")
			h.press("g", "g", "enter")
			var lines strings.Builder
			for i := 0; i < 30; i++ {
				fmt.Fprintf(&lines, "Item %02d\n", i)
			}
			h.pasteText(lines.String())
			switch place {
			case "list":
				h.press("g", "g")
				for range 30 {
					h.press("v")
				}
				h.press("t", "esc", "f", `"Today"`, "enter")
			case "filter", "flat filter":
				h.press("/", `"Item"`)
				if place == "flat filter" {
					h.press("tab")
				}
				h.press("enter")
			}
			// Height 12 means ten usable row lines and exactly five rows per step.
			if h.m.height != 12 || len(h.m.vis) != 30 {
				t.Fatal("paging fixture size")
			}
			before := append([]visRow(nil), h.m.vis...)
			list := append([]string(nil), h.m.at.rows...)
			if place == "list" {
				for _, cmd := range []string{"fold", "fold 1", "unfold"} {
					command(h, cmd)
					if !strings.Contains(h.bottom(), "nothing to fold") || len(h.m.folded) != 0 {
						t.Fatal("list accepted folds")
					}
				}
			}
			check := func(k string, want int) {
				t.Helper()
				h.press(k)
				if h.m.cursor != want {
					t.Fatalf("%s: cursor %d, want %d", k, h.m.cursor, want)
				}
				if !strings.Contains(h.rows(), "▌ ○ "+h.m.tree.Row(h.m.current()).Text()) {
					t.Fatalf("cursor hidden: %s", h.rows())
				}
				if !slices.Equal(h.m.vis, before) || !slices.Equal(h.m.at.rows, list) {
					t.Fatal("paging changed rows or order")
				}
			}
			h.press("g", "g")
			for range 10 {
				h.press("j")
			}
			check("ctrl+d", 15)
			check("ctrl+u", 10)
			h.press("G", "k")
			check("ctrl+d", 29)
			check("ctrl+d", 29)
			h.press("g", "g", "j")
			check("ctrl+u", 0)
			check("ctrl+u", 0)
			check("ctrl+f", 10)
			check("pgdown", 20)
			check("ctrl+f", 29)
			check("ctrl+b", 19)
			check("pgup", 9)
			check("ctrl+b", 0)
			check("end", 29)
			check("home", 0)
		})
	}
	t.Run("typing retains controls", func(t *testing.T) {
		h := fixture(t)
		h.press("g", "g", "e", "ctrl+a", "right", "ctrl+u")
		if string(h.m.edit.buf) != "lpha:" || h.m.edit.caret != 0 {
			t.Fatal("Ctrl+U stopped cutting the line")
		}
		cursor := h.m.cursor
		h.press("ctrl+d")
		if h.m.cursor != cursor || string(h.m.edit.buf) != "lpha:" {
			t.Fatal("Ctrl+D paged while typing")
		}
		h.press("esc", ":", `"export"`, "ctrl+u")
		if len(h.m.ask.ed.buf) != 0 {
			t.Fatal("Ctrl+U stopped cutting the prompt")
		}
	})
}
