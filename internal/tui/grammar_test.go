package tui

import (
	"errors"
	"fmt"
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
		for _, act := range []string{"x", "!", "t", "dd", "y", "m", ">", "<", ".", "#", "@", "$", "*", "-", "v"} {
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
		h.press("3", "e", `" !"`, "esc")
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
		h.press("4", "t", "p", `"Today"`, "enter", "3", "J")
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
		h.press("2", "!", "2", "y")
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
		h.press("4", "t", "p", `"Today"`, "enter", "V", "2", "j")
		picks(h, 0, 1, 2)
		h.press("/", `"Task"`, "tab", "enter", "V", "j")
		picks(h, 0, 1)
		h.press("p", "enter", "j") // Back to the project.
		picks(h)
		h.press("V", "p", `"Today"`, "enter", "j")
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
		for _, k := range []string{"h", "l", "z", "e", "tab", "#"} {
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
