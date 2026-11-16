package tui

import (
	"strings"
	"testing"
	"time"
)

func TestABlankGoToGoesBack(t *testing.T) {
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Alpha\n\tFirst\n\tSecond\nBeta\n\tOther\n\tLast")
		return h
	}
	t.Run("alternates with each cursor and sidebar entry", func(t *testing.T) {
		h := fixture(t)
		h.press("p", `"alpha"`, "enter", "j")
		alpha, alphaSide := h.rows(), h.m.sel
		h.press("p", `"beta"`, "enter", "j")
		beta, betaSide := h.rows(), h.m.sel
		h.press("p")
		if !strings.Contains(h.status(), "back Alpha") {
			t.Fatalf("back offer = %q", h.status())
		}
		h.press(`"alpha"`)
		if strings.Contains(h.status(), "back ") {
			t.Fatalf("typed name still offers back: %q", h.status())
		}
		h.press("esc", "p", "enter")
		if h.header() != "Alpha" || h.rows() != alpha || !h.m.sel.same(alphaSide) {
			t.Fatalf("first return = %q\n%s", h.header(), h.rows())
		}
		h.press("p", "enter")
		if h.header() != "Beta" || h.rows() != beta || !h.m.sel.same(betaSide) {
			t.Fatalf("second return = %q\n%s", h.header(), h.rows())
		}
	})

	t.Run("zoom and reach return to the held list", func(t *testing.T) {
		h := two(t)
		h.press("G", "t", "k", "t", "p", `"today"`, "enter", "j", "t")
		today := h.rows() // Unpicked One must remain in this held list.
		h.press("h", "enter", "o", `"Detail"`, "esc")
		zoomed, heading, side := h.rows(), h.header(), h.m.sel
		h.press("p", `"beta"`, "enter", "p", "enter")
		if h.header() != heading || h.rows() != zoomed || !h.m.sel.same(side) {
			t.Fatalf("zoom return = %q\n%s", h.header(), h.rows())
		}
		h.press("esc", "esc")
		// One gained a child, so its flat rendering now includes +1.
		if h.header() != "Today" || h.rows() != strings.Replace(today, "@tomorrow", "@tomorrow +1", 1) {
			t.Fatalf("Esc lost the held list or its cursor: %q\n%s", h.header(), h.rows())
		}
		h.press("p", "enter")
		if h.header() != "Beta" {
			t.Fatalf("Esc replaced the jump: %q", h.header())
		}
	})

	t.Run("filters sharing a root retain separate hits and cursors", func(t *testing.T) {
		h := fixture(t)
		h.press("p", `"alpha"`, "enter", "/", `"First|Second"`, "tab", "enter", "j")
		h.command("save pair")
		pair, heading := h.rows(), h.header()
		h.press("p", `"alpha"`, "enter", "/", `"First"`, "tab", "enter")
		first, firstHeading := h.rows(), h.header()
		h.press("p", "enter")
		if h.rows() != pair || h.header() != heading {
			t.Fatalf("first filter = %q\n%s", h.header(), h.rows())
		}
		h.press("p", "enter")
		if h.rows() != first || h.header() != firstHeading {
			t.Fatalf("second filter = %q\n%s", h.header(), h.rows())
		}
		// Saved filters also leave a restorable place and sidebar entry.
		h.press("p", `"/pair"`, "enter", "j")
		side := h.m.sel
		h.press("p", `"beta"`, "enter", "p", "enter")
		if h.rows() != pair || h.header() != heading || !h.m.sel.same(side) {
			t.Fatalf("saved filter return = %q\n%s", h.header(), h.rows())
		}
	})

	t.Run("held filter matches survive edits and jumping away", func(t *testing.T) {
		h := work(t)
		h.press("/", `"#bug"`, "tab", "enter", "j", "-", `"#bug"`, "enter")
		before, heading := h.rows(), h.header()
		h.press("p", `"beta"`, "enter", "p", "enter")
		if h.rows() != before || h.header() != heading {
			t.Fatalf("return recomputed held matches: %q\n%s", h.header(), h.rows())
		}
	})

	t.Run("closed rows kept by a reach survive a jump", func(t *testing.T) {
		h := two(t)
		h.press("G", "x")
		h.command("closed")
		h.press("/", `"$done"`, "tab", "enter", "h")
		before := h.rows()
		h.press("p", `"beta"`, "enter", "p", "enter")
		if h.rows() != before {
			t.Fatalf("return hid the closed arrival: %s", h.rows())
		}
		h.press("esc")
		h.want("▌ ✓ Two · Alpha")
	})

	for _, exit := range []string{"tab", "enter", "esc", "l", "right", "picker"} {
		t.Run("sidebar walk exits with "+exit, func(t *testing.T) {
			h := fixture(t)
			h.press("p", `"alpha"`, "enter", "j")
			before, side := h.rows(), h.m.sel
			h.press("tab", "k", "k", "k") // Projects, Stale, Waiting.
			// Canceling a picker keeps the sidebar visit in progress.
			h.press("p", `"beta"`, "esc")
			if exit == "picker" {
				h.press("p", `"beta"`, "enter")
			} else {
				h.press(exit)
			}
			destination := h.header()
			h.press("p", "enter")
			if h.header() != "Alpha" || h.rows() != before || !h.m.sel.same(side) {
				t.Fatalf("sidebar origin = %q\n%s", h.header(), h.rows())
			}
			h.press("p", "enter")
			if h.header() != destination {
				t.Fatalf("sidebar destination = %q, want %q", h.header(), destination)
			}
		})
	}

	t.Run("sidebar returning to its origin does not replace back", func(t *testing.T) {
		h := fixture(t)
		h.press("p", `"beta"`, "enter", "p", `"alpha"`, "enter")
		h.press("tab", "k", "j", "enter", "tab", "esc", "p", "enter")
		if h.header() != "Beta" {
			t.Fatalf("unchanged sidebar visit replaced back: %q", h.header())
		}
	})

	t.Run("before any jump blank picker still opens Inbox", func(t *testing.T) {
		h := fixture(t)
		h.press("enter", "esc", "tab", "esc", "p")
		if strings.Contains(h.status(), "back ") {
			t.Fatalf("non-jumps offered back: %q", h.status())
		}
		h.press("enter")
		if h.header() != "Inbox" {
			t.Fatalf("first blank jump = %q", h.header())
		}
		h.press("p", "enter")
		if h.header() != "Projects" {
			t.Fatalf("rootless return = %q", h.header())
		}
		h = open(t, h.path).wide()
		h.press("p")
		if strings.Contains(h.status(), "back ") {
			t.Fatalf("back persisted across launches: %q", h.status())
		}
	})

	for _, target := range []string{"alpha", "heading"} {
		t.Run("deleted root or ancestor is not offered/"+target, func(t *testing.T) {
			h := start(t).wide()
			h.pasteText("Alpha\n\tHeading:\n\t\tTask\nBeta")
			h.press("p", `"`+target+`"`, "enter", "p", `"projects"`, "enter", "g", "g", "d", "d", "p")
			if strings.Contains(h.status(), "back ") {
				t.Fatalf("deleted place offered: %q", h.status())
			}
			h.press("enter")
			if h.header() != "Inbox" {
				t.Fatalf("deleted place taken: %q", h.header())
			}
		})
	}
}

func TestHShowsAListsRowWhereItLives(t *testing.T) {
	t.Run("Today holds its rows and cursor", func(t *testing.T) {
		h := two(t)
		h.press("G", "t", "k", "t", "p", `"today"`, "enter", "j", "t")
		before := h.rows() // Unpicked here, One stays in the held list.
		h.press("h")
		if h.header() != "Alpha" {
			t.Fatalf("header = %q", h.header())
		}
		h.want("▌ ○ One @tomorrow", "  ◔ Two")
		h.press("j", "esc")
		if h.header() != "Today" || h.rows() != before {
			t.Fatalf("return = %q\n%s; want\n%s", h.header(), h.rows(), before)
		}
	})

	t.Run("flat filter holds its query hits and cursor", func(t *testing.T) {
		h := work(t)
		h.press("/", `"#bug"`, "tab", "enter", "j", "-", `"#bug"`, "enter")
		before, header := h.rows(), h.header()
		h.press("h")
		h.want("▌ ○ Fix login !", "  ○ Write docs @fri #docs", "  ○ Refactor auth #bug")
		// Nest a second reach from a filter whose root is the same as the
		// parent it opens. Each zoom must retain its own return row.
		h.press("/", `"auth"`, "tab", "enter", "h", "enter", "esc", "k", "esc")
		h.want("▌ ○ Refactor auth #bug · Alpha")
		h.press("esc", "g", "g", "esc")
		if h.header() != header || h.rows() != before {
			t.Fatalf("return = %q\n%s; want %q\n%s", h.header(), h.rows(), header, before)
		}
	})

	for _, state := range []string{"done", "canceled"} {
		t.Run(state+" row stays visible", func(t *testing.T) {
			h := two(t)
			h.press("G", "$", `"`+state+`"`, "enter")
			h.command("closed")
			h.press("/", `"$`+state+`"`, "tab", "enter", "h")
			glyph := "✓"
			if state == "canceled" {
				glyph = "✕"
			}
			h.want("  ○ One @tomorrow", "▌ "+glyph+" Two")
			h.press("k", "j") // The exception survives another redraw.
			h.want("  ○ One @tomorrow", "▌ "+glyph+" Two")
			h.press("esc")
			h.want("▌ " + glyph + " Two · Alpha")
			h.press("p", `"alpha"`, "enter")
			h.want("▌ ○ One @tomorrow") // Leaving ends the exception.
		})
	}

	for _, source := range []string{"list", "flat filter"} {
		for _, folded := range []bool{false, true} {
			name := source + "/unfolded"
			if folded {
				name = source + "/folded"
			}
			t.Run(name, func(t *testing.T) {
				h := two(t)
				h.press("G", "enter", "o", `"Detail"`, "esc", "esc", "t")
				if folded {
					h.press("h")
				}
				if source == "list" {
					h.press("p", `"today"`, "enter")
				} else {
					h.press("/", `"Two"`, "tab", "enter")
				}
				before := h.rows()
				h.press("l", "right")
				if h.rows() != before {
					t.Fatal("l moved in a flat place")
				}
				h.press("v", "h")
				if !strings.Contains(h.status(), "one row") {
					t.Fatalf("selection was not refused: %q", h.status())
				}
				h.press("esc", "left")
				if folded {
					h.want("  ○ One @tomorrow", "▌ ◔ Two +1")
				} else {
					h.want("  ○ One @tomorrow", "▌ ◔ Two", "      Detail")
				}
				h.press("esc")
				if h.rows() != before {
					t.Fatalf("return changed rows: %s", h.rows())
				}
			})
		}
	}

	for _, list := range []string{"waiting", "stale", "review"} {
		t.Run(list, func(t *testing.T) {
			h := two(t)
			h.clock = h.clock.Add(15 * 24 * time.Hour)
			if list == "review" {
				h.command("review")
			} else {
				h.press("p", `"`+list+`"`, "enter")
			}
			before, header, row := h.rows(), h.header(), h.m.current()
			h.press("h")
			wantHeader := "Alpha"
			if list == "review" {
				wantHeader = "Projects"
			}
			if h.header() != wantHeader || h.m.current() != row {
				t.Fatalf("arrival = %q, row %q", h.header(), h.m.current())
			}
			h.press("esc")
			if h.header() != header || h.rows() != before {
				t.Fatalf("return = %q\n%s", h.header(), h.rows())
			}
		})
	}
}
