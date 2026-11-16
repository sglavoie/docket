package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCtrlOGoesBackAlongTheTrailOfJumps(t *testing.T) {
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Alpha\n\tFirst\n\tSecond\nBeta\n\tOther\n\tLast")
		return h
	}
	t.Run("alternates with each cursor and sidebar entry", func(t *testing.T) {
		h := fixture(t)
		h.press("f", `"alpha"`, "enter", "j")
		alpha, alphaSide := h.rows(), h.m.sel
		h.press("f", `"beta"`, "enter", "j")
		beta, betaSide := h.rows(), h.m.sel
		// The picker repeats no key: back is Ctrl+O alone (rule 3).
		h.press("f")
		if strings.Contains(h.status(), "back ") {
			t.Fatalf("the picker offers back: %q", h.status())
		}
		h.press("esc", "ctrl+o")
		if h.header() != "Alpha" || h.rows() != alpha || !h.m.sel.same(alphaSide) {
			t.Fatalf("first return = %q\n%s", h.header(), h.rows())
		}
		h.press("ctrl+i")
		if h.header() != "Beta" || h.rows() != beta || !h.m.sel.same(betaSide) {
			t.Fatalf("second return = %q\n%s", h.header(), h.rows())
		}
		h.press("ctrl+i")
		if h.header() != "Beta" || h.status() != "no newer place" {
			t.Fatalf("past the newest = %q, %q", h.header(), h.status())
		}
	})

	t.Run("zoom and reach return to the held list", func(t *testing.T) {
		h := two(t)
		h.press("G", "t", "k", "t", "f", `"today"`, "enter", "j", "t")
		today := h.rows() // Unpicked One must remain in this held list.
		h.press("h", "enter", "o", `"Detail"`, "esc")
		zoomed, heading, side := h.rows(), h.header(), h.m.sel
		h.press("f", `"beta"`, "enter", "ctrl+o")
		if h.header() != heading || h.rows() != zoomed || !h.m.sel.same(side) {
			t.Fatalf("zoom return = %q\n%s", h.header(), h.rows())
		}
		h.press("esc", "esc")
		// One gained a child, so its flat rendering now includes +1.
		if h.header() != "Today" || h.rows() != strings.Replace(today, "@tomorrow", "@tomorrow +1", 1) {
			t.Fatalf("Esc lost the held list or its cursor: %q\n%s", h.header(), h.rows())
		}
		h.press("ctrl+i")
		if h.header() != "Beta" {
			t.Fatalf("Esc replaced the jump: %q", h.header())
		}
	})

	t.Run("filters sharing a root retain separate hits and cursors", func(t *testing.T) {
		h := fixture(t)
		h.press("f", `"alpha"`, "enter", "/", `"First|Second"`, "tab", "enter", "j")
		h.command("save pair")
		pair, heading := h.rows(), h.header()
		h.press("f", `"alpha"`, "enter", "/", `"First"`, "tab", "enter")
		first, firstHeading := h.rows(), h.header()
		h.press("ctrl+o")
		if h.rows() != pair || h.header() != heading {
			t.Fatalf("first filter = %q\n%s", h.header(), h.rows())
		}
		h.press("ctrl+i")
		if h.rows() != first || h.header() != firstHeading {
			t.Fatalf("second filter = %q\n%s", h.header(), h.rows())
		}
		// Saved filters also leave a restorable place and sidebar entry.
		h.press("f", `"/pair"`, "enter", "j")
		side := h.m.sel
		h.press("f", `"beta"`, "enter", "ctrl+o")
		if h.rows() != pair || h.header() != heading || !h.m.sel.same(side) {
			t.Fatalf("saved filter return = %q\n%s", h.header(), h.rows())
		}
	})

	t.Run("held filter matches survive edits and jumping away", func(t *testing.T) {
		h := work(t)
		h.press("/", `"#bug"`, "tab", "enter", "j", "-", `"#bug"`, "enter")
		before, heading := h.rows(), h.header()
		h.press("f", `"beta"`, "enter", "ctrl+o")
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
		h.press("f", `"beta"`, "enter", "ctrl+o")
		if h.rows() != before {
			t.Fatalf("return hid the closed arrival: %s", h.rows())
		}
		h.press("esc")
		h.want("▌ ✓ Two · Alpha")
	})

	for _, exit := range []string{"tab", "enter", "esc", "l", "right", "picker"} {
		t.Run("sidebar walk exits with "+exit, func(t *testing.T) {
			h := fixture(t)
			h.press("f", `"alpha"`, "enter", "j")
			before, side := h.rows(), h.m.sel
			h.press("tab", "k", "k", "k") // Projects, Stale, Waiting.
			// Canceling a picker keeps the sidebar visit in progress.
			h.press("f", `"beta"`, "esc")
			if exit == "picker" {
				h.press("f", `"beta"`, "enter")
			} else {
				h.press(exit)
			}
			destination := h.header()
			h.press("ctrl+o")
			if h.header() != "Alpha" || h.rows() != before || !h.m.sel.same(side) {
				t.Fatalf("sidebar origin = %q\n%s", h.header(), h.rows())
			}
			h.press("ctrl+i")
			if h.header() != destination {
				t.Fatalf("sidebar destination = %q, want %q", h.header(), destination)
			}
		})
	}

	t.Run("sidebar returning to its origin does not replace back", func(t *testing.T) {
		h := fixture(t)
		h.press("f", `"beta"`, "enter", "f", `"alpha"`, "enter")
		h.press("tab", "k", "j", "enter", "tab", "esc", "ctrl+o")
		if h.header() != "Beta" {
			t.Fatalf("unchanged sidebar visit replaced back: %q", h.header())
		}
	})

	t.Run("zooming is no jump, and the trail lasts for the session", func(t *testing.T) {
		h := fixture(t)
		h.press("enter", "esc", "tab", "esc", "ctrl+o")
		if h.header() != "Projects" || !strings.HasPrefix(h.status(), "no older place") {
			t.Fatalf("non-jumps left a trail: %q, %q", h.header(), h.status())
		}
		h.press("f", "enter")
		if h.header() != "Inbox" {
			t.Fatalf("blank jump = %q", h.header())
		}
		h.press("ctrl+o")
		if h.header() != "Projects" {
			t.Fatalf("rootless return = %q", h.header())
		}
		h = open(t, h.path).wide()
		h.press("ctrl+o")
		if !strings.HasPrefix(h.status(), "no older place") {
			t.Fatalf("the trail persisted across launches: %q", h.status())
		}
	})

	t.Run("a count walks that far and a new jump drops what was newer", func(t *testing.T) {
		h := fixture(t)
		h.press("f", `"alpha"`, "enter", "f", `"beta"`, "enter", "f", `"inbox"`, "enter", "3", "ctrl+o")
		if h.header() != "Projects" {
			t.Fatalf("3 Ctrl+O = %q", h.header())
		}
		h.press("2", "ctrl+i")
		if h.header() != "Beta" {
			t.Fatalf("2 Ctrl+I = %q", h.header())
		}
		h.press("f", `"alpha"`, "enter", "ctrl+i")
		if h.header() != "Alpha" || h.status() != "no newer place" {
			t.Fatalf("a jump kept newer places: %q, %q", h.header(), h.status())
		}
		h.press("ctrl+o")
		if h.header() != "Beta" {
			t.Fatalf("back after a new jump = %q", h.header())
		}
	})

	for _, target := range []string{"alpha", "heading"} {
		t.Run("deleted root or ancestor is passed over/"+target, func(t *testing.T) {
			h := start(t).wide()
			h.pasteText("Alpha\n\tHeading:\n\t\tTask\nBeta")
			h.press("f", `"`+target+`"`, "enter", "f", `"projects"`, "enter", "g", "g", "d", "d", "ctrl+o")
			if h.header() != "Projects" || h.status() != "" {
				t.Fatalf("deleted place taken: %q, %q", h.header(), h.status())
			}
			h.press("ctrl+o")
			if !strings.HasPrefix(h.status(), "no older place") {
				t.Fatalf("past the oldest = %q", h.status())
			}
		})
	}
}

func TestHShowsAListsRowWhereItLives(t *testing.T) {
	t.Run("Today holds its rows and cursor", func(t *testing.T) {
		h := two(t)
		h.press("G", "t", "k", "t", "f", `"today"`, "enter", "j", "t")
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
			h.press("f", `"alpha"`, "enter")
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
					h.press("f", `"today"`, "enter")
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
				h.press("f", `"`+list+`"`, "enter")
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

func TestFilterReadsAnyTagAnyStateAndMixedAlternatives(t *testing.T) {
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Home #home $open\nFlagged ! $doing\nWaiting $waiting\nOverdue @2026-11-14 $open\nMilk $note\nFood #food $note\nDone $done\nCanceled $canceled\nParked $parked\nLiteral $open|@nope $note\nLiteral milk|#123 $note\nLiteral $open|zzz $note\nLiteral #home| $note")
		return h
	}
	t.Run("filter rows", func(t *testing.T) {
		cases := []struct {
			query string
			rows  []string
		}{
			{"#", []string{"▌ ○ Home #home", "    Food #food"}},
			{"-#", []string{"▌ ● Flagged !", "  ◔ Waiting", "  ○ Overdue @Nov 14", "    Milk", "  ✓ Done", "  ✕ Canceled", "  ◌ Parked", "    Literal $open|@nope", "    Literal milk|#123", "    Literal $open|zzz", "    Literal #home|"}},
			{"$", []string{"▌ ○ Home #home", "  ● Flagged !", "  ◔ Waiting", "  ○ Overdue @Nov 14", "  ✓ Done", "  ✕ Canceled", "  ◌ Parked"}},
			{"-$", []string{"▌   Milk", "    Food #food", "    Literal $open|@nope", "    Literal milk|#123", "    Literal $open|zzz", "    Literal #home|"}},
			{"$note", []string{"▌   Milk", "    Food #food", "    Literal $open|@nope", "    Literal milk|#123", "    Literal $open|zzz", "    Literal #home|"}},
			{"#home|!", []string{"▌ ○ Home #home", "  ● Flagged !"}},
			{"!|#home", []string{"▌ ○ Home #home", "  ● Flagged !"}},
			{"$open|doing", []string{"▌ ○ Home #home", "  ● Flagged !", "  ○ Overdue @Nov 14"}},
			{"#home|food", []string{"▌ ○ Home #home", "    Food #food"}},
			{"$waiting|@overdue", []string{"▌ ○ Overdue @Nov 14", "  ◔ Waiting"}},
			{"milk|#food", []string{"▌   Milk", "    Food #food", "    Literal milk|#123"}},
			{"#|$note", []string{"▌ ○ Home #home", "    Milk", "    Food #food", "    Literal $open|@nope", "    Literal milk|#123", "    Literal $open|zzz", "    Literal #home|"}},
			{"-$|#", []string{"▌   Milk", "    Literal $open|@nope", "    Literal milk|#123", "    Literal $open|zzz", "    Literal #home|"}},
			{"$open|@nope", []string{"▌   Literal $open|@nope"}},
			{"milk|#123", []string{"▌   Literal milk|#123"}},
			{"$open|zzz", []string{"▌   Literal $open|zzz"}},
			{"#home|", []string{"▌   Literal #home|"}},
			{"#home|$waiting|food", []string{"▌ ○ Home #home", "  ◔ Waiting", "    Food #food"}},
		}
		for _, tc := range cases {
			t.Run(tc.query, func(t *testing.T) {
				h := fixture(t)
				h.press("/", `"`+tc.query+`"`)
				if !h.m.at.flat {
					h.press("tab")
				}
				h.want(tc.rows...)
				h.press("esc")
			})
		}
	})
	t.Run("completion follows the active side", func(t *testing.T) {
		h := fixture(t)
		for _, tc := range []struct{ typed, want string }{
			{"#home|$wa", "#home|$waiting"},
			{"$waiting|@ov", "$waiting|@overdue"},
			{"milk|#fo", "milk|#food"},
			{"#home|fo", "#home|food"},
			{"$open|do", "$open|doing"},
			{"@@later|to", "@@later|today"},
			{"#home|@@la", "#home|@@later"},
			{"@overdue|$ne", "@overdue|$next"},
			{"$open|#food|do", "$open|#food|doing"},
			{"milk|#food|mi", "milk|#food|mi"},
			{"#home|!", "#home|!"},
			{"milk|", "milk|"},
		} {
			h.press("/", `"`+tc.typed+`"`, "right")
			if h.m.at.query != tc.want {
				t.Errorf("completed %q to %q, want %q", tc.typed, h.m.at.query, tc.want)
			}
			h.press("esc")
		}
	})
	t.Run("colour follows each side and unreadable words stay literal", func(t *testing.T) {
		h := fixture(t)
		for _, tc := range []struct{ query, painted string }{
			{"#", sgrAccent + "#" + sgrReset},
			{"-$", sgrAccent + "-$" + sgrReset},
			{"milk|#food", "milk|" + sgrAccent + "#food" + sgrReset},
			{"牛奶|#食品", "牛奶|" + sgrAccent + "#食品" + sgrReset},
			{"-#home|!", sgrAccent + "-#home" + sgrReset + "|" + sgrAccent + "!" + sgrReset},
			{"$open|doing", sgrAccent + "$open" + sgrReset + "|" + sgrAccent + "doing" + sgrReset},
			{"#home|@@later", sgrAccent + "#home" + sgrReset + "|" + sgrAccent + "@@later" + sgrReset},
			{"milk|#123", sgrWarn + "milk|#123" + sgrReset},
			{"#home|@nope", sgrWarn + "#home|@nope" + sgrReset},
		} {
			h.press("/", `"`+tc.query+`"`)
			if raw := h.m.View().Content; !strings.Contains(raw, tc.painted) {
				t.Errorf("%q missing coloured spans %q: %q", tc.query, tc.painted, raw)
			}
			h.press("esc")
		}
	})
}

func TestAFilterTermReachesTheRowsAbove(t *testing.T) {
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Docket #office\n\tWork: #work\n\t\tFirst $open\n\t\tNested: #work $waiting\n\t\t\tDeep $open\n\tHome: #home\n\t\tSecond $open\n\tPlain $open\nElsewhere\n\tOther $open")
		return h
	}
	t.Run("context keeps dim ancestors and all takes matches only", func(t *testing.T) {
		h := fixture(t)
		h.press("/", `"^#work"`, "enter")
		h.want("▌ ▾ Docket #office", "    ▾ Work: #work", "      ○ First", "      ◔ Nested: #work", "        ○ Deep")
		if !strings.Contains(h.m.View().Content, sgrDim+"Work:") {
			t.Fatal("context heading is not dimmed")
		}
		h.command("all")
		if got := h.selected(); got != "First, Nested:, Deep" {
			t.Fatalf("selected %q", got)
		}
	})
	t.Run("flat excludes the tagged heading itself and negates ancestry", func(t *testing.T) {
		h := fixture(t)
		h.press("/", `"^#work"`, "tab")
		h.want("▌ ○ First · Docket", "  ◔ Nested: #work +1 · Docket", "  ○ Deep · Docket")
		h.press("esc", "/", `"-^#work"`)
		h.want("▌ ▾ Docket #office", "  ▾ Work: #work · Docket", "  ▾ Home: #home · Docket", "  ○ Second · Docket", "  ○ Plain · Docket", "  ▾ Elsewhere", "  ○ Other · Elsewhere")
	})
	t.Run("project name scopes next action from Projects", func(t *testing.T) {
		h := fixture(t)
		h.press("/", `"^docket $next"`, "tab")
		h.want("▌ ○ First · Docket")
	})
	t.Run("project tag counts above the pane root", func(t *testing.T) {
		h := fixture(t)
		h.press("f", `"nested"`, "enter", "/", `"^#office ^#work"`)
		h.want("▌ ○ Deep")
		h.press("esc", "/", `"-^#office"`)
		h.want("  Nothing matches.")
	})
	t.Run("Today tests parents of held rows", func(t *testing.T) {
		h := fixture(t)
		h.press("/", `"$open"`, "tab", "enter")
		h.command("all")
		h.press("t", "f", `"today"`, "enter", "/", `"^#work"`)
		h.want("▌ ○ First · Docket", "  ○ Deep · Docket")
		h.press("esc", "/", `"-^#work"`)
		h.want("▌ ○ Second · Docket", "  ○ Plain · Docket", "  ○ Other · Elsewhere")
	})
	t.Run("ancestor grammar and independent terms", func(t *testing.T) {
		for _, tc := range []struct{ query, names string }{
			{"^#work|home", "First, Nested:, Deep, Second"},
			{"-^#work|home", "Docket, Work:, Home:, Plain, Elsewhere, Other"},
			{"^#home|$waiting", "Deep, Second"},
			{"^#work ^$waiting", "Deep"},
			{"^#work -^$waiting", "First, Nested:"},
			{"^$", "Deep"},
			{"^#", "Work:, First, Nested:, Deep, Home:, Second, Plain"},
			{"^$note", "Work:, First, Nested:, Deep, Home:, Second, Plain, Other"},
			{"^ $next", "First, Other"},
			{"-^ $next", "First, Other"},
		} {
			t.Run(tc.query, func(t *testing.T) {
				h := fixture(t)
				h.press("/", `"`+tc.query+`"`, "tab", "enter")
				h.command("all")
				if got := h.selected(); got != tc.names {
					t.Fatalf("selected %q, want %q", got, tc.names)
				}
			})
		}
	})
	t.Run("doubled prefix and unreadable alternatives are literal", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("^^marker\nLiteral $open|@nope\n\tChild $open")
		h.press("/", `"^^marker"`, "tab")
		h.want("▌   ^^marker")
		h.press("esc", "/", `"^$open|@nope"`)
		h.want("▌ ○ Child · Literal $open|@nope")
	})
	t.Run("completion and colour follow the term after the prefix", func(t *testing.T) {
		h := fixture(t)
		for _, tc := range []struct{ typed, want string }{
			{"^#wo", "^#work"}, {"-^#wo", "-^#work"},
			{"^#work|ho", "^#work|home"}, {"^#work|$wa", "^#work|$waiting"},
			{"^@@la", "^@@later"}, {"^^#wo", "^^#wo"},
		} {
			h.press("/", `"`+tc.typed+`"`, "right")
			if h.m.at.query != tc.want {
				t.Errorf("completed %q to %q, want %q", tc.typed, h.m.at.query, tc.want)
			}
			h.press("esc")
		}
		for _, tc := range []struct{ query, painted string }{
			{"^#wo", sgrAccent + "^#wo" + sgrReset},
			{"-^#work|$waiting", sgrAccent + "-^#work" + sgrReset + "|" + sgrAccent + "$waiting" + sgrReset},
			{"^$nope", sgrWarn + "^$nope" + sgrReset},
			{"^#work|@nope", sgrWarn + "^#work|@nope" + sgrReset},
			{"^^#work", "^^#work"},
		} {
			h.press("/", `"`+tc.query+`"`)
			if raw := h.m.View().Content; !strings.Contains(raw, tc.painted) {
				t.Errorf("%q missing coloured spans %q", tc.query, tc.painted)
			}
			h.press("esc")
		}
	})
}

func TestSortPutsSiblingsInOrder(t *testing.T) {
	fixture := func(t *testing.T, rows string) *harness {
		h := start(t).wide()
		h.pasteText("Project\n\t" + strings.ReplaceAll(rows, "\n", "\n\t"))
		h.press("g", "g", "enter", "g", "g")
		return h
	}
	for _, tc := range []struct {
		name, rows, command string
		want                []string
	}{
		{"deadline and children", "Late @+3d\n\tZ child\n\tA child\nUndated\nSoon @today\nTie @today\nAlso undated", "sort @", []string{
			"  ○ Soon @today", "  ○ Tie @today", "▌ ○ Late @wed", "      Z child", "      A child", "  ○ Undated", "  ○ Also undated",
		}},
		{"flag then deadline", "Plain late @+3d\nFlag late ! @+3d\nPlain early @today\nFlag missing !\nFlag early ! @today\nPlain missing", "sort ! @", []string{
			"  ○ Flag early ! @today", "  ○ Flag late ! @wed", "  ○ Flag missing !", "  ○ Plain early @today", "▌ ○ Plain late @wed", "  ○ Plain missing",
		}},
		{"state", "Canceled $canceled\nDone $done\nParked $parked\nNote $note\nWaiting $waiting\nOpen\nDoing $doing\nDoing again $doing", "sort $", []string{
			"  ● Doing", "  ● Doing again", "  ○ Open", "  ◔ Waiting", "    Note", "  ◌ Parked", "  ✓ Done", "▌ ✕ Canceled",
		}},
		{"text and case ties", "zebra\nBeta\nalpha\nALPHA\nbeta", "sort", []string{
			"  ○ alpha", "  ○ ALPHA", "  ○ Beta", "  ○ beta", "▌ ○ zebra",
		}},
		{"three keys", "Open ! @today\nDoing ! @today $doing\nEarlier @2026-11-14\nLater ! @+3d $doing", "sort ! @ $", []string{
			"  ● Doing ! @today", "▌ ○ Open ! @today", "  ● Later ! @wed", "  ○ Earlier @Nov 14",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := fixture(t, tc.rows)
			parent, cur, steps := h.m.at.root, h.m.current(), len(h.m.undo)
			before := slices.Clone(h.m.tree.Children(parent))
			h.command(tc.command)
			h.want(tc.want...)
			if h.m.current() != cur || len(h.m.undo) != steps+1 || h.m.msg != "sorted "+fmt.Sprint(len(before))+" rows" {
				t.Fatalf("sort lost cursor, undo grouping or count: %q", h.m.msg)
			}
			ordered, gen := h.rows(), h.m.tree.Gen()
			h.command(tc.command)
			if h.rows() != ordered || h.m.tree.Gen() != gen || len(h.m.undo) != steps+1 || h.m.msg != "already in order" {
				t.Fatal("sorting an ordered span changed rows or recorded an undo step")
			}
			h.press("u")
			if !slices.Equal(h.m.tree.Children(parent), before) || len(h.m.undo) != steps {
				t.Fatal("one undo did not restore sibling order")
			}
		})
	}

	t.Run("selected span with selected descendants", func(t *testing.T) {
		h := fixture(t, "Before\nZulu\n\tZ child\n\tA child\nAlpha\nAfter")
		h.press("j", "v", "v", "v", "v", "k", "k", "k")
		cur, picked, steps := h.m.current(), maps.Clone(h.m.picked), len(h.m.undo)
		kids := slices.Clone(h.m.tree.Children(h.m.at.root))
		left, right := *h.m.tree.Row(kids[0]), *h.m.tree.Row(kids[len(kids)-1])
		h.command("sort")
		h.want("  ○ Before", " ┃○ Alpha", " ┃○ Zulu", "▌┃    Z child", " ┃    A child", "  ○ After")
		if h.m.current() != cur || !maps.Equal(h.m.picked, picked) || len(h.m.undo) != steps+1 || len(h.m.undo[steps]) != 2 || h.m.msg != "sorted 2 rows" {
			t.Fatal("sort lost cursor/selection or moved descendants separately")
		}
		if h.m.tree.Row(left.ID).Pos != left.Pos || h.m.tree.Row(right.ID).Pos != right.Pos {
			t.Fatal("sort moved a row outside the selected span")
		}
		if h.m.tree.Row(kids[2]).Pos <= left.Pos || h.m.tree.Row(kids[1]).Pos >= right.Pos {
			t.Fatal("sort escaped the original span")
		}
		h.press("u")
		if !slices.Equal(h.m.tree.Children(h.m.at.root), kids) {
			t.Fatal("undo did not restore selected span")
		}
	})

	for _, place := range []string{"gap", "different parents", "list", "filter", "flat filter", "unknown key"} {
		t.Run("refuses "+place, func(t *testing.T) {
			h := fixture(t, "Zulu\n\tChild\nBeta\nAlpha")
			command, message := "sort", "select rows that sit together to move them"
			switch place {
			case "gap":
				h.press("v", "j", "j", "v")
			case "different parents":
				h.press("j", "v", "v")
			case "list":
				h.press("t", "f", `"today"`, "enter")
				message = "a list has no order"
			case "filter", "flat filter":
				h.press("/", `"a"`)
				if place == "flat filter" {
					h.press("tab")
				}
				h.press("enter")
				message = "clear the filter first"
			case "unknown key":
				command, message = "sort @ ! bogus", "unknown sort key: bogus"
			}
			before, gen, steps, picked := h.rows(), h.m.tree.Gen(), len(h.m.undo), maps.Clone(h.m.picked)
			h.command(command)
			if !strings.Contains(h.status(), message) || h.rows() != before || h.m.tree.Gen() != gen || len(h.m.undo) != steps || !maps.Equal(picked, h.m.picked) {
				t.Fatalf("refusal changed rows or had wrong message: %q\n%s", h.status(), h.rows())
			}
		})
	}

	t.Run("Projects keeps reserved roots in place", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Zulu\nAlpha\nBeta")
		h.press("/", `"a"`, "enter")
		h.command("save letters") // Ensure the reserved filters row exists too.
		h.press("esc", "g", "g")
		inbox, filters := *h.m.tree.Row(inboxID), *h.m.tree.Row(filtersID)
		h.command("sort")
		h.want("    Alpha", "    Beta", "▌   Zulu")
		if h.m.tree.Row(inboxID).Pos != inbox.Pos || h.m.tree.Row(filtersID).Pos != filters.Pos || h.m.msg != "sorted 3 rows" {
			t.Fatal("sort included a reserved root")
		}
	})

	t.Run("empty outline", func(t *testing.T) {
		h := start(t)
		gen, steps := h.m.tree.Gen(), len(h.m.undo)
		h.command("sort")
		if h.m.msg != "nothing to sort" || h.m.tree.Gen() != gen || len(h.m.undo) != steps {
			t.Fatal("sorting an empty pane changed it")
		}
	})
}

func TestDotRepeatsTheLastStampOrMove(t *testing.T) {
	fixture := func(t *testing.T) *harness {
		h := start(t).wide()
		h.pasteText("Alpha\n\tFirst\n\tSecond\n\tThird\nBeta\n\tStorage:")
		h.press("f", `"alpha"`, "enter", "g", "g")
		return h
	}
	t.Run("stamp row and selection each undo once", func(t *testing.T) {
		h := fixture(t)
		h.press("#", `"work @+3d"`, "enter", "j", ".")
		h.want("  ○ First @wed #work", "▌ ○ Second @wed #work", "  ○ Third")
		h.press("u")
		h.want("  ○ First @wed #work", "▌ ○ Second", "  ○ Third")
		h.press("v", "v", ".")
		h.want("  ○ First @wed #work", " ┃○ Second @wed #work", "▌┃○ Third @wed #work")
		h.press("u", "esc")
		h.want("  ○ First @wed #work", "▌ ○ Second", "  ○ Third")
		// Undo and other keys retain the exact line, with dates read anew.
		h.clock = h.clock.Add(24 * time.Hour)
		h.press(".")
		if got := h.m.tree.Row(h.m.current()).Fields["due"]; got != "2026-11-19" {
			t.Fatalf("repeated date = %q", got)
		}
		h = open(t, h.path).wide()
		h.press(".")
		if h.status() != "nothing to repeat" {
			t.Fatalf("repeat survived restart: %q", h.status())
		}
	})
	t.Run("Inbox filing and undo retain named destination", func(t *testing.T) {
		h := fixture(t)
		h.press("a", `"One"`, "enter", `"Two"`, "enter", `"Three"`, "enter", "enter")
		h.press("f", `"inbox"`, "enter", "m", `"beta"`, "enter")
		for _, want := range []string{"▌ ○ Two\n  ○ Three", "▌ ○ Three", "  The Inbox is empty. Press a to capture a thought."} {
			if h.status() != "moved to Beta" || h.rows() != want {
				t.Fatalf("filing = %q\n%s, want\n%s", h.status(), h.rows(), want)
			}
			if len(h.m.tree.Children(inboxID)) != 0 {
				h.press(".")
			}
		}
		h.press("u")
		h.want("▌ ○ Three")
		h.press(".")
		if h.status() != "moved to Beta" || len(h.m.tree.Children(inboxID)) != 0 {
			t.Fatalf("repeat after undo = %q\n%s", h.status(), h.rows())
		}
		h.press("f", `"beta"`, "enter")
		h.want("▌   Storage:", "  ○ One", "  ○ Two", "  ○ Three")
	})
	for _, kind := range []string{"stamp", "move"} {
		t.Run("here and unreadable stamp retain "+kind, func(t *testing.T) {
			h := fixture(t)
			if kind == "stamp" {
				h.press("#", `"kept"`, "enter")
			} else {
				h.press("m", `"beta"`, "enter", "u")
			}
			h.press("g", "g", "v", "m", "enter", "esc") // Move First here, after Second.
			h.press("@", `"nonsense"`, "enter")
			if h.status() == "" || h.m.ask == nil {
				t.Fatal("unreadable stamp must leave its prompt open")
			}
			h.press("esc", "g", "g", ".")
			if kind == "stamp" {
				h.want("▌ ○ Second #kept", "  ○ First #kept", "  ○ Third")
			} else {
				if h.status() != "moved to Beta" {
					t.Fatalf("lost destination: %q", h.status())
				}
				h.want("▌ ○ First", "  ○ Third")
			}
		})
	}
	for _, target := range []string{"inbox", "storage"} {
		t.Run("selection appends to "+target, func(t *testing.T) {
			h := fixture(t)
			h.press("m", `"`+target+`"`, "enter", "v", "v", ".")
			label := "Inbox"
			if target == "storage" {
				label = "Beta › Storage"
			}
			if !strings.HasPrefix(h.status(), "moved to "+label+" ") {
				t.Fatalf("message = %q", h.status())
			}
			h.press("u", "esc")
			h.want("▌ ○ Second", "  ○ Third")
			h.press("f", `"`+target+`"`, "enter")
			h.want("▌ ○ First")
		})
	}
	for _, remove := range []string{"destination", "ancestor"} {
		t.Run("refuse deleted "+remove, func(t *testing.T) {
			h := fixture(t)
			h.press("m", `"storage"`, "enter")
			// A delete made here would be what the dot key repeats, so
			// the destination goes in another window on the database.
			other := open(t, h.path).wide()
			if remove == "destination" {
				other.press("f", `"beta"`, "enter", "g", "g", "d", "d")
			} else {
				other.press("f", `"projects"`, "enter", "G", "h", "h", "d", "d")
			}
			h.press("f", `"alpha"`, "enter")
			before, undo := h.rows(), len(h.m.undo)
			h.press(".")
			if h.status() != "destination is no longer live: Beta › Storage" || h.rows() != before || len(h.m.undo) != undo {
				t.Fatalf("deleted destination repeat = %q\n%s", h.status(), h.rows())
			}
		})
	}
	t.Run("cycles roll back the whole selection", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Alpha\n\tFirst\n\tStorage:\n\t\tNested:")
		h.press("f", `"alpha"`, "enter", "g", "g", "m", `"nested"`, "enter", "u")
		h.press("g", "g", "v", "v")
		before, undo := h.rows(), len(h.m.undo)
		h.press(".")
		if !strings.HasPrefix(h.status(), "a row cannot move under itself ") || h.rows() != before || len(h.m.undo) != undo {
			t.Fatalf("cycle = %q\n%s", h.status(), h.rows())
		}
	})
	for _, view := range []string{"list", "context filter", "flat filter"} {
		for _, act := range []string{"stamp", "move"} {
			t.Run(act+" in "+view, func(t *testing.T) {
				h := fixture(t)
				if view == "list" {
					// Picked first: the last change is what is repeated.
					h.command("all")
					h.press("t", "esc")
				}
				if act == "stamp" {
					h.press("#", `"work"`, "enter")
				} else {
					h.press("m", `"beta"`, "enter")
				}
				h.press("u")
				if view == "list" {
					h.press("f", `"today"`, "enter")
				} else {
					h.press("/", `"First|Second|Third"`)
					if view == "flat filter" {
						h.press("tab")
					}
					h.press("enter")
				}
				h.command("all")
				ids := h.m.acting()
				before, undo := h.rows(), len(h.m.undo)
				h.press(".")
				if len(h.m.undo) != undo+1 || len(ids) != 3 {
					t.Fatal("repeat must apply to all three rows as one undo step")
				}
				for _, id := range ids {
					r := h.m.tree.Row(id)
					if act == "stamp" && r.Fields["tags"] != "work" || act == "move" && h.m.tree.Row(r.Parent).Text() != "Beta" {
						t.Fatalf("repeat missed row: %+v", r)
					}
				}
				h.press("u")
				if h.rows() != before {
					t.Fatalf("undo =\n%s\nwant\n%s", h.rows(), before)
				}
			})
		}
	}
	t.Run("empty repeat sidebar and help bindings", func(t *testing.T) {
		h := start(t).wide()
		h.press(".")
		if h.status() != "nothing to repeat" {
			t.Fatalf("empty repeat = %q", h.status())
		}
		h.press("?")
		if h.m.help {
			t.Fatal("question mark opened help")
		}
		h.command("help")
		if !h.m.help || !strings.Contains(strings.Join(h.lines(), "\n"), "Docket") {
			t.Fatal(":help did not open help")
		}
		h.press("?") // Any previously closing key still closes help.
		if h.m.help {
			t.Fatal("question mark did not close help")
		}
		h = fixture(t)
		h.press("#", `"work"`, "enter", "u", "tab")
		before, undo := h.rows(), len(h.m.undo)
		h.press(".", "?")
		if h.m.help || h.rows() != before || len(h.m.undo) != undo || h.status() == "nothing to repeat" {
			t.Fatal("sidebar acted on dot or question mark")
		}
		h.press("tab", ".")
		h.want("▌ ○ First #work", "  ○ Second", "  ○ Third")
	})
}
