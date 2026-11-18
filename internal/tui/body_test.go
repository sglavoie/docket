package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/sigil"
)

// inProject opens the project made by project(t), its two tasks in the
// pane and the cursor on the first.
func inProject(t *testing.T) *harness {
	h := project(t)
	h.press("gg", "enter")
	h.want(
		"▌ ○ First",
		"  ○ Second",
	)
	return h
}

func (h *harness) body() string {
	return h.m.tree.Row(h.m.at.root).Fields[fieldBody]
}

// k on the first row steps into the body of the row the pane opens, which
// then takes the whole pane. Nothing in it is read as a sigil. Left, it
// heads the rows folded to its first line, and the row says it has a body.
// Keeping it is one undo step.
func TestABodyIsWrittenAboveTheRows(t *testing.T) {
	h := inProject(t)
	h.press("k")
	h.want(
		"    no body · A writes one",
	)
	if !strings.HasPrefix(h.status(), "in the body") {
		t.Fatalf("status says %q", h.status())
	}
	h.press("A", `"Move the reporting DB first."`, "enter", "enter", `"Ask @monday about #ops !"`, "esc")
	h.want(
		"    Move the reporting DB first.",
		"    Ask @monday about #ops !",
	)
	// The blank line between them is kept and drawn.
	if lines := h.lines(); strings.TrimSpace(lines[2]) != "" {
		t.Fatalf("the blank line is not drawn: %q", lines[1:4])
	}
	h.press("esc")
	if got, want := h.body(), "Move the reporting DB first.\n\nAsk @monday about #ops !"; got != want {
		t.Fatalf("body is %q, want %q", got, want)
	}
	if r := h.m.tree.Row(h.m.at.root); r.Fields[sigil.FieldTags] != "" || r.Fields[sigil.FieldDue] != "" || r.Fields[sigil.FieldFlag] != "" {
		t.Fatalf("the body set fields: %q", r.Fields)
	}
	h.want(
		"    Move the reporting DB first. +2",
		"    ──────────────────────────────────",
		"▌ ○ First",
		"  ○ Second",
	)
	h.press("esc")
	h.want(
		"▌ ▾ Project ≡",
		"    ○ First",
		"    ○ Second",
	)
	h.press("u")
	if h.status() != "undo: body changed on 1 row" {
		t.Fatalf("undo says %q", h.status())
	}
	h.want(
		"▌ ▾ Project",
		"    ○ First",
		"    ○ Second",
	)
}

// Projects and the lists open no row, so they have no body to step into.
func TestOnlyARowsPaneHasABody(t *testing.T) {
	h := project(t)
	h.press("gg", "k")
	if h.m.edit != nil {
		t.Fatal("Projects has no body")
	}
	h.press("enter", "/", `"First"`, "enter", "gg", "k")
	if h.m.edit != nil {
		t.Fatal("a filter shows no body")
	}
}

// Inside the body the grammar is the row's, over lines too: j and k by line,
// staying in the body until its last line, which returns to the rows.
func TestTheBodyIsEditedByLines(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"one"`, "enter", `"two"`, "enter", `"three"`, "esc")
	// dd deletes the line, p puts it back as a line.
	h.press("k", `"dd"`)
	if h.body() != "one\nthree" && h.m.edit != nil && string(h.m.edit.buf) != "one\nthree" {
		t.Fatalf("dd left %q", string(h.m.edit.buf))
	}
	h.press("p")
	if got := string(h.m.edit.buf); got != "one\nthree\ntwo" {
		t.Fatalf("p put %q", got)
	}
	// o opens a line below, O above; J joins the next line on.
	h.press("gg", "o", `"half"`, "esc", "k", "J")
	if got := string(h.m.edit.buf); got != "one half\nthree\ntwo" {
		t.Fatalf("o and J made %q", got)
	}
	// $ and 0 keep to the line, and D deletes to its end.
	h.press("0", "w", "D")
	if got := string(h.m.edit.buf); got != "one \nthree\ntwo" {
		t.Fatalf("D made %q", got)
	}
	// G and k stay in the body; j past its last line returns to the rows.
	h.press("G", "j")
	if h.m.edit != nil {
		t.Fatal("j past the last line stays in the body")
	}
	// A line keeps the blank typed at its end; only the body's end drops them.
	if h.body() != "one \nthree\ntwo" {
		t.Fatalf("the body kept is %q", h.body())
	}
	h.want(
		"    one +2",
		"    ──────────────────────────────────",
		"▌ ○ First",
		"  ○ Second",
	)
}

// Paragraphs are the lines between empty ones: } and { move by them, and ip
// and ap take one whole.
func TestParagraphsInTheBody(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"a1"`, "enter", `"a2"`, "enter", "enter", `"b1"`, "enter", `"b2"`, "enter", "enter", `"c1"`, "esc")
	h.press("gg", "}")
	if e := h.m.edit; e.caret != 6 {
		t.Fatalf("} went to %d", e.caret)
	}
	h.press("j", `"dap"`)
	if got := string(h.m.edit.buf); got != "a1\na2\n\nc1" {
		t.Fatalf("dap left %q", got)
	}
	h.press("gg", `"yip"`, "G", "p")
	if got := string(h.m.edit.buf); got != "a1\na2\n\nc1\na1\na2" {
		t.Fatalf("yip p made %q", got)
	}
	h.press("{")
	if e := h.m.edit; e.caret != lineStart(e.buf, 6) {
		t.Fatalf("{ went to %d", e.caret)
	}
}

// Sentences end at a full stop, a question or an exclamation mark.
func TestSentencesInTheBody(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"One here. Two there? Three."`, "esc", "0", ")")
	if e := h.m.edit; e.caret != 10 {
		t.Fatalf(") went to %d", e.caret)
	}
	h.press(")", "(", "(")
	if e := h.m.edit; e.caret != 0 {
		t.Fatalf("( went to %d", e.caret)
	}
}

// A paste into the body is text: lines and blanks kept, no rows made, no
// bullets read.
func TestAPasteIntoTheBodyIsText(t *testing.T) {
	h := inProject(t)
	h.press("k", "A")
	h.pasteText("- first\n\n  - [ ] second\n")
	h.press("esc", "esc")
	if got := h.body(); got != "- first\n\n  - [ ] second" {
		t.Fatalf("body is %q", got)
	}
	h.press("k", `"zo"`, "esc")
	h.want(
		"    - first",
		"      - [ ] second",
		"    ──────────────────────────────────",
		"▌ ○ First",
		"  ○ Second",
	)
}

// A body folds to its first line above the rows unless it was opened: zo
// inside it keeps it open, zc folds it again, za turns it over. While the
// caret is in it, it takes the whole pane.
func TestABodyFoldsAboveTheRows(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"Summary line"`, "enter", `"detail one"`, "enter", `"detail two"`, "esc")
	h.want(
		"    Summary line",
		"    detail one",
		"    detail two",
	)
	h.press(`"zo"`)
	if h.status() != "the body stays open above the rows" {
		t.Fatalf("zo says %q", h.status())
	}
	h.press("esc")
	h.want(
		"    Summary line",
		"    detail one",
		"    detail two",
		"    ──────────────────────────────────",
		"▌ ○ First",
		"  ○ Second",
	)
	h.press("k", `"za"`, "esc")
	h.want(
		"    Summary line +2",
		"    ──────────────────────────────────",
		"▌ ○ First",
		"  ○ Second",
	)
	// Opening it is kept on this machine, as folds are.
	h.press("k", `"zo"`, "esc")
	h.st.Close()
	again := open(t, h.path)
	again.press("gg", "enter")
	again.want(
		"    Summary line",
		"    detail one",
		"    detail two",
		"    ──────────────────────────────────",
		"▌ ○ First",
		"  ○ Second",
	)
}

// A filter reads bodies; = holds a word to the row's own line.
func TestTheFilterReadsBodies(t *testing.T) {
	h := project(t)
	h.press("gg", "j", "enter", "k", "A", `"Ask about the cutover window"`, "esc", "esc", "esc", "gg")
	h.want(
		"▌ ▾ Project",
		"    ○ First ≡",
		"    ○ Second",
	)
	h.press("/", `"cutover"`)
	h.want(
		"  ▾ Project",
		"▌   ○ First ≡",
	)
	h.press("esc", "/", `"=cutover"`)
	if got := parseQuery("=cutover", h.m.today())[0].any[0].words(h.m.today()); got != `"cutover" on the row's line` {
		t.Fatalf("read back as %q", got)
	}
	if h.rows() != "" && strings.Contains(h.rows(), "First") {
		t.Fatalf("= read the body:\n%s", h.rows())
	}
	h.press("esc", "/", `"first -=first"`)
	if strings.Contains(h.rows(), "First") {
		t.Fatalf("a word on the line counts as on the line:\n%s", h.rows())
	}
}

// Copy and put carry a body as > lines under its row; a row whose text
// starts with > is kept apart from them.
func TestCopyCarriesTheBody(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"line one"`, "enter", "enter", `"  indented"`, "esc", "esc")
	h.press("esc")
	text, _ := h.m.writeRows([]string{h.m.current()})
	want := "Project\n\t> line one\n\t>\n\t>   indented\n\tFirst\n\tSecond\n"
	if text != want {
		t.Fatalf("written as %q, want %q", text, want)
	}
	h.press(`"yyp"`)
	h.want(
		"  ▾ Project ≡",
		"    ○ First",
		"    ○ Second",
		"  ▾ Project ≡",
		"    ○ First",
		"▌   ○ Second",
	)
	if got := h.m.tree.Row(h.m.tree.Row(h.m.current()).Parent).Fields[fieldBody]; got != "line one\n\n  indented" {
		t.Fatalf("put body is %q", got)
	}

	h.press(`"cc>quoted"`, "esc")
	text, _ = h.m.writeRows([]string{h.m.current()})
	if text != `\>quoted`+"\n" {
		t.Fatalf("a row starting with > is written %q", text)
	}
	h.press(`"yyp"`)
	if got := h.m.tree.Row(h.m.current()).Text(); got != ">quoted" {
		t.Fatalf("it comes back as %q", got)
	}
}

// A body taller than the pane gives up its first lines to keep the cursor's
// row on screen, and scrolls by its caret while it is edited.
func TestATallBodyKeepsTheRowsInSight(t *testing.T) {
	h := inProject(t)
	h.press("k", "A")
	var lines []string
	for i := range 30 {
		lines = append(lines, "line "+string(rune('a'+i%26)))
	}
	h.pasteText(strings.Join(lines, "\n"))
	h.press("esc", "esc")
	rows := h.rows()
	if !strings.Contains(rows, "▌ ○ First") {
		t.Fatalf("the cursor's row is off screen:\n%s", rows)
	}
	h.press("k", "gg")
	if v := h.m.View(); v.Cursor == nil || v.Cursor.Y != 1 {
		t.Fatalf("the caret is not on the body's first line: %+v", v.Cursor)
	}
	if !strings.HasPrefix(strings.TrimSpace(h.lines()[1]), "line a") {
		t.Fatalf("the body did not scroll to its start: %q", h.lines()[1])
	}
}

// A body is stored like any field, so it survives a restart, and undo of it
// does too.
func TestABodySurvivesARestart(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"kept"`, "enter", `"across"`, "esc", "esc", "esc")
	h.st.Close()

	again := open(t, h.path)
	again.press("gg")
	again.want(
		"▌ ▾ Project ≡",
		"    ○ First",
		"    ○ Second",
	)
	if got := again.m.tree.Row(again.m.current()).Fields[fieldBody]; got != "kept\nacross" {
		t.Fatalf("the body came back as %q", got)
	}
	again.press("u")
	if again.status() != "undo: body changed on 1 row" {
		t.Fatalf("undo after a restart says %q", again.status())
	}
}

// Esc ends typing in the body, then leaves it; u inside takes back one change
// at a time, as in a row.
func TestEscAndUndoInsideTheBody(t *testing.T) {
	h := inProject(t)
	h.press("k", "A", `"draft"`, "esc")
	if h.m.edit == nil || !h.m.edit.normal {
		t.Fatal("Esc after typing left the body")
	}
	h.press(`"dd"`, "u")
	if got := string(h.m.edit.buf); got != "draft" {
		t.Fatalf("u inside the body left %q", got)
	}
	h.press("esc")
	if h.m.edit != nil || h.body() != "draft" {
		t.Fatalf("Esc kept %q", h.body())
	}
	if !strings.HasPrefix(h.rows(), "    draft") {
		t.Fatalf("screen:\n%s", h.rows())
	}
}

// On the first row of a row's pane the status line names the way into its
// body, which nothing on screen shows otherwise.
func TestTheFirstRowNamesTheBody(t *testing.T) {
	h := inProject(t)
	if h.status() != "k writes its body" {
		t.Fatalf("status says %q", h.status())
	}
	h.press("j")
	if h.status() != "" {
		t.Fatalf("the hint stays off the first row: %q", h.status())
	}
	h.press("k", "3")
	if h.status() != "3" {
		t.Fatalf("a count hides the hint: %q", h.status())
	}
	h.press("esc", "k", "A", `"notes"`, "esc", "esc")
	if h.status() != "k edits its body" {
		t.Fatalf("with a body, status says %q", h.status())
	}
	// Projects opens no row, so it has no body to name.
	h.press("esc", "gg")
	if h.status() != "" {
		t.Fatalf("Projects names a body: %q", h.status())
	}
}
