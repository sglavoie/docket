package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/store"
)

type harness struct {
	t     *testing.T
	path  string
	st    *store.Store
	m     *Model
	clock time.Time
}

func open(t *testing.T, path string) *harness {
	t.Helper()
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return resume(t, path, st, time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
}

// resume starts the client at a given moment. The default is noon UTC on a
// Sunday, so the local day is the same in every zone.
func resume(t *testing.T, path string, st *store.Store, at time.Time) *harness {
	t.Helper()
	tree, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	folds, err := st.Folds()
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, path: path, st: st, clock: at}
	m := newModel(tree, st, folds, func() int64 { return h.clock.UnixMilli() })
	n := 0
	m.newID = func() string { n++; return fmt.Sprintf("%s-row%03d", filepath.Base(path), n) }
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	h.m = m
	h.settle()
	return h
}

// settle does a sync pass's work in the folder, which the program does away
// from the keys, and hands back what it found.
func (h *harness) settle() {
	for h.m.syncIO != nil && h.m.syncHeld == nil {
		h.m.Update(h.m.syncIO())
	}
}

// tick is the minute passing.
func (h *harness) tick() {
	h.m.Update(tickMsg{})
	h.settle()
}

func start(t *testing.T) *harness {
	return open(t, filepath.Join(t.TempDir(), "docket.db"))
}

func key(tok string) tea.KeyPressMsg {
	switch tok {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	}
	if rest, ok := strings.CutPrefix(tok, "alt+"); ok {
		k := key(rest)
		k.Mod |= tea.ModAlt
		k.Text = ""
		return k
	}
	switch tok {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	if c, ok := strings.CutPrefix(tok, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(c)[0], Mod: tea.ModCtrl}
	}
	r := []rune(tok)
	return tea.KeyPressMsg{Code: r[0], Text: tok}
}

// press sends named keys; a token in quotes is typed character by character.
func (h *harness) press(toks ...string) *harness {
	h.t.Helper()
	// The program draws after every message, and drawing settles the
	// scroll position, so the harness draws too.
	send := func(k tea.KeyPressMsg) {
		h.m.Update(k)
		h.settle()
		h.m.View()
	}
	for _, tok := range toks {
		if len(tok) > 1 && strings.HasPrefix(tok, `"`) {
			for _, r := range strings.Trim(tok, `"`) {
				send(key(string(r)))
			}
			continue
		}
		send(key(tok))
	}
	return h
}

const sideCols = sidebarCols + 3

func (h *harness) lines() []string {
	return strings.Split(ansi.Strip(h.m.View().Content), "\n")
}

// rows is the outline pane as drawn: no sidebar, no header, no status line,
// no styling.
func (h *harness) rows() string {
	h.t.Helper()
	lines := h.lines()
	var out []string
	for _, l := range lines[1 : len(lines)-1] {
		if r := []rune(l); h.m.width >= sidebarFrom {
			l = string(r[min(sideCols, len(r)):])
		}
		if l = strings.TrimRight(l, " "); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// side is the sidebar, one entry per line with its spacing collapsed.
func (h *harness) side() string {
	h.t.Helper()
	lines := h.lines()
	var out []string
	for _, l := range lines[:len(lines)-1] {
		if r := []rune(l); h.m.width >= sidebarFrom {
			if len(r) < sideCols {
				h.t.Fatalf("line narrower than the sidebar: %q", l)
			}
			l = string(r[:sidebarCols])
		}
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func (h *harness) wantSide(lines ...string) {
	h.t.Helper()
	if got, want := h.side(), strings.Join(lines, "\n"); got != want {
		h.t.Fatalf("sidebar:\n%s\n\nwant:\n%s", got, want)
	}
}

func (h *harness) header() string {
	l := h.lines()[0]
	if r := []rune(l); h.m.width >= sidebarFrom {
		l = string(r[min(sideCols, len(r)):])
	}
	return strings.TrimSpace(l)
}

func (h *harness) status() string {
	lines := h.lines()
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lines[len(lines)-1]), "? help"))
}

func (h *harness) wide() *harness {
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	return h
}

func (h *harness) want(rows ...string) {
	h.t.Helper()
	if got, want := h.rows(), strings.Join(rows, "\n"); got != want {
		h.t.Fatalf("screen:\n%s\n\nwant:\n%s", got, want)
	}
}

func outlineOfThree(t *testing.T) *harness {
	h := start(t)
	h.press("o", `"Alpha"`, "enter", `"Beta"`, "enter", `"Gamma"`, "esc")
	h.want(
		"    Alpha",
		"    Beta",
		"▌   Gamma",
	)
	return h
}

// project is one top-level row holding two tasks, the cursor on the second.
func project(t *testing.T) *harness {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"First"`, "enter", `"Second"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ○ Second",
	)
	return h
}

func TestTypingAListChainsRows(t *testing.T) {
	h := outlineOfThree(t)
	// A fresh row left blank disappears and the cursor returns.
	h.press("o", "esc")
	h.want(
		"    Alpha",
		"    Beta",
		"▌   Gamma",
	)
}

func TestIndentWhileTypingAndStructureKeys(t *testing.T) {
	h := project(t)
	h.press("K")
	h.want(
		"  ▾ Project",
		"▌   ○ Second",
		"    ○ First",
	)
	h.press("<")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌ ○ Second",
	)
	h.press(">")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ○ Second",
	)
	h.press("<", "<")
	if h.m.msg != "already at the top level" {
		t.Fatalf("msg = %q", h.m.msg)
	}
}

func TestStructureKeysMoveASelectedBlock(t *testing.T) {
	block := func(t *testing.T) *harness {
		h := start(t)
		h.press("o", `"Before"`, "enter", "tab", `"Existing"`, "enter", "shift+tab",
			`"One"`, "enter", `"Two"`, "enter", `"Three"`, "enter", `"After"`, "enter", `"Tail"`, "esc",
			"g", "g", "h", "j", "v", "v", "v", "k", "k")
		h.want("  ▸ Before", " ┃  One", "▌┃  Two", " ┃  Three", "    After", "    Tail")
		return h
	}
	for _, tc := range []struct {
		key  string
		want []string
	}{
		{">", []string{"  ▾ Before", "    ○ Existing", " ┃    One", "▌┃    Two", " ┃    Three", "    After", "    Tail"}},
		{"<", []string{"  ▾ Before", "    ○ Existing", " ┃  One", "▌┃  Two", " ┃  Three", "    After", "    Tail"}},
		{"J", []string{"  ▸ Before", "    After", " ┃  One", "▌┃  Two", " ┃  Three", "    Tail"}},
		{"K", []string{" ┃  One", "▌┃  Two", " ┃  Three", "  ▸ Before", "    After", "    Tail"}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			h := block(t)
			if tc.key == "<" {
				h.press(">")
			}
			before, cur, steps := h.rows(), h.m.current(), len(h.m.undo)
			ids := h.m.acting()
			h.press(tc.key)
			h.want(tc.want...)
			if h.m.current() != cur || len(h.m.picked) != 3 || len(h.m.undo) != steps+1 {
				t.Fatal("move lost the cursor or selection, or was not one undo step")
			}
			for _, id := range ids {
				if !h.m.picked[id] {
					t.Fatal("move changed selected rows")
				}
			}
			h.press("u", "j") // Undo seeks the first moved row; return to Two.
			if tc.key == ">" {
				// Folds are view state and are not undone.
				h.press("g", "g", "h", "j", "j")
			}
			if got := h.rows(); got != before || len(h.m.undo) != steps {
				t.Fatalf("one undo did not restore the block:\n%s\nwant:\n%s", got, before)
			}
		})
	}

	t.Run("repeat and edges", func(t *testing.T) {
		h := block(t)
		h.press("J", "J")
		h.want("  ▸ Before", "    After", "    Tail", " ┃  One", "▌┃  Two", " ┃  Three")
		before, steps := h.rows(), len(h.m.undo)
		h.press("J")
		if h.rows() != before || len(h.m.undo) != steps {
			t.Fatal("moved beyond the last sibling")
		}
		h.press("K", "K", "K")
		h.want(" ┃  One", "▌┃  Two", " ┃  Three", "  ▸ Before", "    After", "    Tail")
		before, steps = h.rows(), len(h.m.undo)
		for _, key := range []string{"K", ">", "<"} {
			h.press(key)
			if h.rows() != before || len(h.m.undo) != steps {
				t.Fatalf("%s moved beyond the first sibling or top level", key)
			}
		}
	})

	for _, shape := range []string{"gap", "different parents"} {
		t.Run(shape, func(t *testing.T) {
			h := block(t)
			h.press("esc", "g", "g", "j", "v", "j", "v") // One and Three.
			if shape == "different parents" {
				h.press("esc", "g", "g", "l", "j", "v", "v") // Existing and One.
			}
			before, steps := h.rows(), len(h.m.undo)
			for _, key := range []string{">", "<", "J", "K"} {
				h.press(key)
				if h.m.msg != "select rows that sit together to move them" || h.rows() != before || len(h.m.undo) != steps {
					t.Fatalf("%s did not refuse the selection unchanged: %q\n%s", key, h.m.msg, h.rows())
				}
			}
		})
	}

	t.Run("selected parent and child", func(t *testing.T) {
		h := start(t)
		h.press("o", `"Above"`, "enter", `"Parent"`, "enter", "tab", `"Child"`, "enter", "shift+tab", `"Below"`, "esc",
			"g", "g", "j", "v", "v", "k")
		for _, key := range []string{">", "<", "J", "K"} {
			if key == "<" {
				h.press(">")
			}
			before, cur, steps := h.rows(), h.m.current(), len(h.m.undo)
			h.press(key)
			if len(h.m.undo) != steps+1 || len(h.m.undo[steps]) != 1 || len(h.m.picked) != 2 || h.m.current() != cur {
				t.Fatalf("%s did not move the parent exactly once with the cursor on its child", key)
			}
			h.press("u", "j")
			if h.rows() != before {
				t.Fatalf("%s did not restore parent and child:\n%s\nwant:\n%s", key, h.rows(), before)
			}
			if key == "<" {
				h.press("u", "j")
			}
		}
	})

	t.Run("hidden selection follows sibling order", func(t *testing.T) {
		h := block(t)
		h.press("esc", "g", "g", "l", "j", "o", `"Later"`, "esc", "K")
		// Later was created last, but now sits before Existing.
		h.press("v", "v", "g", "g", "h")
		cur := h.m.current() // The unselected folded parent stays under the cursor.
		h.press("<")
		h.want("▌   Before", " ┃○ Later", " ┃○ Existing", "    One", "    Two", "    Three", "    After", "    Tail")
		if h.m.current() != cur {
			t.Fatal("moving hidden rows changed the cursor's row")
		}
	})

	for _, place := range []string{"filter", "list"} {
		t.Run(place, func(t *testing.T) {
			h := block(t)
			h.press("esc")
			if place == "filter" {
				h.press("/", `"o"`, "enter", "v")
			} else {
				h.press("t", "p", `"today"`, "enter", "v")
			}
			before, steps := h.rows(), len(h.m.undo)
			for _, key := range []string{">", "<", "J", "K"} {
				h.press(key)
				if h.m.msg == "" || h.rows() != before || len(h.m.undo) != steps {
					t.Fatalf("%s was not refused in a %s", key, place)
				}
			}
		})
	}

	t.Run("failed save", func(t *testing.T) {
		h := block(t)
		h.m.backend = &flaky{Backend: h.m.backend, fail: true}
		before, steps := h.rows(), len(h.m.undo)
		for _, key := range []string{">", "J", "K"} {
			h.press(key)
			if !strings.Contains(h.m.msg, "disk full") || h.rows() != before || len(h.m.undo) != steps {
				t.Fatalf("%s left a failed block move behind: %q", key, h.m.msg)
			}
		}
	})
}

func TestNewRowUnderAnOpenParentIsItsFirstChild(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Child"`, "esc", "k", "o", `"Top"`, "esc")
	h.want(
		"  ▾ Project",
		"▌   ○ Top",
		"    ○ Child",
	)
	h.press("k", "h", "o", `"Sibling"`, "esc")
	h.want(
		"  ▸ Project",
		"▌   Sibling",
	)
}

func TestFoldZoomAndBackOut(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Child"`, "esc")
	h.press("h")
	h.want(
		"▌ ▾ Project",
		"    ○ Child",
	)
	h.press("h")
	h.want("▌ ▸ Project")
	h.press("l", "l")
	h.want(
		"  ▾ Project",
		"▌   ○ Child",
	)
	h.press("k", "enter")
	h.want("▌ ○ Child")
	if got := ansi.Strip(strings.SplitN(h.m.View().Content, "\n", 2)[0]); got != "Project" {
		t.Fatalf("breadcrumb = %q", got)
	}
	h.press("esc")
	h.want(
		"▌ ▾ Project",
		"    ○ Child",
	)
}

func TestDeleteUndoRedo(t *testing.T) {
	h := outlineOfThree(t)
	h.press("k", "d", "d")
	h.want(
		"    Alpha",
		"▌   Gamma",
	)
	h.press("u")
	h.want(
		"    Alpha",
		"▌   Beta",
		"    Gamma",
	)
	h.press("ctrl+r")
	h.want(
		"    Alpha",
		"▌   Gamma",
	)
	// An edit is one undo step, text and indent together, and indenting a
	// note while typing leaves it a note.
	h.press("u", "e", `" two"`, "tab", "esc")
	h.want(
		"  ▾ Alpha",
		"▌     Beta two",
		"    Gamma",
	)
	h.press("u")
	h.want(
		"    Alpha",
		"▌   Beta",
		"    Gamma",
	)
}

func TestEverythingSurvivesARestart(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Child ! @fri #api $doing"`, "enter", `"Gone"`, "esc", "d", "d")
	h.press("g", "g", "o", "esc", "O", `"Above"`, "esc", "j", "h")
	want := []string{
		"    Above",
		"▌ ▸ Project",
	}
	h.want(want...)
	h.st.Close()

	again := open(t, h.path)
	again.press("j")
	again.want(want...)
	again.press("l")
	again.want(
		"    Above",
		"▌ ▾ Project",
		"    ● Child ! @fri #api",
	)
}

func TestLongRowScrollsWhileTypingAndWrapsWhenRead(t *testing.T) {
	h := start(t)
	long := strings.Repeat("word ", 12) + "end"
	h.press("o", `"`+long+`"`)
	v := h.m.View()
	if v.Cursor == nil || v.Cursor.X != 39 {
		t.Fatalf("caret = %+v, want column 39", v.Cursor)
	}
	if !strings.HasSuffix(h.rows(), "end") {
		t.Fatalf("typing window does not follow the caret:\n%s", h.rows())
	}
	// Read back, nothing is cut: the row wraps under its own text, and the
	// cursor mark runs its height.
	h.press("enter", `"next"`, "esc")
	h.want(
		"    word word word word word word word",
		"    word word word word word end",
		"▌   next",
	)
	h.press("k")
	h.want(
		"▌   word word word word word word word",
		"▌   word word word word word end",
		"    next",
	)
	for _, line := range h.lines() {
		if ansi.StringWidth(line) > 40 {
			t.Fatalf("line wider than the terminal: %q", line)
		}
	}
}

func TestAWordWiderThanThePaneIsCutAcrossLines(t *testing.T) {
	h := start(t)
	h.m.Update(tea.WindowSizeMsg{Width: 24, Height: 12})
	h.press("o", `"see https://example.com/a/very/long/path/indeed ok #x"`, "esc")
	h.want(
		"▌   see https://example.",
		"▌   com/a/very/long/path",
		"▌   /indeed ok #x",
	)
}

func TestScrollingKeepsTheCursorRowWhole(t *testing.T) {
	h := start(t)
	h.m.Update(tea.WindowSizeMsg{Width: 24, Height: 6}) // four list lines
	long := "alpha beta gamma delta epsilon zeta"
	h.press("o", `"one"`, "enter", `"two"`, "enter", `"`+long+`"`, "enter", `"four"`, "esc", "k")
	h.want(
		"    two",
		"▌   alpha beta gamma",
		"▌   delta epsilon zeta",
		"    four",
	)
	h.press("g", "g")
	h.want(
		"▌   one",
		"    two",
		"    alpha beta gamma",
		"    delta epsilon zeta",
	)
	h.press("G")
	h.want(
		"    two",
		"    alpha beta gamma",
		"    delta epsilon zeta",
		"▌   four",
	)
}

func TestAWrappedRowKeepsItsStyleOnEveryLine(t *testing.T) {
	h := project(t)
	h.m.Update(tea.WindowSizeMsg{Width: 24, Height: 12})
	h.press("o", `"alpha beta gamma delta epsilon zeta"`, "esc", "x", "k")
	lines := strings.Split(h.m.View().Content, "\n")
	for _, n := range []int{4, 5} {
		if !strings.Contains(lines[n], sgrDim+"alpha") && !strings.Contains(lines[n], sgrDim+"delta") {
			t.Fatalf("line %d of a done row is not dimmed: %q", n, lines[n])
		}
	}
}

func TestSigilsBecomeFieldsAndTheLineRoundTrips(t *testing.T) {
	h := project(t)
	h.m.Update(tea.WindowSizeMsg{Width: 70, Height: 12})
	h.press("o", `"Ship it ! @fri #Release $wait"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ◔ Ship it ! @fri #release",
	)
	// Editing shows the canonical line: a fixed date, since a weekday would
	// drift, and the state because it is not the default.
	h.press("e")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ◔ Ship it ! @2026-11-20 #release $waiting",
	)
	// Deleting a sigil from the line clears its field.
	h.press("ctrl+u", `"Ship it #release"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ○ Ship it #release",
	)
	h.press("u")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ◔ Ship it ! @fri #release",
	)
}

func TestProseThatLooksLikeSigilsStaysProse(t *testing.T) {
	h := project(t)
	h.m.Update(tea.WindowSizeMsg{Width: 70, Height: 12})
	h.press("o", `"Fix #123 for @alice in R&D \#api"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ○ Fix #123 for @alice in R&D #api",
	)
	h.press("e")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"▌   ○ Fix #123 for @alice in R&D \\#api",
	)
}

func TestDoneAndFlagKeys(t *testing.T) {
	h := project(t)
	h.press("x", "k", "!")
	h.want(
		"  ▾ Project",
		"▌   ○ First !",
		"    ✓ Second",
	)
	changed := h.m.tree.Row(h.m.vis[2].id).Fields["changed"]
	if changed != "1794744000000" {
		t.Fatalf("state clock = %q", changed)
	}
	// Done rows stay where they are, and x takes them back.
	h.press("!", "j", "x")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ○ Second",
	)
	h.press("u")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ✓ Second",
	)
	h.press("g", "g", "x")
	if !strings.Contains(h.m.msg, "a note has no state") {
		t.Fatalf("msg = %q", h.m.msg)
	}
}

func TestHeadingsNotesAndFoldedTasks(t *testing.T) {
	h := project(t)
	h.press("o", `"Later:"`, "enter", "tab", `"Idea"`, "enter", `"a plain remark $note"`, "esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ▾ Later:",
		"      ○ Idea",
		"▌       a plain remark",
	)
	// A folded task keeps its state glyph and counts what it hides.
	h.press(">", "h", "h")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ▾ Later:",
		"▌     ○ Idea +1",
	)
}

func TestOverdueAndTodayAreColouredUntilTheRowCloses(t *testing.T) {
	h := project(t)
	h.press("o", `"Late @2026-11-10"`, "enter", `"Now @today"`, "esc")
	raw := h.m.View().Content
	if !strings.Contains(raw, sgrAlert+"@Nov 10") || !strings.Contains(raw, sgrWarn+"@today") {
		t.Fatalf("deadlines are not coloured:\n%q", raw)
	}
	h.press("x", "k", "x")
	raw = h.m.View().Content
	if strings.Contains(raw, sgrAlert) || strings.Contains(raw, sgrWarn) {
		t.Fatalf("a closed row still raises an alarm:\n%q", raw)
	}
}

func TestARowUnderATaskIsANote(t *testing.T) {
	for _, state := range []string{"open", "doing", "waiting", "done", "canceled", "parked"} {
		t.Run("parent "+state, func(t *testing.T) {
			h := project(t)
			h.press("$", `"`+state+`"`, "enter", "o", `"Detail"`, "tab", "esc")
			if got := h.rows(); !strings.Contains(got, "▌       Detail") {
				t.Fatalf("child should draw without a state glyph:\n%s", got)
			}
			h.press("e")
			if got := string(h.m.edit.buf); got != "Detail" {
				t.Fatalf("note edit line = %q", got)
			}
			h.press(`" $open"`, "esc", "e")
			if got := string(h.m.edit.buf); got != "Detail $open" {
				t.Fatalf("subtask edit line = %q", got)
			}
			h.press("esc")
			if got := h.rows(); !strings.Contains(got, "▌     ○ Detail") {
				t.Fatalf("explicit subtask lacks task glyph:\n%s", got)
			}
		})
	}

	t.Run("children of notes are tasks", func(t *testing.T) {
		for _, parent := range []string{"Heading:", "Plain note $note"} {
			h := project(t)
			h.press("o", `"`+parent+`"`, "enter", "tab", `"Child"`, "esc")
			if got := h.rows(); !strings.Contains(got, "▌     ○ Child") {
				t.Fatalf("child of %q should be a task:\n%s", parent, got)
			}
		}
		h := start(t)
		h.press("a", `"Inbox task"`, "esc", "p", `"Inbox"`, "enter")
		h.want("▌ ○ Inbox task")
	})

	t.Run("existing state survives moves edits and restart", func(t *testing.T) {
		h := project(t)
		id := h.m.current()
		h.press("e", "tab", "esc") // Rendered as a task before moving under First.
		h.want("  ▾ Project", "    ○ First", "▌     ○ Second")
		h = open(t, h.path)
		h.m.seek(id)
		h.press("e")
		if got := string(h.m.edit.buf); got != "Second $open" {
			t.Fatalf("stored task after restart = %q", got)
		}
		h.press("esc", "<", ">")
		h.want("  ▾ Project", "    ○ First", "▌     ○ Second")
		h = project(t) // The restart harness reuses fixture IDs; create new rows in a new database.
		h.press("o", "tab", `"Note"`, "esc", "e", "shift+tab", "esc", "e")
		if got := string(h.m.edit.buf); got != "Note $note" {
			t.Fatalf("existing note changed when outdented during editing: %q", got)
		}
	})

	for _, tc := range []struct{ paste, line, state, glyph string }{
		{"[ ] child", "child $open", "open", "○"},
		{"[x] child", "child $done", "done", "✓"},
		{"- [ ] child", "child $open", "open", "○"},
		{"* [X] child", "child $done", "done", "✓"},
		{"+ [ ] child $done", "child $done $open", "open", "○"},
		{"• [x] child $open", "child $open $done", "done", "✓"},
	} {
		t.Run("paste "+tc.paste, func(t *testing.T) {
			h := project(t)
			h.press("o", "tab")
			id, steps := h.m.edit.row, len(h.m.undo)
			h.m.Update(tea.PasteMsg{Content: tc.paste})
			if h.m.edit == nil || h.m.edit.row != id || string(h.m.edit.buf) != tc.line || len(h.m.undo) != steps {
				t.Fatal("single checkbox paste must normalize in the same open editor")
			}
			h.press("esc")
			fields := h.m.tree.Row(id).Fields
			if fields["text"] != "child" || fields["state"] != tc.state {
				t.Fatalf("pasted fields = %v", fields)
			}
			h.want("  ▾ Project", "    ○ First", "    ○ Second", "▌     "+tc.glyph+" child")
			h.press("u")
			h.want("  ▾ Project", "    ○ First", "▌   ○ Second")
		})
	}

	t.Run("ordinary insertion keeps spaces at the caret", func(t *testing.T) {
		h := project(t)
		h.press("e", "ctrl+a", "right")
		h.m.Update(tea.PasteMsg{Content: "  plain   text  "})
		if got := string(h.m.edit.buf); got != "S  plain   text  econd" {
			t.Fatalf("ordinary paste buffer = %q", got)
		}
		h.press("esc")
	})
}

func TestCaptureUsesTheDestinationDefault(t *testing.T) {
	h := start(t)
	h.press("o", `"Task $open"`, "esc", "a", `"Detail %Task"`, "esc", "p", `"Task"`, "enter")
	h.want("▌   Detail")
	h.press("e")
	if got := string(h.m.edit.buf); got != "Detail" {
		t.Fatalf("captured child of task = %q", got)
	}
}

// outlineToGather is a project of five rows, one holding a child, and a
// second project with one task.
func outlineToGather(t *testing.T) *harness {
	h := start(t)
	h.press("o", `"Alpha"`, "enter", "tab", `"A1 $open"`, "enter", `"A2 $open"`, "enter", "tab", `"A2a $open"`, "enter", "shift+tab", `"A3 $open"`, "enter", `"A4 $open"`, "esc")
	h.press("g", "g", "O", `"Beta"`, "enter", "tab", `"B1 $open"`, "esc")
	return h
}

func TestMoveHereGathersTheSelection(t *testing.T) {
	t.Run("scattered rows gather after the cursor", func(t *testing.T) {
		h := outlineToGather(t)
		h.press("p", `"alpha"`, "enter", "v", "j", "j", "v", "k", "k")
		h.want(
			" ┃○ A1",
			"  ○ A2",
			"▌   ○ A2a",
			" ┃○ A3",
			"  ○ A4",
		)
		h.press("m")
		if got := h.status(); got != "Move to ›    here · Inbox · Beta · Alpha" {
			t.Fatalf("picker line = %q", got)
		}
		steps := len(h.m.undo)
		h.press("enter")
		if h.m.msg != "moved here" || len(h.m.undo) != steps+1 {
			t.Fatalf("msg = %q, undo steps %d", h.m.msg, len(h.m.undo)-steps)
		}
		// The cursor stays on its row, and the selection is kept.
		h.want(
			"  ○ A2",
			"▌   ○ A2a",
			" ┃  ○ A1",
			" ┃  ○ A3",
			"  ○ A4",
		)
		// One undo puts every row back.
		h.press("u")
		h.want(
			"▌┃○ A1",
			"  ○ A2",
			"    ○ A2a",
			" ┃○ A3",
			"  ○ A4",
		)
	})

	t.Run("an open parent takes them as first children", func(t *testing.T) {
		h := outlineToGather(t)
		h.press("p", `"alpha"`, "enter", "v", "j", "j", "j", "v", "k", "k", "k", "m", "enter")
		h.want(
			"▌ ○ A2",
			" ┃  ○ A1",
			" ┃  ○ A4",
			"    ○ A2a",
			"  ○ A3",
		)
	})

	t.Run("a folded parent takes them after it", func(t *testing.T) {
		h := outlineToGather(t)
		h.press("p", `"alpha"`, "enter", "v", "j", "j", "j", "v", "k", "k", "k", "h", "m", "enter")
		h.want(
			"▌ ○ A2 +1",
			" ┃○ A1",
			" ┃○ A4",
			"  ○ A3",
		)
	})

	t.Run("across projects in Projects", func(t *testing.T) {
		h := outlineToGather(t)
		// A2a is selected under A2, so it travels with it.
		h.press("j", "j", "j", "v", "v", "g", "g", "j", "m", "enter")
		h.want(
			"  ▾ Beta",
			"▌   ○ B1",
			" ┃  ○ A2",
			" ┃    ○ A2a",
			"  ▾ Alpha",
			"    ○ A1",
			"    ○ A3",
			"    ○ A4",
		)
		h.press("u")
		h.want(
			"  ▾ Beta",
			"    ○ B1",
			"  ▾ Alpha",
			"    ○ A1",
			"▌┃  ○ A2",
			" ┃    ○ A2a",
			"    ○ A3",
			"    ○ A4",
		)
	})

	t.Run("not offered on or under the selection", func(t *testing.T) {
		h := outlineToGather(t)
		h.press("p", `"alpha"`, "enter", "j", "v", "m")
		if got := h.status(); got != "Move to ›    Inbox · Beta · Alpha" {
			t.Fatalf("under the selection: %q", got)
		}
		h.press("esc", "k", "m")
		if got := h.status(); got != "Move to ›    Inbox · Beta · Alpha" {
			t.Fatalf("on the selection: %q", got)
		}
	})

	t.Run("not offered in a filter or a list, nor without a selection", func(t *testing.T) {
		h := outlineToGather(t)
		h.press("p", `"alpha"`, "enter", "m")
		if got := h.status(); got != "Move to ›    Inbox · Beta · Alpha" {
			t.Fatalf("no selection: %q", got)
		}
		h.press("esc", "/", `"A"`, "enter", "j", "v", "m")
		if got := h.status(); got != "Move to ›    Inbox · Beta · Alpha" {
			t.Fatalf("filter: %q", got)
		}
		h.press("esc", "esc", "esc", "t", "j", "t", "p", `"today"`, "enter", "v", "m")
		if got := h.status(); got != "Move to ›    Inbox · Beta · Alpha" {
			t.Fatalf("list: %q", got)
		}
	})

	t.Run("typing a name still moves to it", func(t *testing.T) {
		h := outlineToGather(t)
		h.press("p", `"alpha"`, "enter", "v", "j", "m", `"beta"`)
		if got := h.status(); got != "Move to › beta   Beta" {
			t.Fatalf("picker line = %q", got)
		}
		h.press("enter")
		if h.m.msg != "moved to Beta" {
			t.Fatalf("msg = %q", h.m.msg)
		}
	})
}
