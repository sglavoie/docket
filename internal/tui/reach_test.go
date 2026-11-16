package tui

import (
	"strings"
	"testing"
	"time"
)

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
