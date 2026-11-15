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
