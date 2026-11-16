package tui

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

func countOutline(t *testing.T, n int) *harness {
	t.Helper()
	h := start(t)
	h.press("o", `"Project"`, "esc", "enter")
	for i := 0; i < n; i++ {
		h.press("o", fmt.Sprintf(`"Task %02d"`, i), "esc")
	}
	h.press("g", "g")
	return h
}

func TestACountRepeatsOrTakesRows(t *testing.T) {
	t.Run("motions and boundaries", func(t *testing.T) {
		h := countOutline(t, 12)
		for _, step := range []struct {
			keys []string
			row  int
		}{
			{[]string{"3", "j"}, 3},
			{[]string{"2", "k"}, 1},
			{[]string{"1", "0", "j"}, 11},
			{[]string{"9", "j"}, 11},
			{[]string{"2", "up"}, 9},
			{[]string{"3", "down"}, 11},
			{[]string{"2", "g", "g"}, 0},
			{[]string{"2", "ctrl+d"}, 10},
			{[]string{"2", "ctrl+u"}, 0},
			{[]string{"2", "ctrl+f"}, 11},
			{[]string{"2", "ctrl+b"}, 0},
			{[]string{"2", "G"}, 11},
			{[]string{"9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "k"}, 0},
		} {
			h.press(step.keys...)
			if got := h.m.tree.Row(h.m.current()).Text(); got != fmt.Sprintf("Task %02d", step.row) {
				t.Fatalf("%v landed on %s, want %d", step.keys, got, step.row)
			}
		}
		for _, step := range []struct {
			code rune
			row  int
		}{{tea.KeyPgDown, 11}, {tea.KeyPgUp, 0}, {tea.KeyEnd, 11}, {tea.KeyHome, 0}} {
			h.press("2")
			h.m.Update(tea.KeyPressMsg{Code: step.code})
			if h.m.cursor != step.row || h.m.count != 0 {
				t.Fatalf("alias %v: cursor %d, count %d", step.code, h.m.cursor, h.m.count)
			}
		}
	})

	t.Run("delete and restore one step", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("3", "d", "d")
		h.want("▌ ○ Task 03")
		h.press("u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03")
	})

	t.Run("close stamp undo and redo", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("2", "x")
		h.want("▌ ✓ Task 00", "  ✓ Task 01", "  ○ Task 02", "  ○ Task 03")
		h.press("3", "#", `"tag"`, "enter")
		h.want("▌ ✓ Task 00 #tag", "  ✓ Task 01 #tag", "  ○ Task 02 #tag", "  ○ Task 03")
		h.press("2", "u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03")
		h.press("2", "ctrl+r")
		h.want("  ✓ Task 00 #tag", "  ✓ Task 01 #tag", "▌ ○ Task 02 #tag", "  ○ Task 03")
	})

	t.Run("reorder and restore one step", func(t *testing.T) {
		h := countOutline(t, 5)
		h.press("3", "J")
		h.want("  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "▌ ○ Task 00", "  ○ Task 04")
		h.press("u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04")
		h.press("G", "9", "K", "u")
		h.want("  ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "▌ ○ Task 04")
		h.press("g", "g", "9", "J")
		h.want("  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04", "▌ ○ Task 00")
		h.press("u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	})

	t.Run("selection refuses counted row acts", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("2", "v")
		if !strings.Contains(h.status(), "2 selected") {
			t.Fatal(h.status())
		}
		for _, act := range []string{"x", "!", "t", "dd", "yy", "m", ">", "<", ".", "#", "@", "$", "*", "-", "v"} {
			gen, undos := h.m.tree.Gen(), len(h.m.undo)
			h.press("2", `"`+act+`"`)
			if !strings.Contains(h.status(), "a count cannot act on a selection") || h.m.tree.Gen() != gen || len(h.m.undo) != undos || len(h.m.picked) != 2 || h.m.ask != nil {
				t.Fatalf("%s was not refused: %s", act, h.status())
			}
		}
		h.press("3", "esc")
		if len(h.m.picked) != 2 || h.m.count != 0 {
			t.Fatal("Esc with a count changed the selection")
		}
		h.press("esc", "G", "9", "v")
		if len(h.m.picked) != 1 {
			t.Fatal("v went beyond the end")
		}
	})

	t.Run("pending count and cancellation", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("0")
		if h.m.count != 0 {
			t.Fatal("zero started a count")
		}
		h.press("3")
		if !strings.Contains(h.status(), "count 3") {
			t.Fatal(h.status())
		}
		h.press("esc", "j")
		if h.m.cursor != 1 || h.m.at.root == "" {
			t.Fatal("Esc failed to drop only the count")
		}
		h.press("3", "d", "esc", "j")
		if h.m.cursor != 2 || len(h.m.vis) != 4 {
			t.Fatal("Esc failed to cancel counted dd")
		}
		h.press("3", "A", `" !"`, "esc")
		if h.m.count != 0 || h.m.tree.Row(h.m.current()).Fields[sigil.FieldFlag] != "1" {
			t.Fatal("edit did not drop the count and act normally")
		}
	})

	t.Run("filter skips contextual ancestors", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First #hit"`, "esc", "o", `"Heading:"`, "esc", "o", `"Second #hit"`, "esc", ">")
		h.press("/", `"#hit"`, "enter", "2", "d", "d")
		h.press("esc")
		h.want("▌   Heading:")
		h.press("u")
		h.want("▌   First #hit", "  ▾ Heading:", "      Second #hit")
	})

	t.Run("counted block indent and outdent", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("j", "3", ">")
		h.want("  ○ Task 00", "▌   ○ Task 01", "    ○ Task 02", "    ○ Task 03")
		h.press("3", "<")
		h.want("  ○ Task 00", "▌ ○ Task 01", "  ○ Task 02", "  ○ Task 03")
		h.press("2", "u")
		h.want("  ○ Task 00", "▌ ○ Task 01", "  ○ Task 02", "  ○ Task 03")
	})

	t.Run("today reorders in one undo step", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("4", "t", "f", `"Today"`, "enter", "3", "J")
		if h.m.tree.Row(h.m.current()).Text() != "Task 00" || h.m.cursor != 3 {
			t.Fatal("counted Today reorder lost the cursor")
		}
		h.press("u")
		if h.m.cursor != 0 || h.m.tree.Row(h.m.current()).Text() != "Task 00" {
			t.Fatal("Today reorder was not undone in one step")
		}
	})

	t.Run("remaining row acts take a range", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("2", "!", "2", "y", "y")
		if !strings.Contains(h.status(), "copied 2 rows") {
			t.Fatal(h.status())
		}
		h.press("2", "m", `"Inbox"`, "enter")
		h.want("▌ ○ Task 02", "  ○ Task 03")
		h.press("2", ".")
		if len(h.m.vis) != 0 || len(h.m.picked) != 0 {
			t.Fatal("counted repeat move did not take the remaining rows")
		}
		h.press("u", "u")
		h.want("▌ ○ Task 00 !", "  ○ Task 01 !", "  ○ Task 02", "  ○ Task 03")
	})
}

func TestCountsAndMotionsReuseVisibleRows(t *testing.T) {
	h := countOutline(t, 12)
	// A sentinel depth in the cached view detects rebuilding from the tree,
	// even if rebuild reuses the backing slice. Exercise the real Update path.
	h.m.vis[11].depth = 99
	for _, tok := range []string{"1", "0", "j", "2", "k", "3", "g", "g", "5", "esc", "2", "v"} {
		h.m.Update(key(tok))
		if h.m.vis[11].depth != 99 {
			t.Fatalf("%s rebuilt the visible outline", tok)
		}
	}
	if got := h.m.shownRange(3, 1); !slices.Equal(got, []string{h.m.vis[1].id, h.m.vis[2].id, h.m.vis[3].id}) {
		t.Fatalf("reversed shown range = %v", got)
	}
}

// failCountedMove lets the first moves save, then fails a later one.
type failCountedMove struct {
	Backend
	remaining int
}

func (b *failCountedMove) Commit(ops []outline.Op, now int64) error {
	if b.remaining == 0 {
		return errors.New("disk full")
	}
	b.remaining--
	return b.Backend.Commit(ops, now)
}

func TestCountedReorderKeepsHistory(t *testing.T) {
	t.Run("full history and restart", func(t *testing.T) {
		h := countOutline(t, 5)
		for range undoDepth {
			h.press("!")
		}
		oldestKept := slices.Clone(h.m.undo[1])
		h.press("3", "J")
		if len(h.m.undo) != undoDepth || !slices.Equal(h.m.undo[0], oldestKept) {
			t.Fatal("a counted reorder evicted more than one history entry")
		}
		h = reopen(t, h)
		h.press("g", "g", "enter", "u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	})

	t.Run("later save failure keeps earlier moves undoable", func(t *testing.T) {
		h := countOutline(t, 5)
		backend := h.m.backend
		h.m.backend = &failCountedMove{Backend: backend, remaining: 2}
		h.press("4", "J")
		h.want("  ○ Task 01", "  ○ Task 02", "▌ ○ Task 00", "  ○ Task 03", "  ○ Task 04")
		if !strings.Contains(h.status(), "disk full") {
			t.Fatal(h.status())
		}
		h.m.backend = backend
		h.press("u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	})

	t.Run("selected block repeats as one step", func(t *testing.T) {
		h := countOutline(t, 5)
		h.press("2", "v", "k", "k")
		before := h.rows()
		h.press("3", "J", "u")
		if h.rows() != before || len(h.m.picked) != 2 {
			t.Fatal("counted selected reorder lost order, cursor or selection")
		}
	})
}

func TestVSweepsASelection(t *testing.T) {
	picks := func(h *harness, indices ...int) {
		t.Helper()
		want := make([]string, 0, len(indices))
		for _, i := range indices {
			want = append(want, h.m.vis[i].id)
		}
		if len(h.m.picked) != len(want) {
			t.Fatalf("picks = %v, want rows %v", h.m.picked, indices)
		}
		for _, id := range want {
			if !h.m.picked[id] {
				t.Fatalf("missing pick %s in %v", id, h.m.picked)
			}
		}
	}

	t.Run("grows shrinks crosses anchor and ends", func(t *testing.T) {
		h := countOutline(t, 5)
		h.press("j", "V", "j", "j")
		picks(h, 1, 2, 3)
		h.want("  ○ Task 00", " ┃○ Task 01", " ┃○ Task 02", "▌┃○ Task 03", "  ○ Task 04")
		h.press("k")
		picks(h, 1, 2)
		h.press("3", "k")
		picks(h, 0, 1)
		h.press("j", "j", "V", "j")
		picks(h, 1, 2)
		if h.m.sweep != "" || h.m.cursor != 3 {
			t.Fatal("V did not end the sweep")
		}
		h.press("esc")
		picks(h)
	})

	t.Run("delete to end is one undo step", func(t *testing.T) {
		h := countOutline(t, 5)
		h.press("j", "V", "G")
		picks(h, 1, 2, 3, 4)
		before := len(h.m.undo)
		h.press("d")
		if h.m.sweep != "" {
			t.Fatal("operator prefix left the sweep live")
		}
		h.press("d")
		h.want("▌ ○ Task 00")
		if len(h.m.undo) != before+1 {
			t.Fatal("swept delete was not one undo step")
		}
		h.press("u")
		h.want("  ○ Task 00", "▌ ○ Task 01", "  ○ Task 02", "  ○ Task 03", "  ○ Task 04")
	})

	t.Run("counted motion closes three rows", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("V", "2", "j", "x")
		h.want(" ┃✓ Task 00", " ┃✓ Task 01", "▌┃✓ Task 02", "  ○ Task 03")
		h.press("j")
		picks(h, 0, 1, 2)
		h.press("u", "esc")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03")
	})

	t.Run("earlier picks survive shrinking and v still toggles", func(t *testing.T) {
		h := countOutline(t, 5)
		h.press("v", "j", "v", "k", "k", "V", "3", "j", "3", "k")
		picks(h, 0, 1, 2)
		h.press("v", "j")
		picks(h, 0, 2)
		if h.m.cursor != 3 || h.m.sweep != "" {
			t.Fatal("v did not toggle, advance and end the sweep")
		}
		h.press("V", "g", "esc", "j")
		picks(h)
		if h.m.sweep != "" {
			t.Fatal("Esc left a sweep live")
		}
	})

	t.Run("all motions and aliases", func(t *testing.T) {
		h := countOutline(t, 12)
		h.press("V")
		for _, step := range []struct {
			keys []string
			code rune
			last int
		}{
			{keys: []string{"2", "ctrl+d"}, last: 10},
			{keys: []string{"ctrl+u"}, last: 5},
			{keys: []string{"ctrl+f"}, last: 11},
			{keys: []string{"ctrl+b"}, last: 1},
			{keys: []string{"2", "down"}, last: 3},
			{keys: []string{"up"}, last: 2},
			{keys: []string{"2", "g", "g"}, last: 0},
			{keys: []string{"2", "G"}, last: 11},
			{code: tea.KeyHome, last: 0},
			{code: tea.KeyEnd, last: 11},
			{code: tea.KeyPgUp, last: 1},
			{code: tea.KeyPgDown, last: 11},
		} {
			h.press(step.keys...)
			if step.code != 0 {
				h.m.Update(tea.KeyPressMsg{Code: step.code})
			}
			indices := make([]int, step.last+1)
			for i := range indices {
				indices[i] = i
			}
			picks(h, indices...)
		}
	})

	t.Run("filter leaves dimmed ancestors unselected", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First #hit"`, "esc", "o", `"Heading:"`, "esc", "o", `"Second #hit"`, "esc", ">")
		h.press("/", `"#hit"`, "enter", "V", "G")
		picks(h, 0, 2)
		h.press("k")
		picks(h, 0)
		h.press("esc", "V") // Starting on a dimmed ancestor also skips it.
		picks(h)
		h.press("j")
		picks(h, 2)
	})

	t.Run("lists flat filters and destination resets", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("4", "t", "f", `"Today"`, "enter", "V", "2", "j")
		picks(h, 0, 1, 2)
		h.press("/", `"Task"`, "tab", "enter", "V", "j")
		picks(h, 0, 1)
		h.press("f", "enter", "j") // Back to the project.
		picks(h)
		h.press("V", "f", `"Today"`, "enter", "j")
		picks(h)
		h.press("V", "/")
		h.m.Update(tea.PasteMsg{Content: "Task"})
		h.press("enter", "j")
		picks(h)
		if h.m.sweep != "" {
			t.Fatal("filter paste retained a sweep")
		}
	})

	t.Run("non motions end sweep and count cancellation keeps picks", func(t *testing.T) {
		for _, k := range []string{"h", "l", "z", "A", "tab", "#"} {
			h := countOutline(t, 4)
			h.press("V", "j", k)
			if h.m.sweep != "" {
				t.Fatalf("%s left a live sweep", k)
			}
			picks(h, 0, 1)
		}
		h := countOutline(t, 4)
		h.press("V", "j", "3", "esc", "j")
		picks(h, 0, 1)
		h.press("esc")
		picks(h)
	})

	t.Run("empty pane and cached visible rows", func(t *testing.T) {
		h := start(t)
		h.press("V", "j", "G")
		picks(h)
		if h.m.sweep != "" {
			t.Fatal("empty pane started a sweep")
		}
		h = countOutline(t, 12)
		h.m.vis[11].depth = 99
		before := len(h.m.undo)
		for _, tok := range []string{"V", "2", "j", "k", "G", "g", "g", "V", "j"} {
			h.m.Update(key(tok))
			if h.m.vis[11].depth != 99 || len(h.m.undo) != before {
				t.Fatalf("%s rebuilt the outline or changed history", tok)
			}
		}
	})
}

func TestDeleteAndCopyTakeMotions(t *testing.T) {
	for _, tc := range []struct {
		name, finish, text string
		child              bool
		remaining          int
	}{
		{"dd captures subtree before deleting", "d", "Task 00\n\tTask 01 $open\n", true, 2},
		{"dj captures range before deleting", "j", "Task 00\nTask 01\n", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := countOutline(t, 4)
			if tc.child {
				h.press("j", ">", "g", "g")
			}
			ids := []string{h.m.vis[0].id, h.m.vis[1].id}
			before, undo := h.rows(), len(h.m.undo)
			for i, tok := range []string{"d", tc.finish} {
				_, cmd := h.m.Update(key(tok))
				if got := clipboardMessages(t, cmd, tc.text); got != i {
					t.Fatalf("key %d (%s) emitted %d clipboard messages, want %d", i, tok, got, i)
				}
				if i == 0 && (h.m.register != "" || h.rows() != before || len(h.m.undo) != undo) {
					t.Fatal("pending d changed rows, history or register")
				}
			}
			if h.m.register != tc.text || len(h.m.undo) != undo+1 || len(h.m.vis) != tc.remaining {
				t.Fatalf("delete: register %q, history %d, visible %d", h.m.register, len(h.m.undo), len(h.m.vis))
			}
			for _, id := range ids {
				if h.m.live(id) {
					t.Fatalf("deleted row %s is still live", id)
				}
			}
			h.want("▌ ○ Task 02", "  ○ Task 03")
			_, cmd := h.m.Update(tea.WindowSizeMsg{Width: h.m.width, Height: h.m.height})
			if clipboardMessages(t, cmd, tc.text) != 0 {
				t.Fatal("resize delivered a duplicate clipboard command")
			}
			_, cmd = h.m.Update(key("u"))
			if clipboardMessages(t, cmd, tc.text) != 0 || h.rows() != before || h.m.register != tc.text || len(h.m.undo) != undo {
				t.Fatal("one undo did not restore both rows while keeping the register")
			}
		})
	}

	for _, tc := range []struct {
		keys               string
		start, first, last int
	}{
		{"dG", 2, 2, 6}, {"dgg", 3, 0, 3}, {"d2j", 0, 0, 2},
		{"2dj", 0, 0, 2}, {"2d2j", 0, 0, 4}, {"3dd", 0, 0, 2},
		{"d3j", 0, 0, 3}, {"3dj", 0, 0, 3}, {"2d2k", 5, 1, 5},
	} {
		t.Run(tc.keys, func(t *testing.T) {
			h := countOutline(t, 7)
			h.m.cursor = tc.start
			before := len(h.m.undo)
			var expected strings.Builder
			for i := tc.first; i <= tc.last; i++ {
				fmt.Fprintf(&expected, "Task %02d\n", i)
			}
			h.press(`"` + tc.keys + `"`)
			if len(h.m.vis) != 7-(tc.last-tc.first+1) || h.m.register != expected.String() || len(h.m.undo) != before+1 {
				t.Fatalf("wrong delete range: %s, register %q", h.rows(), h.m.register)
			}
			h.press("u")
			if len(h.m.vis) != 7 || h.m.register != expected.String() {
				t.Fatal("undo did not restore range and retain register")
			}
		})
	}

	t.Run("copy waits keeps cursor and has no undo", func(t *testing.T) {
		h := countOutline(t, 4)
		copyThroughUpdate(t, h, "Task 00\n", 1)
		before, undo := h.rows(), len(h.m.undo)
		for i, tok := range []string{"y", "j"} {
			_, cmd := h.m.Update(key(tok))
			if clipboardMessages(t, cmd, "Task 00\nTask 01\n") != i {
				t.Fatal("yj clipboard delivery")
			}
		}
		if h.m.msg != "copied 2 rows" || h.rows() != before || len(h.m.undo) != undo || h.m.register != "Task 00\nTask 01\n" {
			t.Fatal("yj changed rows/cursor/history or failed to copy")
		}
		for _, cancel := range []string{"esc", "x", "tab", "d", "h", "V"} {
			for _, tok := range []string{"y", cancel} {
				_, cmd := h.m.Update(key(tok))
				if clipboardMessages(t, cmd, "") != 0 {
					t.Fatal("cancelled copy emitted a command")
				}
			}
			if h.rows() != before || len(h.m.undo) != undo || h.m.register != "Task 00\nTask 01\n" || h.m.operator != "" || h.m.sideFocus || h.m.sweep != "" {
				t.Fatalf("%s did not cancel only the operator", cancel)
			}
		}
	})

	t.Run("selection refuses motions and accepts doubled operators", func(t *testing.T) {
		h := countOutline(t, 4)
		h.press("V", "j")
		before, undo := h.rows(), len(h.m.undo)
		for _, op := range []string{"d", "y"} {
			for _, tok := range []string{op, "j"} {
				_, cmd := h.m.Update(key(tok))
				if clipboardMessages(t, cmd, "") != 0 {
					t.Fatal("refused operator copied")
				}
			}
			if !strings.Contains(h.m.msg, "a motion cannot act on a selection") || h.rows() != before || len(h.m.undo) != undo || h.m.sweep != "" {
				t.Fatal("operator motion was not refused or sweep stayed live")
			}
		}
		copyThroughUpdate(t, h, "Task 00\nTask 01\n", 2)
		h.press("d", "d", "u")
		if len(h.m.vis) != 4 || h.m.register != "Task 00\nTask 01\n" {
			t.Fatal("selection delete/undo")
		}
	})

	t.Run("motion aliases and counted copy", func(t *testing.T) {
		for _, tc := range []struct {
			motion      string
			code        rune
			first, last int
		}{
			{"down", 0, 6, 7}, {"up", 0, 5, 6},
			{"ctrl+d", 0, 6, 11}, {"ctrl+u", 0, 1, 6},
			{"ctrl+f", 0, 6, 11}, {"ctrl+b", 0, 0, 6},
			{"", tea.KeyPgDown, 6, 11}, {"", tea.KeyPgUp, 0, 6},
			{"", tea.KeyHome, 0, 6}, {"", tea.KeyEnd, 6, 11},
		} {
			h := countOutline(t, 12)
			h.m.cursor = 6
			h.press("y")
			msg := tea.KeyPressMsg{Code: tc.code}
			if tc.motion != "" {
				msg = key(tc.motion)
			}
			_, cmd := h.m.Update(msg)
			var expected strings.Builder
			for i := tc.first; i <= tc.last; i++ {
				fmt.Fprintf(&expected, "Task %02d\n", i)
			}
			if clipboardMessages(t, cmd, expected.String()) != 1 || h.m.cursor != 6 || h.m.register != expected.String() {
				t.Fatalf("motion %s/%d failed", tc.motion, tc.code)
			}
		}
		h := countOutline(t, 7)
		h.press("3", "y", "y")
		if h.m.msg != "copied 3 rows" {
			t.Fatal(h.m.msg)
		}
		h.press("2", "y")
		if !strings.Contains(h.status(), "count 2 y") {
			t.Fatal(h.status())
		}
		h.press("2", "j")
		if h.m.msg != "copied 5 rows" || h.m.cursor != 0 {
			t.Fatal(h.m.msg)
		}
		h.press(`"999999999999999999999d999999999999999999999j"`)
		if len(h.m.vis) != 0 {
			t.Fatal("multiplied count overflowed")
		}
	})

	t.Run("filter leaves dimmed ancestors out", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First #hit"`, "esc", "o", `"Heading:"`, "esc", "o", `"Second #hit"`, "esc", ">")
		h.press("/", `"#hit"`, "enter", "d", "G")
		if h.m.register != "First #hit\nSecond #hit $note\n" {
			t.Fatal(h.m.register)
		}
		h.press("esc")
		h.want("▌   Heading:")
		h.press("u")
		h.want("▌   First #hit", "  ▾ Heading:", "      Second #hit")
	})
}

func TestPPutsTheRegister(t *testing.T) {
	t.Run("delete move put and two undos", func(t *testing.T) {
		h := countOutline(t, 4)
		original, steps := h.m.current(), len(h.m.undo)
		h.press("d", "d", "j", "p")
		h.want("  ○ Task 01", "  ○ Task 02", "▌ ○ Task 00", "  ○ Task 03")
		if h.m.current() == original || h.status() != "put 1 rows" || len(h.m.undo) != steps+2 {
			t.Fatal("put must create a fresh row and its own undo step")
		}
		register := h.m.register
		h.press("u")
		h.want("  ○ Task 01", "  ○ Task 02", "▌ ○ Task 03")
		h.press("u")
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02", "  ○ Task 03")
		if h.m.current() != original || h.m.register != register {
			t.Fatal("undo must restore the original identity and keep the register")
		}
	})
	t.Run("ddp swaps neighbours", func(t *testing.T) {
		h := countOutline(t, 3)
		h.press("d", "d", "p")
		h.want("  ○ Task 01", "▌ ○ Task 00", "  ○ Task 02")
	})
	t.Run("copy a subtree at its level with fields", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Alpha\n\tHeading:\n\t\tTask ! #tag @2026-12-01\n\t\t\tDetail\n\tOther")
		h.press("f", `"Alpha"`, "enter", "j", "y", "y")
		original, register, steps := h.m.current(), h.m.register, len(h.m.undo)
		h.press("p")
		h.want("  ▾ Heading:", "    ○ Task ! @Dec 1 #tag", "        Detail", "    ○ Task ! @Dec 1 #tag", "▌       Detail", "  ○ Other")
		copy := h.m.tree.Row(h.m.current()).Parent
		if copy == original || h.m.tree.Row(copy).Parent != h.m.tree.Row(original).Parent ||
			h.m.register != register || h.status() != "put 2 rows" || len(h.m.undo) != steps+1 {
			t.Fatal("subtree put lost level, identity, register, status or undo grouping")
		}
		h.press("u")
		h.want("  ▾ Heading:", "    ○ Task ! @Dec 1 #tag", "        Detail", "▌ ○ Other")
	})
	t.Run("before keeps multiple roots and children in order", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Alpha\n\tFirst\n\t\tChild\n\tSecond\n\tTarget")
		h.press("f", `"Alpha"`, "enter", "3", "y", "y", "G")
		register, steps := h.m.register, len(h.m.undo)
		h.press("P")
		h.want("  ○ First", "      Child", "  ○ Second", "  ○ First", "      Child", "▌ ○ Second", "  ○ Target")
		if h.status() != "put 3 rows" || h.m.register != register || len(h.m.undo) != steps+1 {
			t.Fatal("P lost row count, register or undo grouping")
		}
		h.press("u")
		h.want("  ○ First", "      Child", "  ○ Second", "▌ ○ Target")
	})
	t.Run("each put creates fresh rows and drops a count", func(t *testing.T) {
		h := countOutline(t, 1)
		h.press("y", "y")
		register := h.m.register
		h.press("3", "p", "2", "p", "4", "P")
		h.want("  ○ Task 00", "  ○ Task 00", "▌ ○ Task 00", "  ○ Task 00")
		if len(h.m.tree.Children(h.m.at.root)) != 4 || h.m.register != register || h.m.count != 0 {
			t.Fatal("put did not create exactly one fresh row per press or retain the register")
		}
	})
	t.Run("done task stays done", func(t *testing.T) {
		h := countOutline(t, 1)
		h.command("closed")
		h.press("x", "y", "y", "p")
		h.want("  ✓ Task 00", "▌ ✓ Task 00")
	})
	for _, put := range []string{"p", "P"} {
		t.Run(put+" empty register", func(t *testing.T) {
			h := start(t)
			gen, steps := h.m.tree.Gen(), len(h.m.undo)
			h.press(put)
			if h.status() != "nothing to put" || h.m.ask != nil || h.m.tree.Gen() != gen || len(h.m.undo) != steps {
				t.Fatal("empty register changed rows or opened a picker")
			}
		})
		t.Run(put+" empty place", func(t *testing.T) {
			h := countOutline(t, 1)
			h.press("d", "d", put)
			h.want("▌ ○ Task 00")
			if h.m.tree.Row(h.m.current()).Parent != h.m.at.root || h.status() != "put 1 rows" {
				t.Fatal("empty place did not receive a child")
			}
		})
		for _, place := range []string{"Today", "filter", "flat filter", "selection", "sweep", "sidebar"} {
			t.Run(put+" refused in "+place, func(t *testing.T) {
				h := countOutline(t, 3).wide()
				h.press("y", "y")
				message := "nothing selected"
				switch place {
				case "Today":
					h.press("t", "f", `"Today"`, "enter")
					message = "a list has no order"
				case "filter", "flat filter":
					h.press("/", `"Task"`)
					if place == "flat filter" {
						h.press("tab")
					}
					h.press("enter")
					message = "clear the filter first"
				case "selection":
					h.press("v")
				case "sweep":
					h.press("V", "j")
				case "sidebar":
					h.press("tab")
					message = ""
				}
				before, gen, steps, register := h.rows(), h.m.tree.Gen(), len(h.m.undo), h.m.register
				h.press(put)
				if !strings.Contains(h.status(), message) || h.rows() != before || h.m.tree.Gen() != gen || len(h.m.undo) != steps || h.m.register != register || h.m.ask != nil {
					t.Fatalf("put refusal changed state or had wrong message: %q", h.status())
				}
			})
		}
	}
	t.Run("failed put keeps rows register and history", func(t *testing.T) {
		h := countOutline(t, 2).wide()
		h.press("2", "y", "y")
		before, register, steps := h.rows(), h.m.register, len(h.m.undo)
		h.m.backend = &failCountedMove{Backend: h.m.backend}
		h.press("P")
		if h.rows() != before || h.m.register != register || len(h.m.undo) != steps || h.m.msg != "not saved, nothing changed: disk full" {
			t.Fatalf("failed put changed state or overwrote the error: %q", h.m.msg)
		}
	})
	t.Run("f opens Go to and blank returns including sidebar", func(t *testing.T) {
		h := countOutline(t, 3)
		h.press("j")
		before := h.rows()
		h.press("f")
		if h.m.ask == nil || h.m.ask.kind != askJump || !strings.HasPrefix(h.bottom(), "Go to ›") {
			t.Fatal("f did not open Go to")
		}
		h.press(`"Inbox"`, "enter", "f", "enter")
		if h.header() != "Project" || h.rows() != before {
			t.Fatal("blank f Enter did not restore the previous place and cursor")
		}
		h.press("tab", "f", `"Inbox"`, "enter")
		if h.m.sideFocus || h.header() != "Inbox" {
			t.Fatal("sidebar f did not jump")
		}
	})
}

func TestIAndAOpenTypingAndCcRetypes(t *testing.T) {
	t.Run("append and insert drop counts", func(t *testing.T) {
		h := countOutline(t, 2)
		h.press("3", "A", `" café"`, "esc")
		h.want("▌ ○ Task 00 café", "  ○ Task 01")
		h.press("2", "I", `"New "`, "esc")
		h.want("▌ ○ New Task 00 café", "  ○ Task 01")
		h.press("j")
		if h.m.cursor != 1 || h.m.count != 0 {
			t.Fatal("typing entry leaked a count")
		}
	})

	t.Run("retype preserves all fields and undoes once", func(t *testing.T) {
		h := countOutline(t, 1)
		h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
		h.press("A", "ctrl+u", `"Old ! @2026-11-20 @@2026-11-14 #release $done"`, "esc", "t")
		id := h.m.current()
		before := maps.Clone(h.m.tree.Row(id).Fields)
		undos := len(h.m.undo)
		h.press("5", "c")
		if h.m.edit != nil || !strings.Contains(h.status(), "c") || h.m.count != 0 || h.m.opCount != 0 {
			t.Fatal("c did not wait and drop its count")
		}
		h.press("c")
		if got := string(h.m.edit.buf); got != " ! @2026-11-20 @@2026-11-14 #release $done" || h.m.edit.caret != 0 {
			t.Fatalf("cc opened %q at %d", got, h.m.edit.caret)
		}
		if !maps.Equal(before, h.m.tree.Row(id).Fields) || len(h.m.undo) != undos {
			t.Fatal("cc wrote before typing ended")
		}
		h.press(`"New"`, "esc")
		h.want("▌ ✓ New ! @fri @@Nov 14 #release")
		want := maps.Clone(before)
		want[sigil.FieldText] = "New"
		if !maps.Equal(want, h.m.tree.Row(id).Fields) || len(h.m.undo) != undos+1 {
			t.Fatalf("retype changed fields or history: %v", h.m.tree.Row(id).Fields)
		}
		h.press("u")
		if !maps.Equal(before, h.m.tree.Row(id).Fields) || len(h.m.undo) != undos {
			t.Fatal("one undo did not restore all fields")
		}
		h.want("▌ ✓ Old ! @fri @@Nov 14 #release")
		h.press("c", "c", "esc")
		delete(want, sigil.FieldText)
		if !maps.Equal(want, h.m.tree.Row(id).Fields) || h.m.current() != id {
			t.Fatal("blank retype lost the row or its fields")
		}
		h.press("u")
		if !maps.Equal(before, h.m.tree.Row(id).Fields) {
			t.Fatal("undo did not restore blank retype")
		}
	})

	t.Run("state survives a changed text default", func(t *testing.T) {
		for _, tc := range []struct{ old, text, state string }{
			{"Heading:", "Plain", ""},
			{"Task", "Heading:", sigil.Open},
			{"Note $note", "Plain", ""},
			{"Task *2w @@2026-11-20", "Heading:", sigil.Open},
		} {
			t.Run(tc.old, func(t *testing.T) {
				h := countOutline(t, 1)
				h.press("A", "ctrl+u", `"`+tc.old+`"`, "esc")
				id := h.m.current()
				before := maps.Clone(h.m.tree.Row(id).Fields)
				h.press("c", "c", `"`+tc.text+`"`, "esc")
				want := maps.Clone(before)
				want[sigil.FieldText] = tc.text
				if got := h.m.tree.Row(id).Fields; !maps.Equal(want, got) || got[sigil.FieldState] != tc.state {
					t.Fatalf("retype changed fields: %v", got)
				}
				h.press("u")
				if !maps.Equal(before, h.m.tree.Row(id).Fields) {
					t.Fatal("undo did not restore original fields")
				}
			})
		}
	})

	t.Run("c cancels without dispatch and e does nothing", func(t *testing.T) {
		h := countOutline(t, 3)
		for _, k := range []string{"esc", "j", "g", "G", "d", "y", "A", "I", "2", "0", "f", "!", "v"} {
			gen, undos, cur, root := h.m.tree.Gen(), len(h.m.undo), h.m.current(), h.m.at.root
			h.press("c", k)
			if h.m.edit != nil || h.m.ask != nil || h.m.operator != "" || h.m.prefix != "" || h.m.count != 0 || h.m.tree.Gen() != gen || len(h.m.undo) != undos || h.m.current() != cur || h.m.at.root != root || len(h.m.picked) != 0 {
				t.Fatalf("c %s did not cancel without acting", k)
			}
		}
		gen, undos := h.m.tree.Gen(), len(h.m.undo)
		h.press("e")
		if h.m.edit != nil || h.m.tree.Gen() != gen || len(h.m.undo) != undos {
			t.Fatal("e still edits")
		}
		h.want("▌ ○ Task 00", "  ○ Task 01", "  ○ Task 02")
	})

	t.Run("all entries refuse selection and end sweeps", func(t *testing.T) {
		for _, entry := range [][]string{{"A"}, {"I"}, {"c", "c"}} {
			h := countOutline(t, 3)
			h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
			h.press("V", "j", "4")
			gen, undos := h.m.tree.Gen(), len(h.m.undo)
			h.press(entry...)
			if h.m.edit != nil || h.m.tree.Gen() != gen || len(h.m.undo) != undos || len(h.m.picked) != 2 || h.m.sweep != "" || h.m.count != 0 || !strings.Contains(h.status(), "that works on one row") {
				t.Fatalf("%v did not refuse selection: %s", entry, h.status())
			}
		}
	})

	t.Run("lists and filters", func(t *testing.T) {
		for _, place := range []string{"Today", "filter", "flat filter"} {
			t.Run(place, func(t *testing.T) {
				h := countOutline(t, 1)
				if place == "Today" {
					h.press("t", "f", `"Today"`, "enter")
				} else {
					h.press("/", `"Task"`)
					if place == "flat filter" {
						h.press("tab")
					}
					h.press("enter")
				}
				id := h.m.current()
				h.press("A", `" end"`, "esc", "I", `"Start "`, "esc")
				if got := h.m.tree.Row(id).Text(); got != "Start Task 00 end" {
					t.Fatalf("typing in %s: %q", place, got)
				}
				h.press("c", "c", `"Replacement"`, "esc")
				if h.m.tree.Row(id).Text() != "Replacement" || h.m.current() != id {
					t.Fatalf("retype in %s lost the row", place)
				}
			})
		}
	})
}
