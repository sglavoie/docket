package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
		{"A", `" more"`, "esc"},
		{"d", "d"},
		{"<", "<"},
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
		"Alpha @ 15d",
	)
	h.want(
		"▌ · Beta",
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
	if h.header() != "Beta  · 1 to do" {
		t.Fatalf("header = %q", h.header())
	}
}

func TestClosedRowsCanBeLeftOut(t *testing.T) {
	h := work(t)
	h.press("x", ":", `"closed"`, "enter")
	if h.m.msg != "closed rows hidden" || h.header() != "Projects  · 3 to do  · closed hidden" {
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
	if m.helpTop != 1+m.helpRows()/2 {
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
		h.want("  ▾ Alpha: 0/2", "▌   ▸ Branch: 0/1", "    ○ Other", "  ▾ Beta: 0/1", "    ○ Outside")
		command(h, "fold 1")
		h.want("▌ ▸ Alpha: 0/2", "  ▸ Beta: 0/1")
		command(h, "unfold")
		h.want("▌ ▾ Alpha: 0/2", "    ▾ Branch: 0/1", "      ○ Leaf #match", "    ○ Other", "  ▾ Beta: 0/1", "    ○ Outside")
		h.press("j", "j")
		command(h, "fold")
		h.want("▌ ▸ Alpha: 0/2", "  ▸ Beta: 0/1")
		if len(h.m.folded) != 3 || len(h.m.undo) != steps || h.m.tree.Gen() != generation {
			t.Fatal("fold-all must cover hidden descendants without changing tree or undo")
		}
		again := reopen(t, h)
		again.want("▌ ▸ Alpha: 0/2", "  ▸ Beta: 0/1")
		command(again, "fold 2")
		again.want("▌ ▾ Alpha: 0/2", "    ▸ Branch: 0/1", "    ○ Other", "  ▾ Beta: 0/1", "    ○ Outside")
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
		h.want("▌ ▸ Branch: 0/1", "  ○ Other")
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
		h.press("g", "g", "A", "ctrl+a", "right", "ctrl+u")
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

// The header says where the pane is among its rows once they overflow it,
// and nothing while every row is on screen.
func TestHeaderRulesTheOverflowingPane(t *testing.T) {
	h := work(t)
	if h.header() != "Projects  · 4 to do" {
		t.Fatalf("header = %q", h.header())
	}
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 5})
	for _, step := range []struct{ keys, want string }{
		{"gg", "Top"}, {"j", "Top"}, {"jj", "33%"}, {"G", "Bot"},
	} {
		h.press(`"` + step.keys + `"`)
		if got := h.header(); !strings.HasPrefix(got, "Projects  · 4 to do  ") || !strings.HasSuffix(got, " "+step.want) {
			t.Fatalf("after %s header = %q, want it to end in %s", step.keys, got, step.want)
		}
	}
	// Too narrow for both, the place keeps the line.
	h.m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	if got := h.lines()[0]; got != "Projects  · 4 to do" {
		t.Fatalf("narrow header = %q", got)
	}
}

// The header reads in three weights: the place, what is said about it, and a
// held filter, which hides rows. A standing hint is dimmer than a message.
func TestHeaderAndHintsCarryTheirWeight(t *testing.T) {
	h := work(t)
	c := h.m.colors()
	raw := func(n int) string {
		lines := strings.Split(h.m.View().Content, "\n")
		return lines[(n+len(lines))%len(lines)]
	}
	// The place is the last name; the way to it is dim.
	at := h.m.at
	h.m.at.root = h.m.vis[h.m.cursor].id
	if got := h.m.paneHeader(80, ""); !strings.HasPrefix(got, c.dim+"Beta › "+sgrReset+"Fix cache") {
		t.Fatalf("header = %q", got)
	}
	h.m.at = at
	h.press("/", `"#bug"`, "enter")
	if got := raw(0); !strings.Contains(got, c.accent+"/ #bug"+sgrReset) || !strings.Contains(got, c.dim+"  · 3 to do"+sgrReset) {
		t.Fatalf("filtered header = %q", got)
	}
	h.press("esc", "A")
	if got := raw(-1); !strings.Contains(got, c.dim+"Enter next row") {
		t.Fatalf("hint = %q", got)
	}
	h.press("esc", "g", "p")
	if got := raw(-1); h.m.msg == "" || strings.Contains(got, c.dim+h.m.msg) {
		t.Fatalf("message = %q, drawn %q", h.m.msg, got)
	}
}

// The command line names its commands as it is typed, so none has to be
// remembered: all of them at first, then fewer, then the one with what it does.
func TestTheCommandLineNamesItsCommands(t *testing.T) {
	h := work(t).wide()
	last := func() string { l := h.lines(); return strings.TrimSpace(l[len(l)-1]) }
	h.press(":")
	if got := last(); !strings.HasPrefix(got, ":    q · help · tutor · closed · ") {
		t.Fatalf("blank line = %q", got)
	}
	for typed, want := range map[string]string{
		"s":       "sort [keys] · save name · sync [folder|off]",
		"so":      "sort [keys]: order siblings once",
		"sort @":  "sort [keys]: order siblings once",
		"s/a":     "s/old/new: change text",
		"qui":     "q: quit",
		"h":       "help: the keys",
		"sorted":  "no such command",
		"unfold ": "unfold: open every fold here",
	} {
		h.press("ctrl+u", `"`+typed+`"`)
		if got := last(); !strings.Contains(got, want) {
			t.Errorf(":%s says %q, want %q in it", typed, got, want)
		}
	}
	h.press("ctrl+u", `"nonsense"`, "enter")
	if !strings.HasPrefix(h.status(), "no command nonsense: q, help, tutor, closed, numbers, all, s/old/new, ") {
		t.Fatalf("status = %q", h.status())
	}
}

// A sweep is a mode, so the status line says it is on and which key ends it.
func TestASweepIsSaidWhileItLasts(t *testing.T) {
	h := work(t).wide()
	last := func() string { l := h.lines(); return strings.TrimSpace(l[len(l)-1]) }
	h.press("j", "V", "j")
	if got := last(); got != "2 selected · V keeps · Esc clears" {
		t.Fatalf("sweeping = %q", got)
	}
	h.press("V")
	if got := last(); got != "2 selected · Esc clears" {
		t.Fatalf("kept = %q", got)
	}
}

// The row being typed shows the glyph of what its line makes it so far, so
// what Enter will keep is seen before it is pressed.
func TestTheTypedRowShowsWhatItWillBe(t *testing.T) {
	h := work(t)
	h.press("g", "g", "j", "o", `"Call"`)
	h.want("  ▾ Beta", "    ○ Fix cache #bug", "▌   ○ Call", "  ▾ Alpha", "    ○ Fix login ! #bug", "    ○ Write docs @fri #docs", "    ○ Refactor auth #bug")
	h.press(`" $w"`)
	if got := strings.Split(h.rows(), "\n")[2]; got != "▌   ◔ Call $waiting" { // the rest is the completion offered
		t.Fatalf("typed state = %q", got)
	}
	h.press("ctrl+u", `"Later:"`)
	if got := strings.Split(h.rows(), "\n")[2]; got != "▌   · Later:" {
		t.Fatalf("typed heading = %q", got)
	}
	h.press("esc")
	if got := strings.Split(h.rows(), "\n")[2]; got != "▌   · Later:" {
		t.Fatalf("kept heading = %q", got)
	}
	// Retyping an existing row previews it the same way.
	h.press("k", "A", `" $done"`)
	if got := strings.Split(h.rows(), "\n")[1]; got != "▌   ✓ Fix cache #bug $done" {
		t.Fatalf("retyped = %q", got)
	}
}

// A flat row says what the header does not: inside a project that is the
// heading it lives under, and nothing folds, so no row offers to.
func TestAFlatRowNamesWhatTheHeaderDoesNot(t *testing.T) {
	h := work(t)
	h.press("G", "o", `"Later:"`, "enter", "tab", `"Ship it"`, "esc")
	h.press("/", `"$open"`, "tab", "enter")
	h.want("▌ ○ Fix cache #bug · Beta", "  ○ Fix login ! #bug · Alpha", "  ○ Write docs @fri #docs · Alpha", "  ○ Refactor auth #bug · Alpha", "  ○ Ship it · Alpha")
	h.press("esc", "g", "g", "}", "enter", "/", `"$open"`, "tab", "enter")
	h.want("▌ ○ Fix login ! #bug", "  ○ Write docs @fri #docs", "  ○ Refactor auth #bug", "  ○ Ship it · Later")
	h.press("/", "ctrl+u", `"-$"`, "enter")
	h.want("▌ · Later: 0/1")
}

// What is said after a flat row keeps its dot when the row wraps.
func TestAWrappedFlatRowKeepsItsDots(t *testing.T) {
	h := work(t)
	h.press("g", "g", "j", "$", `"w"`, "enter", "f", `"Waiting"`, "enter")
	for width := 12; width < 40; width++ {
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 12})
		for _, l := range strings.Split(h.rows(), "\n") {
			if strings.HasSuffix(l, "·") {
				t.Fatalf("at %d columns a dot is left behind:\n%s", width, h.rows())
			}
		}
	}
}

// An overdue project is told by weight as well as colour, and Today says
// how much of it is done.
func TestOverdueIsBoldAndTodayCountsWhatIsDone(t *testing.T) {
	h := work(t).wide()
	h.press("g", "g", "j", "@", `"2020-01-01"`, "enter", "t", "j", "j", "t")
	if raw := h.m.View().Content; !strings.Contains(raw, darkPalette.alert+sgrBold+"@"+sgrReset) {
		t.Fatalf("overdue mark is not bold:\n%q", raw)
	}
	h.press("f", `"Today"`, "enter")
	if h.header() != "Today  · 2 to do" {
		t.Fatalf("header = %q", h.header())
	}
	h.press("x")
	if h.header() != "Today  · 1 to do · 1 done" {
		t.Fatalf("header = %q", h.header())
	}
	h.press("j", "x")
	if h.header() != "Today  · all 2 done" {
		t.Fatalf("header = %q", h.header())
	}
}

// A deadline within a week is in the colour the sidebar marks its project
// with, so the row behind the mark is found; one further off stays quiet.
func TestADeadlineThisWeekReadsAsTheSidebarMarksIt(t *testing.T) {
	h := work(t)
	h.press("o", `"Far @2026-12-25"`, "esc")
	c := h.m.colors()
	raw := h.m.View().Content
	if !strings.Contains(raw, c.warn+"@"+sgrReset) || !strings.Contains(raw, c.warn+"@fri") {
		t.Fatalf("the week's deadline is not marked on its row:\n%q", raw)
	}
	if !strings.Contains(raw, c.dim+"@Dec 25") {
		t.Fatalf("a far deadline is not quiet:\n%q", raw)
	}
}

// A heading with every task under it done keeps its weight and gives up its
// colour, so the eye passes over what is finished.
func TestAFinishedHeadingGivesUpItsColour(t *testing.T) {
	h := project(t)
	h.press("o", `"Later:"`, "enter", "tab", `"Idea"`, "esc")
	c := h.m.colors()
	if raw := h.m.View().Content; !strings.Contains(raw, c.accent+sgrBold+"Later:") {
		t.Fatalf("a heading with work left is not in the accent colour:\n%q", raw)
	}
	h.press("x")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ▾ Later: 1/1",
		"▌     ✓ Idea",
	)
	if raw := h.m.View().Content; strings.Contains(raw, c.accent+sgrBold+"Later:") || !strings.Contains(raw, c.dim+sgrBold+"Later:") {
		t.Fatalf("a finished heading keeps its colour:\n%q", raw)
	}
}

// Delete asks nothing, so it says how many rows went, those inside a fold
// included, and undo says they are back.
func TestDeleteSaysHowManyRowsWent(t *testing.T) {
	h := work(t)
	h.press("g", "g", "h", "d", "d")
	h.want(
		"▌ ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	if h.m.msg != "deleted 2 rows" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("u")
	if h.m.msg != "undo: 2 rows back" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("j", "j", "d", "d")
	if h.m.msg != "deleted 1 row" {
		t.Fatalf("msg = %q", h.m.msg)
	}
}

// What was not written is drawn in the alert colour, so it does not pass
// for a confirmation; an ordinary message stays in the text colour.
func TestAFailedSaveIsDrawnAsAnAlarm(t *testing.T) {
	h := work(t)
	disk := &flaky{Backend: h.m.backend, fail: true}
	h.m.backend = disk
	h.press("x")
	c := h.m.colors()
	if raw := h.m.View().Content; !strings.Contains(raw, c.alert+"not saved, nothing changed: disk full") {
		t.Fatalf("a failed save is not in the alert colour:\n%q", raw)
	}
	disk.fail = false
	h.press("y", "y")
	if raw := h.m.View().Content; strings.Contains(raw, c.alert+"copied") || !strings.Contains(raw, "copied 1 row") {
		t.Fatalf("a confirmation is drawn as an alarm:\n%q", raw)
	}
}

// A sidebar that takes a narrow pane keeps each count beside its name, and
// does not send it to the far edge.
func TestANarrowSidebarKeepsCountsBesideNames(t *testing.T) {
	h := work(t)
	h.press("a", `"Thought"`, "enter", "esc")
	h.m.Update(tea.WindowSizeMsg{Width: 70, Height: 12})
	h.press("tab")
	lines := h.lines()
	if got, want := strings.TrimRight(lines[0], " "), "  Inbox"+strings.Repeat(" ", sidebarAlone-8)+"1"; got != want {
		t.Fatalf("sidebar = %q, want %q", got, want)
	}
	h.m.Update(tea.WindowSizeMsg{Width: 30, Height: 12})
	if got, want := strings.TrimRight(h.lines()[0], " "), "  Inbox"+strings.Repeat(" ", 22)+"1"; got != want {
		t.Fatalf("sidebar = %q, want %q", got, want)
	}
}

// A header too narrow for its whole path gives up the rows above the place,
// from the project down, and keeps the place; a heading drops its colon
// there as it does in the pickers.
func TestANarrowHeaderKeepsThePlace(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Storage:"`, "enter", "tab",
		`"Decide op-log format"`, "enter", "tab", `"per field"`, "esc", "k", "enter")
	for width, want := range map[int]string{
		60: "Project › Storage › Decide op-log format",
		36: "… › Storage › Decide op-log format",
		30: "… › Decide op-log format",
		20: "… › Decide op-log f…",
	} {
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: 12})
		if h.header() != want {
			t.Fatalf("at %d columns header = %q, want %q", width, h.header(), want)
		}
	}
}

// The filter line counts its matches as a sentence would.
func TestTheFilterLineCountsAsASentence(t *testing.T) {
	h := work(t)
	for query, want := range map[string]string{"cache": "1 match · ", "fix": "2 matches · ", "zebra": "no match · "} {
		h.press("/", `"`+query+`"`)
		if !strings.HasPrefix(h.bottom(), "Filter › "+query+"   "+want) {
			t.Fatalf("prompt = %q", h.bottom())
		}
		h.press("esc")
	}
}
