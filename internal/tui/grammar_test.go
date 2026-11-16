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
