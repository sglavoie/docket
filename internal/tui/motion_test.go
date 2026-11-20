package tui

import (
	"strings"
	"testing"
)

// branches is a project of headings and tasks, the cursor on its first row:
//
//	Alpha:
//	  a1
//	  a2
//	    deep
//	  a3
//	Beta:
//	  b1 milk
//	Gamma
//	Delta milk:
func branches(t *testing.T) *harness {
	t.Helper()
	h := start(t)
	h.press("o", `"Project"`, "esc", "enter")
	h.pasteText("Alpha:\n\ta1\n\ta2\n\t\t// deep\n\ta3\nBeta:\n\tb1 milk\nGamma\nDelta milk:\n")
	h.press("g", "g")
	if len(h.m.vis) != 9 {
		t.Fatalf("fixture has %d rows:\n%s", len(h.m.vis), h.rows())
	}
	return h
}

func (h *harness) on() string { return h.m.tree.Row(h.m.current()).Text() }

func TestStructuralMotions(t *testing.T) {
	h := branches(t)
	for _, step := range []struct {
		keys string
		row  string
	}{
		{"}", "Beta:"}, {"}", "Gamma"}, {"9}", "Delta milk:"}, {"}", "Delta milk:"},
		{"{", "Gamma"}, {"2{", "Alpha:"}, {"{", "Alpha:"},
		{")", "Delta milk:"}, {"(", "Alpha:"},
		{"j}", "a2"}, {"}", "a3"}, {"}", "a3"}, {"(", "a1"}, {")", "a3"},
		{"{j", "deep"}, {"}", "deep"}, {"{", "deep"}, {")", "deep"},
		{"H", "a2"}, {"H", "Alpha:"}, {"H", "Alpha:"},
		{"3j", "deep"}, {"2H", "Alpha:"}, {"3j9H", "Alpha:"},
		{"]", "Beta:"}, {"]", "Delta milk:"}, {"]", "Delta milk:"}, {"[", "Beta:"},
		{"j[", "Beta:"}, {"gg2]", "Delta milk:"}, {"9[", "Alpha:"},
	} {
		for _, r := range step.keys {
			h.press(string(r))
		}
		if got := h.on(); got != step.row || h.m.count != 0 {
			t.Fatalf("%s landed on %q (count %d), want %q", step.keys, got, h.m.count, step.row)
		}
	}
	if len(h.m.folded) != 0 {
		t.Fatal("a motion changed a fold")
	}
}

func TestStructuralMotionsComposeWithOperatorsAndSweeps(t *testing.T) {
	t.Run("d with a sibling motion takes both branches", func(t *testing.T) {
		h := branches(t)
		h.press("d", "}")
		h.want("▌ ○ Gamma", "  · Delta milk:")
		if h.m.register != "Alpha:\n\ta1\n\ta2\n\t\t// deep\n\ta3\nBeta:\n\tb1 milk\n" {
			t.Fatalf("register = %q", h.m.register)
		}
		h.press("u")
		if len(h.m.vis) != 9 {
			t.Fatal("one undo did not restore both branches")
		}
	})
	t.Run("y to the last sibling and d to the parent", func(t *testing.T) {
		h := branches(t)
		h.press("j", "y", ")")
		if h.m.register != "a1\na2\n\t// deep\na3\n" || h.status() != "copied 4 rows" {
			t.Fatalf("register = %q, status = %q", h.m.register, h.status())
		}
		h.press("d", "H")
		h.want("  ▾ Beta: 0/1", "▌   ○ b1 milk", "  ○ Gamma", "  · Delta milk:")
	})
	t.Run("a sweep follows the motions", func(t *testing.T) {
		h := branches(t)
		h.press("j", "V", "}")
		if len(h.m.picked) != 2 {
			t.Fatalf("V} picked %d rows", len(h.m.picked))
		}
		h.press(")")
		if len(h.m.picked) != 4 || h.m.sweep == "" {
			t.Fatalf("V}) picked %d rows", len(h.m.picked))
		}
		h.press("(")
		if len(h.m.picked) != 1 {
			t.Fatalf("sweeping back kept %d rows", len(h.m.picked))
		}
		h.press("]", "x", "esc")
		h.want(
			"  ▾ Alpha: 3/3", "    ✓ a1", "    ✓ a2", "      · deep", "    ✓ a3",
			"▌ ▾ Beta: 0/1", "    ○ b1 milk", "  ○ Gamma", "  · Delta milk:",
		)
	})
	t.Run("lists step row by row and an empty place does nothing", func(t *testing.T) {
		h := branches(t)
		h.press("j", "t", "j", "t", "f", `"today"`, "enter")
		h.press("}")
		if h.m.cursor != 1 {
			t.Fatal("} did not step in a list")
		}
		h.press("H", "(")
		if h.m.cursor != 0 {
			t.Fatal("( did not reach the first row of a list")
		}
		e := start(t)
		for _, k := range []string{"}", "{", ")", "(", "H", "]", "[", "n", "N", "d", "}"} {
			e.press(k)
		}
		if e.m.cursor != 0 {
			t.Fatal("a motion moved in an empty place")
		}
	})
}

func TestFindMotions(t *testing.T) {
	t.Run("nothing remembered", func(t *testing.T) {
		h := branches(t)
		h.press("n")
		if h.on() != "Alpha:" || !strings.Contains(h.status(), "nothing to find") {
			t.Fatalf("n without a filter: %q, %q", h.on(), h.status())
		}
	})
	t.Run("a kept filter is found again in the whole outline", func(t *testing.T) {
		h := branches(t)
		h.press("/", `"milk"`, "enter")
		h.want("  ▾ Beta: 0/1", "▌   ○ b1 milk", "  · Delta milk:")
		// Under the filter n passes over the dimmed ancestor.
		h.press("k", "n")
		if h.on() != "b1 milk" {
			t.Fatalf("n under a filter landed on %q", h.on())
		}
		h.press("n", "n")
		if h.on() != "Delta milk:" || !strings.Contains(h.status(), "no match further on") {
			t.Fatalf("n at the last match: %q, %q", h.on(), h.status())
		}
		h.press("N", "esc")
		if len(h.m.vis) != 9 || h.on() != "b1 milk" {
			t.Fatalf("Esc left %d rows on %q", len(h.m.vis), h.on())
		}
		h.press("n")
		if h.on() != "Delta milk:" {
			t.Fatalf("n landed on %q", h.on())
		}
		h.press("g", "g", "2", "n")
		if h.on() != "Delta milk:" {
			t.Fatalf("2n landed on %q", h.on())
		}
		h.press("2", "N")
		if h.on() != "b1 milk" {
			t.Fatalf("2N stopped on %q", h.on())
		}
		h.press("N")
		if h.on() != "b1 milk" || !strings.Contains(h.status(), "no match further on") {
			t.Fatal("N passed the first match")
		}
	})
	t.Run("a filter dropped while typing is not remembered", func(t *testing.T) {
		h := branches(t)
		h.press("/", `"milk"`, "esc", "n")
		if !strings.Contains(h.status(), "nothing to find") {
			t.Fatalf("status = %q", h.status())
		}
	})
	t.Run("sigils, ancestors and operators", func(t *testing.T) {
		h := branches(t)
		h.press("/", `"^alpha $open"`, "enter", "esc", "g", "g")
		h.press("n")
		if h.on() != "a1" {
			t.Fatalf("n landed on %q", h.on())
		}
		h.press("9", "n")
		if h.on() != "a3" {
			t.Fatalf("9n landed on %q", h.on())
		}
		h.press("g", "g", "j", "V", "n", "n", "!")
		h.press("esc", "g", "g", "j", "d", "n")
		h.want("  ▾ Alpha: 0/1", "▌   ○ a3 !", "  ▾ Beta: 0/1", "    ○ b1 milk", "  ○ Gamma", "  · Delta milk:")
	})
}
