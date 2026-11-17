package tui

import (
	"strings"
	"testing"
)

// textRow is a project open on one task holding line, the cursor on it.
func textRow(t *testing.T, line string) *harness {
	t.Helper()
	h := start(t).wide()
	h.press("o", `"Project"`, "esc", "enter", "o", `"`+line+`"`, "esc")
	return h
}

// typing is the line being edited and where its caret is.
func (h *harness) typing() (string, int) {
	h.t.Helper()
	if h.m.edit == nil {
		h.t.Fatal("no row is open")
	}
	return string(h.m.edit.buf), h.m.edit.caret
}

func (h *harness) wantTyping(line string, caret int) {
	h.t.Helper()
	if got, at := h.typing(); got != line || at != caret || !h.m.edit.normal {
		h.t.Fatalf("in the row: %q caret %d (inside %v), want %q caret %d", got, at, h.m.edit.normal, line, caret)
	}
}

func TestWordMotionsStepIntoTheRow(t *testing.T) {
	t.Run("a count lands on that word and cw changes it in one undo step", func(t *testing.T) {
		h := textRow(t, "Buy oat milk today")
		steps := len(h.m.undo)
		h.press("3", "w")
		h.wantTyping("Buy oat milk today", 8)
		if !strings.HasPrefix(h.status(), "in the row") {
			t.Fatalf("status = %q", h.status())
		}
		if c := h.m.View().Cursor; c == nil || c.Shape != 0 {
			t.Fatalf("the caret inside a row is not a block: %+v", c)
		}
		// Typing opened inside the row ends inside it: the caret is back on
		// a character, and a second Esc leaves.
		h.press("c", "w", `"soy"`, "esc")
		h.wantTyping("Buy oat soy today", 10)
		h.press("esc")
		h.want("▌ ○ Buy oat soy today")
		if h.m.edit != nil || len(h.m.undo) != steps+1 {
			t.Fatal("Esc did not keep the row as one undo step")
		}
		h.press("u")
		h.want("▌ ○ Buy oat milk today")
	})

	t.Run("each word motion enters from its own side", func(t *testing.T) {
		h := textRow(t, "Buy oat milk today")
		for _, tc := range []struct {
			keys  []string
			caret int
		}{
			{[]string{"w"}, 0}, {[]string{"2", "w"}, 4}, {[]string{"9", "w"}, 17},
			{[]string{"e"}, 2}, {[]string{"2", "e"}, 6},
			{[]string{"b"}, 13}, {[]string{"2", "b"}, 8}, {[]string{"9", "b"}, 0},
			{[]string{"W"}, 0}, {[]string{"E"}, 2}, {[]string{"B"}, 13},
		} {
			h.press(tc.keys...)
			h.wantTyping("Buy oat milk today", tc.caret)
			h.press("esc")
		}
		if len(h.m.undo) != 2 {
			t.Fatal("stepping in and out made undo steps")
		}
	})

	t.Run("motions inside the row", func(t *testing.T) {
		h := textRow(t, "alpha beta-gamma delta")
		h.press("w")
		for _, tc := range []struct {
			keys  string
			caret int
		}{
			{"w", 6}, {"w", 10}, {"w", 11}, {"W", 17}, {"w", 21}, {"b", 17}, {"B", 6}, {"e", 9}, {"E", 15},
			{"0", 0}, {"$", 21}, {"^", 0}, {"3l", 3}, {"h", 2}, {"9h", 0}, {"fa", 4}, {";", 9}, {";", 12}, {",", 9},
			{"ta", 11}, {"Fb", 6}, {"Tl", 2}, {"fz", 2}, {"2fa", 9},
		} {
			for _, r := range tc.keys {
				h.press(string(r))
			}
			h.wantTyping("alpha beta-gamma delta", tc.caret)
		}
	})

	t.Run("acts on characters, and u inside the row", func(t *testing.T) {
		h := textRow(t, "Buy soy milk")
		h.press("w", "x")
		h.wantTyping("uy soy milk", 0)
		h.press("r", "B")
		h.wantTyping("By soy milk", 0)
		h.press("~")
		h.wantTyping("by soy milk", 1)
		h.press("u")
		h.wantTyping("By soy milk", 0)
		h.press("u")
		h.wantTyping("uy soy milk", 0)
		h.press("ctrl+r")
		h.wantTyping("By soy milk", 0)
		h.press("f", "m", "D")
		h.wantTyping("By soy ", 6)
		h.press("x")
		h.wantTyping("By soy", 5)
		h.press("2", "X")
		h.wantTyping("By y", 3)
		h.press("u", "u", "u", "u", "u", "u")
		h.wantTyping("Buy soy milk", 0)
		if !strings.HasPrefix(h.status(), "nothing to undo inside this row") {
			t.Fatalf("status = %q", h.status())
		}
		h.press("3", "x", "esc")
		h.want("▌ ○ soy milk")
	})

	t.Run("operators take motions and text objects", func(t *testing.T) {
		h := textRow(t, `Call "Ann Lee" (urgent, today) now`)
		h.press("w", "f", "L", "c", "i", `"`, `"Bob"`, "esc", "esc")
		h.want(`▌ ○ Call "Bob" (urgent, today) now`)
		h.press("w", "f", "u", "d", "i", "(")
		h.wantTyping(`Call "Bob" () now`, 12)
		h.press("d", "a", "b")
		h.wantTyping(`Call "Bob"  now`, 11)
		h.press("d", "B")
		h.wantTyping(`Call  now`, 5)
		h.press("w", "d", "a", "w")
		h.wantTyping(`Call`, 3)
		h.press("y", "i", "w", "P")
		h.wantTyping(`CallCall`, 3)
		if h.m.textRegister != "Call" {
			t.Fatalf("text register = %q", h.m.textRegister)
		}
		h.press("0", "d", "t", "l", "d", "$")
		h.wantTyping(``, 0)
		h.press("u", "u")
		h.wantTyping(`CallCall`, 0)
		// A doubled operator belongs to the outline: here it only cancels.
		h.press("d", "d", "c", "c", "y", "y", "d", "esc", "2", "esc")
		h.wantTyping(`CallCall`, 0)
		h.press("esc")
		h.want("▌ ○ CallCall")
	})

	t.Run("the dot key repeats the last change inside a row", func(t *testing.T) {
		h := textRow(t, "one two three four")
		steps := len(h.m.undo)
		h.press("w", ".")
		if !strings.HasPrefix(h.status(), "nothing to repeat inside a row") {
			t.Fatalf("status = %q", h.status())
		}
		h.press("c", "i", "w", `"X"`, "esc", "esc")
		h.want("▌ ○ X two three four")
		h.press("2", "w", ".")
		h.wantTyping("X X three four", 2)
		h.press("w", ".", "u")
		h.wantTyping("X X three four", 4)
		h.press("d", "w", ".", "0", ".")
		h.wantTyping("X ", 0)
		h.press("esc")
		h.want("▌ ○ X")
		if len(h.m.undo) != steps+2 {
			t.Fatal("each visit to the row should be one undo step")
		}
	})

	t.Run("sigils are words of the line", func(t *testing.T) {
		h := textRow(t, "Ship it #work @2026-11-20")
		h.press("B")
		h.wantTyping("Ship it @2026-11-20 #work", 20)
		h.press("d", "a", "W", "esc")
		h.want("▌ ○ Ship it @fri")
		h.press("B", "c", "W", `"@tom"`, "esc", "esc")
		h.want("▌ ○ Ship it @tomorrow")
	})

	t.Run("i a I A open typing, Esc returns to the caret and Esc again to the outline", func(t *testing.T) {
		for _, tc := range []struct {
			key, want string
			caret     int
		}{
			{"i", "one _two", 4}, {"a", "one t_wo", 5}, {"I", "_one two", 0}, {"A", "one two_", 7},
		} {
			h := textRow(t, "one two")
			steps := len(h.m.undo)
			h.press("2", "w", tc.key)
			if !strings.HasPrefix(h.status(), "Esc back to the caret") {
				t.Fatalf("status = %q", h.status())
			}
			h.press("_", "esc")
			h.wantTyping(tc.want, tc.caret)
			h.press("esc")
			h.want("▌ ○ " + tc.want)
			if h.m.edit != nil || len(h.m.undo) != steps+1 {
				t.Fatal("the second Esc did not leave the row as one undo step")
			}
		}
	})

	t.Run("a row takes several changes on one visit", func(t *testing.T) {
		h := textRow(t, "one two three four")
		steps := len(h.m.undo)
		h.press("w", "c", "w", `"1"`, "esc", "w", "c", "w", `"2"`, "esc", "w", ".", "$", "a", `"!"`, "esc")
		h.wantTyping("1 2 2 four!", 10)
		// u inside the row takes back each change, typing included.
		h.press("u")
		h.wantTyping("1 2 2 four", 9)
		h.press("u", "u")
		h.wantTyping("1 two three four", 2)
		h.press("ctrl+r", "i", "esc")
		h.wantTyping("1 2 three four", 3)
		h.press("u")
		h.wantTyping("1 two three four", 2)
		h.press("esc")
		h.want("▌ ○ 1 two three four")
		if len(h.m.undo) != steps+1 {
			t.Fatal("one visit should be one undo step")
		}
		// Typing opened from the outline still ends in the outline.
		h.press("A", `"."`, "esc")
		if h.m.edit != nil {
			t.Fatal("Esc after A stayed in the row")
		}
	})

	t.Run("selections are refused; lists and filters are not", func(t *testing.T) {
		h := textRow(t, "one two")
		h.press("v", "k", "w")
		if h.m.edit != nil || !strings.HasPrefix(h.status(), "that works on one row") {
			t.Fatalf("a selection was entered: %q", h.status())
		}
		h.press("esc", "t", "f", `"today"`, "enter", "w", "x", "esc")
		h.want("▌ ○ ne two · Project")
		h.press("/", `"two"`, "enter", "e", "s", `"N"`, "esc", "esc")
		if got := h.m.tree.Row(h.m.current()).Text(); got != "nN two" {
			t.Fatalf("text = %q", got)
		}
	})

	t.Run("j and k move by drawn line, then into the next row", func(t *testing.T) {
		h := start(t)
		h.press("o", `"Project"`, "esc", "enter", "o", `"`+strings.Repeat("word ", 12)+`"`, "esc", "o", `"Below"`, "esc", "k")
		h.press("w", "j")
		if _, at := h.typing(); at == 0 {
			t.Fatal("j did not move down a drawn line")
		}
		h.press("9", "j")
		if !h.m.edit.normal || h.m.tree.Row(h.m.edit.row).Text() != "Below" {
			t.Fatal("j did not go on into the row below")
		}
		h.press("j", "k")
		if line, at := h.typing(); !strings.HasPrefix(line, "word") || at < 30 {
			t.Fatalf("k did not come back to the last line of the row above: %q %d", line, at)
		}
		h.press("9", "k")
		if line, at := h.typing(); !strings.HasPrefix(line, "word") || at > 30 {
			t.Fatalf("k did not climb to the first line and stop: %q %d", line, at)
		}
		// A selection stays in its row.
		h.press("$", "v", "9", "j")
		if h.m.tree.Row(h.m.edit.row).Text() == "Below" {
			t.Fatal("a selection left the row")
		}
	})
}

func TestDotRepeatsEveryChangeToRows(t *testing.T) {
	t.Run("keys that act on rows, with the count they were given", func(t *testing.T) {
		h := countOutline(t, 6)
		h.press("x", "j", ".")
		h.want("  ✓ Task 00", "▌ ✓ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04", "  ○ Task 05")
		steps := len(h.m.undo)
		h.press("j", "2", "!", "2", "j", ".")
		h.want("  ✓ Task 00", "  ✓ Task 01", "  ○ Task 02 !", "  ○ Task 03 !", "▌ ○ Task 04 !", "  ○ Task 05 !")
		if len(h.m.undo) != steps+2 {
			t.Fatal("a repeated count should be one undo step")
		}
		// A count given to the dot replaces the remembered one.
		h.press("g", "g", "1", ".")
		h.want("▌ ✓ Task 00 !", "  ✓ Task 01", "  ○ Task 02 !", "  ○ Task 03 !", "  ○ Task 04 !", "  ○ Task 05 !")
		h.press("u", "t", "j", ".")
		if h.m.tree.Row(h.m.current()).Fields["today"] == "" {
			t.Fatal("the dot did not repeat t")
		}
	})

	t.Run("operators with what completed them", func(t *testing.T) {
		h := countOutline(t, 8)
		h.press("d", "d", ".")
		h.want("▌ ○ Task 02", "  ○ Task 03", "  ○ Task 04", "  ○ Task 05", "  ○ Task 06", "  ○ Task 07")
		h.press("d", "j", ".")
		h.want("▌ ○ Task 06", "  ○ Task 07")
		h.press("u", "u", "u", "u")
		h.press("g", "g", "j", ">", ">", "j", ".")
		rest := []string{"  ○ Task 03", "  ○ Task 04", "  ○ Task 05", "  ○ Task 06", "  ○ Task 07"}
		h.want(append([]string{"  ○ Task 00", "    ○ Task 01", "▌   ○ Task 02"}, rest...)...)
		h.press("<", "<", "k", ".")
		h.want(append([]string{"  ○ Task 00", "▌ ○ Task 01", "  ○ Task 02"}, rest...)...)
	})

	t.Run("reorder and put", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("J", ".")
		h.want("  ○ Task 01", "  ○ Task 02", "▌ ○ Task 00", "  ○ Task 03")
		h.press("y", "y", "p", ".")
		h.want("  ○ Task 01", "  ○ Task 02", "  ○ Task 00", "  ○ Task 00", "▌ ○ Task 00", "  ○ Task 03")
		// A copy changes nothing, so the put is still what is repeated.
		h.press("G", "y", "y", ".")
		h.want("  ○ Task 01", "  ○ Task 02", "  ○ Task 00", "  ○ Task 00", "  ○ Task 00", "  ○ Task 03", "▌ ○ Task 03")
	})

	t.Run("typed text: A I cc o O", func(t *testing.T) {
		h := countOutline(t, 3)
		h.press("A", `" now #home"`, "esc", "j", ".")
		h.want("  ○ Task 00 now #home", "▌ ○ Task 01 now #home", "  ○ Task 02")
		steps := len(h.m.undo)
		h.press("g", "g", "I", `"Do "`, "esc", "v", "v", "v", ".")
		if len(h.m.undo) != steps+2 {
			t.Fatal("typing into a selection should be one undo step")
		}
		h.press("esc")
		h.want("  ○ Do Do Task 00 now #home", "  ○ Do Task 01 now #home", "▌ ○ Do Task 02")
		h.press("c", "c", `"Same"`, "esc", "k", ".")
		h.want("  ○ Do Do Task 00 now #home", "▌ ○ Same #home", "  ○ Same")
		h.press("o", `"Next @tom"`, "esc", ".", "O", `"Above"`, "esc", ".")
		h.want("  ○ Do Do Task 00 now #home", "  ○ Same #home", "  ○ Next @tomorrow", "▌ ○ Above", "  ○ Above", "  ○ Next @tomorrow", "  ○ Same")
		// Typing that did more than add text where it began is not repeated.
		h.press("A", "backspace", "backspace", `"x"`, "esc", "j", ".")
		h.want("  ○ Do Do Task 00 now #home", "  ○ Same #home", "  ○ Next @tomorrow", "  ○ Abox", "▌ ○ Above", "  ○ Above", "  ○ Next @tomorrow", "  ○ Same")
	})

	t.Run("undo, a copy, a motion and a refused key leave it remembered", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("x", "u", "j", "y", "y", "G", "x", "u")
		h.press("g", "g", "o", "esc", ".")
		h.want("▌ ✓ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03")
	})
}

func TestIndentAndOutdentAreOperators(t *testing.T) {
	h := countOutline(t, 5)
	h.press("j", ">")
	if h.m.pending() != ">" {
		t.Fatalf("pending operator = %q", h.m.pending())
	}
	h.press("j")
	h.want("  ○ Task 00", "▌   ○ Task 01", "    ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	if len(h.m.undo) == 0 {
		t.Fatal("no undo step")
	}
	h.press("<", ")")
	h.want("  ○ Task 00", "▌ ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	steps := len(h.m.undo)
	h.press("2", ">", ">")
	h.want("  ○ Task 00", "▌   ○ Task 01", "    ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	if len(h.m.undo) != steps+1 {
		t.Fatal("a counted indent should be one undo step")
	}
	// Another key cancels the operator and does nothing.
	gen := h.m.tree.Gen()
	h.press(">", "x", "<", "esc")
	if h.m.tree.Gen() != gen || h.m.operator != "" {
		t.Fatal("a canceled operator acted")
	}
	h.press("V", "j", ">", "j")
	if !strings.HasPrefix(h.status(), "a motion cannot act on a selection") {
		t.Fatalf("status = %q", h.status())
	}
	h.press("<", "<")
	h.want("  ○ Task 00", " ┃○ Task 01", "▌┃○ Task 02", "  ○ Task 03", "  ○ Task 04")
}

func places(t *testing.T) *harness {
	t.Helper()
	h := start(t).wide()
	h.pasteText("Alpha\n\tHeading:\n\t\tBuy milk\n\t\tSell milk\n\tCall Ann\nBeta\n\tBuy bread\n")
	return h
}

func TestGoToReachesAnyRow(t *testing.T) {
	h := places(t)
	h.press("f", `"milk"`)
	if !strings.Contains(h.bottom(), "Buy milk (Alpha › Heading)") || !strings.Contains(h.bottom(), "Sell milk") {
		t.Fatalf("offers = %q", h.bottom())
	}
	h.press("enter")
	h.want("▌ ○ Buy milk", "  ○ Sell milk")
	if h.header() != "Alpha › Heading  · 2 to do" || h.m.sel.kind != kindProject {
		t.Fatalf("the row did not open inside its project: %q", h.header())
	}
	h.press("esc")
	if h.header() != "Alpha  · 3 to do" {
		t.Fatalf("Esc backed out to %q", h.header())
	}
	h.press("ctrl+o")
	if h.header() != "Projects  · 4 to do" {
		t.Fatalf("Ctrl+O returned to %q", h.header())
	}
	// Places come before rows that match as well, and Down walks on.
	h.press("f", `"b"`)
	if !strings.Contains(h.bottom(), "Beta · Buy milk") {
		t.Fatalf("offers = %q", h.bottom())
	}
	h.press("down", "down", "enter")
	h.want("▌ ○ Buy bread")
	if h.header() != "Beta  · 1 to do" {
		t.Fatalf("header = %q", h.header())
	}
	// A row must hold what is typed; a place may answer to its letters.
	h.press("f", `"bmk"`)
	if !strings.Contains(h.bottom(), "nothing matches") {
		t.Fatalf("letters in order offered a row: %q", h.bottom())
	}
	h.press("esc", "f", `"alp"`, "enter")
	if h.header() != "Alpha  · 3 to do" {
		t.Fatalf("header = %q", h.header())
	}
	// A closed row is shown even while closed rows are hidden.
	h.press("G", "x")
	h.command("closed")
	h.press("f", `"beta"`, "enter", "f", `"call"`, "enter")
	h.want("  ▾ Heading: 0/2", "    ○ Buy milk", "    ○ Sell milk", "▌ ✓ Call Ann")
	// The move picker still offers only places.
	h.press("m", `"milk"`)
	if !strings.Contains(h.bottom(), "nothing matches") {
		t.Fatalf("move offers = %q", h.bottom())
	}
}

func TestARowLinksToAnother(t *testing.T) {
	h := places(t)
	h.press("f", `"bread"`, "enter", "o", `"Eggs"`, "esc", "k")
	steps := len(h.m.undo)
	h.press("g", "l")
	if !strings.HasPrefix(h.status(), "this row has no link") {
		t.Fatalf("status = %q", h.status())
	}
	h.press("L", `"buy m"`, "enter")
	h.want("▌ ○ Buy bread → Buy milk", "  ○ Eggs")
	if h.status() != "linked to Buy milk: gl follows it" || len(h.m.undo) != steps+1 {
		t.Fatalf("status = %q", h.status())
	}
	// A link is made again with the dot.
	h.press("j", ".", "k")
	h.want("▌ ○ Buy bread → Buy milk", "  ○ Eggs → Buy milk")
	h.press("g", "l")
	h.want("▌ ○ Buy milk ← 2", "  ○ Sell milk")
	if h.header() != "Alpha › Heading  · 2 to do" {
		t.Fatalf("header = %q", h.header())
	}
	// Two rows link here, so gl names the filter that lists them.
	h.press("g", "l")
	if h.status() != "2 rows link here: /&. lists them" {
		t.Fatalf("status = %q", h.status())
	}
	// The link reads as the row reads now, and following it was a jump.
	h.press("A", `" today"`, "esc", "ctrl+o")
	h.want("▌ ○ Buy bread → Buy milk today", "  ○ Eggs → Buy milk today")
	h.press("j", "L", `"eggs"`, "enter")
	if h.status() != "a row cannot link to itself" {
		t.Fatalf("status = %q", h.status())
	}
	// A blank Link to offers none, which takes the link off.
	h.press("L")
	if !strings.Contains(h.bottom(), "none") {
		t.Fatalf("offers = %q", h.bottom())
	}
	h.press("enter")
	h.want("  ○ Buy bread → Buy milk today", "▌ ○ Eggs")
	// A link to a row that is gone says so.
	h.press("k", "g", "l", "d", "d", "ctrl+o")
	h.want("▌ ○ Buy bread → gone", "  ○ Eggs")
	h.press("g", "l")
	if h.status() != "the linked row is gone" {
		t.Fatalf("status = %q", h.status())
	}
	h.press("u")
	h.want("▌ ○ Buy bread → Buy milk today", "  ○ Eggs")
}

func TestALinkedRowLeadsBack(t *testing.T) {
	h := places(t)
	h.press("f", `"bread"`, "enter", "L", `"buy m"`, "enter")
	h.press("f", `"call"`, "enter")
	// The row linked to names the one row that links to it.
	h.press("f", `"buy m"`, "enter")
	h.want("▌ ○ Buy milk ← Buy bread", "  ○ Sell milk")
	steps := len(h.m.undo)
	h.press("g", "l")
	h.want("▌ ○ Buy bread → Buy milk")
	if len(h.m.undo) != steps {
		t.Fatal("going back made an undo step")
	}
	// Going back was a jump.
	h.press("ctrl+o")
	h.want("▌ ○ Buy milk ← Buy bread", "  ○ Sell milk")
	// A row's own link wins over the way back.
	h.press("L", `"call"`, "enter", "g", "l")
	if h.header() != "Alpha  · 3 to do" {
		t.Fatalf("header = %q", h.header())
	}
	// Once the link is gone, so is the way back.
	h.press("ctrl+o", "L", "enter", "f", `"bread"`, "enter", "L", "enter")
	h.press("f", `"buy m"`, "enter")
	h.want("▌ ○ Buy milk", "  ○ Sell milk")
	h.press("g", "l")
	if h.status() != "this row has no link: L makes one" {
		t.Fatalf("status = %q", h.status())
	}
}

func TestNumbersCountFromTheCursor(t *testing.T) {
	h := countOutline(t, 4)
	h.command("numbers")
	h.want("   ▌ ○ Task 00", " 1   ○ Task 01", " 2   ○ Task 02", " 3   ○ Task 03")
	h.press("2", "j")
	h.want(" 2   ○ Task 00", " 1   ○ Task 01", "   ▌ ○ Task 02", " 1   ○ Task 03")
	// The number is the count: d2k takes the cursor's row and two above.
	h.press("d", "2", "k")
	h.want("   ▌ ○ Task 03")
	h.press("u")
	// Typing keeps its place beside the numbers.
	h.press("A")
	if c := h.m.View().Cursor; c == nil || c.X != numberCols+4+len("Task 00") {
		t.Fatalf("caret = %+v", c)
	}
	h.press("esc")
	h = open(t, h.path)
	h.press("enter")
	h.want("   ▌ ○ Task 00", " 1   ○ Task 01", " 2   ○ Task 02", " 3   ○ Task 03")
	h.command("numbers")
	h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03")
}

func TestAFilterTestsTheRowsBelow(t *testing.T) {
	h := work(t)
	h.press("/", `"_#bug"`, "tab", "enter")
	h.want("▌ · Beta +1", "  · Alpha +3")
	h.press("/", "ctrl+u", `"_@"`, "enter")
	h.want("▌ · Alpha +3")
	h.press("/", "ctrl+u", `"-_#docs"`, "enter")
	h.want("▌ · Beta +1", "  ○ Fix cache #bug · Beta", "  ○ Fix login ! #bug · Alpha", "  ○ Write docs @fri #docs · Alpha", "  ○ Refactor auth #bug · Alpha")
	// Rows below at any depth count, and every term must hold.
	h.press("esc", "G", "o", `"Deep #docs"`, "tab", "esc")
	h.press("/", `"_#docs|bug -_!"`, "enter")
	h.want("▌ · Beta +1", "  ○ Refactor auth #bug +1 · Alpha")
	// In context a match keeps its ancestors, and n finds it afterwards.
	h.press("/", "ctrl+u", `"_#docs alpha"`, "tab", "enter", "esc", "g", "g", "n")
	if h.on() != "Alpha" {
		t.Fatalf("n landed on %q", h.on())
	}
	// A bare underscore waits for typing and a doubled one is text.
	h.press("/", `"_"`)
	if !strings.Contains(h.bottom(), "no match") {
		t.Fatalf("bare underscore = %q", h.bottom())
	}
	h.press(`"#bug"`)
	if !strings.Contains(h.bottom(), "2 match") {
		t.Fatalf("typed underscore term = %q", h.bottom())
	}
	h.press("esc", "/", `"__x"`)
	if !strings.Contains(h.bottom(), "no match") {
		t.Fatalf("doubled underscore = %q", h.bottom())
	}
}

func TestATagCarriesAValue(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Project"`, "esc", "enter")
	h.press("o", `"Small #est=2 #who=Ann"`, "enter", `"Large #est=10"`, "enter", `"Rough #est"`, "enter", `"None"`, "esc", "g", "g")
	h.want("▌ ○ Small #est=2 #who=Ann", "  ○ Large #est=10", "  ○ Rough #est", "  ○ None")
	for query, want := range map[string]string{
		"#est":     "Small Large Rough",
		"#est=2":   "Small",
		"#est>3":   "Large",
		"#est<2":   "Small",
		"#est>2":   "Small Large",
		"-#est":    "None",
		"#who>ann": "Small",
		"#who<amy": "",
		"#est=":    "",
	} {
		h.press("/", "ctrl+u", `"`+query+`"`, "enter")
		var got []string
		for _, v := range h.m.vis {
			got = append(got, strings.Fields(h.m.tree.Row(v.id).Text())[0])
		}
		if strings.Join(got, " ") != want {
			t.Fatalf("%s matched %v, want %s", query, got, want)
		}
		h.press("esc")
	}
	// A name is held once: a new value replaces the old where it sits.
	h.press("g", "g", "#", `"est=5"`, "enter")
	h.want("▌ ○ Small #est=5 #who=Ann", "  ○ Large #est=10", "  ○ Rough #est", "  ○ None")
	h.press("A", `" #who=Bo"`, "esc")
	h.want("▌ ○ Small #est=5 #who=Bo", "  ○ Large #est=10", "  ○ Rough #est", "  ○ None")
	// -#name takes the tag off whatever its value; -#name=value only that.
	h.command("all")
	h.press("-", `"#est=10"`, "enter")
	h.want("▌┃○ Small #est=5 #who=Bo", " ┃○ Large", " ┃○ Rough #est", " ┃○ None")
	h.press("-", `"#est"`, "enter", "esc")
	h.want("▌ ○ Small #who=Bo", "  ○ Large", "  ○ Rough", "  ○ None")
	// The line a row opens with spells the value back.
	h.press("A")
	if line, _ := h.typing(); line != "Small #who=Bo" {
		t.Fatalf("line = %q", line)
	}
}
