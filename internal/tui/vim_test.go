package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/sigil"
)

func TestFoldMotions(t *testing.T) {
	t.Run("zj and zk go to the next and previous row with rows under it", func(t *testing.T) {
		h := branches(t)
		for _, step := range []struct{ keys, row string }{
			{"zj", "a2"}, {"zj", "Beta:"}, {"zj", "Beta:"}, {"zk", "a2"}, {"9zk", "Alpha:"}, {"2zj", "Beta:"},
		} {
			for _, r := range step.keys {
				h.press(string(r))
			}
			if got := h.on(); got != step.row {
				t.Fatalf("%s landed on %q, want %q", step.keys, got, step.row)
			}
		}
		if len(h.m.folded) != 0 {
			t.Fatal("a motion changed a fold")
		}
	})
	t.Run("they sweep and complete an operator", func(t *testing.T) {
		h := branches(t)
		h.press("V", "z", "j")
		if len(h.m.picked) != 3 {
			t.Fatalf("Vzj selected %d rows, want Alpha: a1 a2", len(h.m.picked))
		}
		h.press("esc", "g", "g", "d", "z", "j")
		if len(h.m.vis) != 4 || h.on() != "Beta:" {
			t.Fatalf("dzj left %d rows on %q:\n%s", len(h.m.vis), h.on(), h.rows())
		}
	})
}

func TestFoldALevelAtATime(t *testing.T) {
	h := branches(t)
	steps := len(h.m.undo)
	for _, step := range []struct {
		keys string
		rows int
		says string
	}{
		{"zm", 8, ""}, {"zm", 4, ""}, {"zm", 4, "every row here is folded"},
		{"zr", 8, ""}, {"zr", 9, ""}, {"zr", 9, "nothing is folded here"},
		{"2zm", 4, ""}, {"5zr", 9, ""},
	} {
		h.m.msg = ""
		for _, r := range step.keys {
			h.press(string(r))
		}
		if len(h.m.vis) != step.rows || !strings.Contains(h.status(), step.says) {
			t.Fatalf("%s shows %d rows, status %q; want %d rows saying %q", step.keys, len(h.m.vis), h.status(), step.rows, step.says)
		}
	}
	if len(h.m.undo) != steps {
		t.Fatal("folding made an undo step")
	}
}

func TestBringBackTheSelection(t *testing.T) {
	t.Run("gv selects again what Esc cleared", func(t *testing.T) {
		h := branches(t)
		h.press("v", "v", "esc")
		if len(h.m.picked) != 0 {
			t.Fatal("Esc kept the selection")
		}
		h.press("g", "v")
		if !h.m.picked[h.m.vis[0].id] || !h.m.picked[h.m.vis[1].id] || len(h.m.picked) != 2 {
			t.Fatalf("gv picked %v", h.m.picked)
		}
		h.press("x")
	})
	t.Run("gv with nothing cleared says so", func(t *testing.T) {
		h := branches(t)
		h.press("g", "v")
		if !strings.Contains(h.status(), "no selection to bring back") {
			t.Fatalf("status = %q", h.status())
		}
	})
	t.Run("gv swaps with a standing selection", func(t *testing.T) {
		h := branches(t)
		h.press("v", "esc", "G", "v", "g", "v")
		if len(h.m.picked) != 1 || !h.m.picked[h.m.vis[0].id] {
			t.Fatalf("gv picked %v", h.m.picked)
		}
		h.press("g", "v")
		if len(h.m.picked) != 1 || !h.m.picked[h.m.vis[8].id] {
			t.Fatalf("a second gv picked %v", h.m.picked)
		}
	})
}

func TestSwapTheEndsOfASweep(t *testing.T) {
	h := branches(t)
	h.press("j", "V", "2", "j", "o")
	if h.on() != "a1" || len(h.m.picked) != 3 || len(h.m.vis) != 9 {
		t.Fatalf("o left the cursor on %q with %d picked and %d rows", h.on(), len(h.m.picked), len(h.m.vis))
	}
	h.press("k")
	if len(h.m.picked) != 4 || !h.m.picked[h.m.vis[3].id] {
		t.Fatalf("moving the other end picked %d rows", len(h.m.picked))
	}
	h.press("V")
	if h.m.sweep != "" || len(h.m.picked) != 4 {
		t.Fatal("V did not keep the swept rows")
	}
}

func TestTypeAgainWhereTypingEnded(t *testing.T) {
	h := textRow(t, "one two")
	h.press("O", `"top"`, "esc", "j", "w", "a", `"X"`, "esc", "esc", "k")
	if h.on() != "top" || h.m.edit != nil {
		t.Fatalf("setup left the cursor on %q", h.on())
	}
	h.press("g", "i")
	if got, at := h.typing(); got != "oXne two" || at != 2 || h.m.edit.normal {
		t.Fatalf("gi is typing %q at %d (normal %v)", got, at, h.m.edit.normal)
	}
	h.press(`"Y"`, "esc")
	if h.on() != "oXYne two" || h.m.edit != nil {
		t.Fatalf("gi typed into %q", h.on())
	}
	h.press("d", "d", "g", "i")
	if !strings.Contains(h.status(), "the row last typed in is gone") {
		t.Fatalf("status = %q", h.status())
	}
	if h := start(t); h.press("g", "i") != nil && !strings.Contains(h.status(), "nothing typed yet") {
		t.Fatalf("status = %q", h.status())
	}
}

func TestBackToAWordsEnd(t *testing.T) {
	h := textRow(t, "one two three")
	h.press("b", "g", "e")
	h.wantTyping("one two three", 6)
	h.press("g", "e")
	h.wantTyping("one two three", 2)
	h.press("w", "d", "g", "e")
	h.wantTyping("onwo three", 2)

	h = textRow(t, "a-b c")
	h.press("b", "g", "e")
	h.wantTyping("a-b c", 2)
	h.press("g", "e")
	h.wantTyping("a-b c", 1)
	h.press("$", "g", "E")
	h.wantTyping("a-b c", 2)
}

func TestMatchingBracket(t *testing.T) {
	h := textRow(t, "do (a (b) c) now")
	h.press("w", "%")
	h.wantTyping("do (a (b) c) now", 11)
	h.press("%")
	h.wantTyping("do (a (b) c) now", 3)
	h.press("5", "l", "%")
	h.wantTyping("do (a (b) c) now", 6)
	h.press("0", "d", "%")
	h.wantTyping(" now", 0)
}

func TestCaseOperators(t *testing.T) {
	h := textRow(t, "one two")
	h.press("w", "g", "U", "w")
	h.wantTyping("ONE two", 0)
	h.press("w", "g", "~", "e")
	h.wantTyping("ONE TWO", 4)
	h.press("g", "u", "u")
	h.wantTyping("one two", 0)
	h.press("g", "U", "g", "U")
	h.wantTyping("ONE TWO", 0)
	h.press("g", "u", "i", "w")
	h.wantTyping("one TWO", 0)
	if !strings.Contains(h.press("g", "U").status(), "gUw gUe gU$ through a motion") {
		t.Fatalf("a waiting gU said %q", h.status())
	}
	h.press("esc", "esc")
	if h.on() != "one TWO" {
		t.Fatalf("the row kept %q", h.on())
	}
	h.press("u")
	if h.on() != "one two" {
		t.Fatalf("one undo left %q", h.on())
	}
}

func TestCaseOfASelection(t *testing.T) {
	h := textRow(t, "one two")
	h.press("w", "v", "e", "U")
	h.wantTyping("ONE two", 0)
	if h.m.edit.sel {
		t.Fatal("U did not end the selection")
	}
	h.press("w", ".")
	h.wantTyping("ONE TWO", 4)
	h.press("0", "v", "$", "u")
	h.wantTyping("one two", 0)
	h.press("v", "e", "g", "~")
	h.wantTyping("ONE two", 0)
}

func TestAddToANumber(t *testing.T) {
	h := textRow(t, "call 3 times")
	h.press("w", "ctrl+a")
	h.wantTyping("call 4 times", 5)
	h.press("5", "ctrl+x")
	h.wantTyping("call -1 times", 6)
	h.press("ctrl+a")
	h.wantTyping("call 0 times", 5)
	h.press("0", ".")
	h.wantTyping("call 1 times", 5)

	h = textRow(t, "due 2026-11-09")
	h.press("w", "$", "ctrl+a")
	h.wantTyping("due 2026-11-10", 13)
	h.press("u", "9", "ctrl+x")
	h.wantTyping("due 2026-11-00", 13)

	h = textRow(t, "no digits")
	h.press("w", "ctrl+a")
	h.wantTyping("no digits", 0)
	if !strings.Contains(h.status(), "no number at or after the caret") {
		t.Fatalf("status = %q", h.status())
	}
}

func TestCaseAndBracketsInABody(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"ab (c"`, "enter", `"d) ef"`, "esc", "g", "g", "%")
	if got, at := h.typing(); got != "ab (c\nd) ef" || at != 7 {
		t.Fatalf("%% in the body: %q at %d", got, at)
	}
	h.press("g", "U", "U")
	if got, _ := h.typing(); got != "ab (c\nD) EF" {
		t.Fatalf("gUU in the body: %q", got)
	}
	h.press("g", "g", "g", "U", "j")
	if got, _ := h.typing(); got != "AB (C\nD) EF" {
		t.Fatalf("gUj in the body: %q", got)
	}
	h.press("j", "$", "g", "e", "g", "e", "g", "e")
	if got, at := h.typing(); at != 4 {
		t.Fatalf("ge in the body: %q at %d", got, at)
	}
	h.press("esc")
	if h.body() != "AB (C\nD) EF" {
		t.Fatalf("body is %q", h.body())
	}
}

func TestMoveTheDayARowHas(t *testing.T) {
	h := inProject(t)
	h.press("A", `" @2026-11-20"`, "esc", "j", "A", `" @@2026-11-25"`, "esc", "k")
	day := func(row int, field string) string { return h.m.tree.Row(h.m.vis[row].id).Fields[field] }
	check := func(keys, due, start string) {
		t.Helper()
		if d, s := day(0, sigil.FieldDue), day(1, sigil.FieldStart); d != due || s != start {
			t.Fatalf("after %s: due %s, start %s; want %s, %s", keys, d, s, due, start)
		}
	}
	h.press("ctrl+a")
	check("Ctrl+A", "2026-11-21", "2026-11-25")
	if h.status() != "moved 1 day later" {
		t.Fatalf("status = %q", h.status())
	}
	h.press("3", "ctrl+x")
	check("3 Ctrl+X", "2026-11-18", "2026-11-25")
	h.press("v", "v", "ctrl+a")
	check("Ctrl+A on both", "2026-11-19", "2026-11-26")
	steps := len(h.m.undo)
	h.press("esc", "g", "g", "5", "ctrl+a", ".")
	check("5 Ctrl+A .", "2026-11-29", "2026-11-26")
	h.press("u")
	check("u", "2026-11-24", "2026-11-26")
	if len(h.m.undo) != steps+1 {
		t.Fatal("a counted Ctrl+A was not one undo step")
	}
	h.press("G", "o", `"Third"`, "esc", "ctrl+a")
	if !strings.Contains(h.status(), "no day to move") {
		t.Fatalf("status = %q", h.status())
	}
}
