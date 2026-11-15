package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/store"
)

type harness struct {
	t    *testing.T
	path string
	st   *store.Store
	m    *Model
}

func open(t *testing.T, path string) *harness {
	t.Helper()
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	tree, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	folds, err := st.Folds()
	if err != nil {
		t.Fatal(err)
	}
	m := New(tree, st, folds)
	n := 0
	m.newID = func() string { n++; return fmt.Sprintf("%s-row%03d", filepath.Base(path), n) }
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	return &harness{t: t, path: path, st: st, m: m}
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
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	}
	r := []rune(tok)
	return tea.KeyPressMsg{Code: r[0], Text: tok}
}

// press sends named keys; a token in quotes is typed character by character.
func (h *harness) press(toks ...string) *harness {
	h.t.Helper()
	for _, tok := range toks {
		if len(tok) > 1 && strings.HasPrefix(tok, `"`) {
			for _, r := range strings.Trim(tok, `"`) {
				h.m.Update(key(string(r)))
			}
			continue
		}
		h.m.Update(key(tok))
	}
	return h
}

// rows is the list as drawn: no header, no status line, no styling.
func (h *harness) rows() string {
	h.t.Helper()
	lines := strings.Split(ansi.Strip(h.m.View().Content), "\n")
	var out []string
	for _, l := range lines[1 : len(lines)-1] {
		if l = strings.TrimRight(l, " "); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
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
		"  • Alpha",
		"  • Beta",
		"▌ • Gamma",
	)
	return h
}

func TestTypingAListChainsRows(t *testing.T) {
	h := outlineOfThree(t)
	// A fresh row left blank disappears and the cursor returns.
	h.press("o", "esc")
	h.want(
		"  • Alpha",
		"  • Beta",
		"▌ • Gamma",
	)
}

func TestIndentWhileTypingAndStructureKeys(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"First"`, "enter", `"Second"`, "esc")
	h.want(
		"  ▾ Project",
		"    • First",
		"▌   • Second",
	)
	h.press("K")
	h.want(
		"  ▾ Project",
		"▌   • Second",
		"    • First",
	)
	h.press("<")
	h.want(
		"  ▾ Project",
		"    • First",
		"▌ • Second",
	)
	h.press(">")
	h.want(
		"  ▾ Project",
		"    • First",
		"▌   • Second",
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
		"▌   • Top",
		"    • Child",
	)
	h.press("k", "h", "o", `"Sibling"`, "esc")
	h.want(
		"  ▸ Project",
		"▌ • Sibling",
	)
}

func TestFoldZoomAndBackOut(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Child"`, "esc")
	h.press("h")
	h.want(
		"▌ ▾ Project",
		"    • Child",
	)
	h.press("h")
	h.want("▌ ▸ Project")
	h.press("l", "l")
	h.want(
		"  ▾ Project",
		"▌   • Child",
	)
	h.press("k", "enter")
	h.want("▌ • Child")
	if got := ansi.Strip(strings.SplitN(h.m.View().Content, "\n", 2)[0]); got != "Project" {
		t.Fatalf("breadcrumb = %q", got)
	}
	h.press("esc")
	h.want(
		"▌ ▾ Project",
		"    • Child",
	)
}

func TestDeleteUndoRedo(t *testing.T) {
	h := outlineOfThree(t)
	h.press("k", "d", "d")
	h.want(
		"  • Alpha",
		"▌ • Gamma",
	)
	h.press("u")
	h.want(
		"  • Alpha",
		"▌ • Beta",
		"  • Gamma",
	)
	h.press("ctrl+r")
	h.want(
		"  • Alpha",
		"▌ • Gamma",
	)
	// An edit is one undo step, text and indent together.
	h.press("u", "e", `" two"`, "tab", "esc")
	h.want(
		"  ▾ Alpha",
		"▌   • Beta two",
		"  • Gamma",
	)
	h.press("u")
	h.want(
		"  • Alpha",
		"▌ • Beta",
		"  • Gamma",
	)
}

func TestEverythingSurvivesARestart(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Child"`, "enter", `"Gone"`, "esc", "d", "d")
	h.press("g", "g", "o", "esc", "O", `"Above"`, "esc", "j", "h")
	want := []string{
		"  • Above",
		"▌ ▸ Project",
	}
	h.want(want...)
	h.st.Close()

	again := open(t, h.path)
	again.press("j")
	again.want(want...)
	again.press("l")
	again.want(
		"  • Above",
		"▌ ▾ Project",
		"    • Child",
	)
}

func TestLongRowIsCutToTheWidthAndScrollsWhileTyping(t *testing.T) {
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
	h.press("esc")
	for _, line := range strings.Split(ansi.Strip(h.m.View().Content), "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatalf("line wider than the terminal: %q", line)
		}
	}
	if !strings.HasSuffix(h.rows(), "…") {
		t.Fatalf("long row is not marked as cut:\n%s", h.rows())
	}
}
