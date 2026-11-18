package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/store"
)

func (h *harness) command(text string) { h.press(":", `"`+text+`"`, "enter") }

func TestUpAndDownKeepTypingInTheNextRow(t *testing.T) {
	h := start(t)
	h.press("o", `"One"`, "enter", `"Two"`, "enter", `"Three"`, "esc")
	steps := len(h.m.undo)

	// Each arrow keeps the row and opens its neighbour at the same column.
	h.press("A", `"X"`, "up", `"Y"`, "up", "left", "left", `"Z"`)
	if h.m.edit == nil || string(h.m.edit.buf) != "OZne" {
		t.Fatalf("edit = %+v", h.m.edit)
	}
	// At either end the line stays as it is.
	h.press("up")
	if h.m.edit == nil || string(h.m.edit.buf) != "OZne" || h.m.cursor != 0 {
		t.Fatal("Up on the first row left the line")
	}
	h.press("down")
	if string(h.m.edit.buf) != "TwoY" || h.m.edit.caret != 2 {
		t.Fatalf("Down opened %q at %d", string(h.m.edit.buf), h.m.edit.caret)
	}
	h.press("esc")
	h.want("  · OZne", "▌ · TwoY", "  · ThreeX")
	if len(h.m.undo) != steps+3 {
		t.Fatalf("%d undo steps for three rows", len(h.m.undo)-steps)
	}
	h.press("u", "u", "u")
	h.want("  · One", "  · Two", "▌ · Three")

	// A fresh row left blank disappears on the way.
	h.press("k", "o", "up")
	if h.m.edit == nil || string(h.m.edit.buf) != "Two" {
		t.Fatalf("edit = %+v", h.m.edit)
	}
	h.press("esc", "G", "O", "down")
	if h.m.edit == nil || string(h.m.edit.buf) != "Three" {
		t.Fatalf("edit = %+v", h.m.edit)
	}
	h.press("esc")
	h.want("  · One", "  · Two", "▌ · Three")

	// A list keeps its rows, so the arrows walk it too.
	h.press("k", "enter", "o", `"Task"`, "enter", `"Other"`, "esc", "g", "g", "t", "j", "t")
	h.press("f", `"today"`, "enter", "A", `"!"`, "down", `"?"`, "esc")
	h.want("  ○ Task! · Two", "▌ ○ Other? · Two")
}

func TestAFilterShowsMatchesInsideFoldedRows(t *testing.T) {
	h := work(t)
	h.press("G", "h", "h", "h")
	h.want("  ▾ Beta", "    ○ Fix cache #bug", "▌ ▸ Alpha +3")
	h.press("/", `"#docs"`, "enter")
	h.want("  ▾ Alpha", "▌   ○ Write docs @fri #docs")
	// The outline's fold never hides a match: h on a row with nothing
	// under it steps to its parent.
	h.press("h")
	h.want("▌ ▾ Alpha", "    ○ Write docs @fri #docs")
	h.press("l")
	h.want("  ▾ Alpha", "▌   ○ Write docs @fri #docs")
	h.press("h", "h")
	// The filter folds on its own, counting the rows it found under the fold.
	h.want("▌ ▸ Alpha +1")
	h.press("l")
	h.want("▌ ▾ Alpha", "    ○ Write docs @fri #docs")
	h.press("h")
	h.want("▌ ▸ Alpha +1")
	// Its folds are never saved, and the outline's are kept for afterwards.
	if folds, _ := h.st.Folds(); len(folds) != 1 {
		t.Fatalf("stored folds = %v", folds)
	}
	h.press("esc")
	h.want("  ▾ Beta", "    ○ Fix cache #bug", "▌ ▸ Alpha +3")
	// A changed query starts with no folds of its own.
	h.press("/", `"#docs"`, "enter")
	h.want("  ▾ Alpha", "▌   ○ Write docs @fri #docs")
}

func TestFilterAlternativesAndChangedSince(t *testing.T) {
	h := work(t)
	filter := func(q string, rows ...string) {
		t.Helper()
		h.press("esc", "/", `"`+q+`"`)
		if !h.m.at.flat {
			h.press("tab")
		}
		h.want(rows...)
		h.press("esc")
	}
	h.press("x") // Fix cache is done today
	filter("#docs|nothere", "▌ ○ Write docs @fri #docs · Alpha")
	filter("$done|doing", "▌ ✓ Fix cache #bug · Beta")
	filter("login|cache", "▌ ○ Fix login ! #bug · Alpha", "  ✓ Fix cache #bug · Beta")
	filter("-login|cache #bug", "▌ ○ Refactor auth #bug · Alpha")
	// An alternative the sigil cannot read leaves the word as text.
	filter("$done|zzz", "  Nothing matches.")

	// Ten days on, another row is closed: ~ reads when the state changed.
	h.clock = h.clock.Add(10 * 24 * time.Hour)
	h.press("G", "x")
	filter("$done ~7d", "▌ ✓ Refactor auth #bug ~today · Alpha")
	filter("$done ~today", "▌ ✓ Refactor auth #bug ~today · Alpha")
	filter("$done -~1w", "▌ ✓ Fix cache #bug · Beta")
	filter("$done ~2w", "▌ ✓ Refactor auth #bug ~today · Alpha", "  ✓ Fix cache #bug ~Nov 15 · Beta")
	filter("$done ~1m", "▌ ✓ Refactor auth #bug ~today · Alpha", "  ✓ Fix cache #bug ~Nov 15 · Beta")

	// The alternative being typed is completed like the first.
	h.press("/", `"$open|do"`, "right")
	if h.m.at.query != "$open|doing" {
		t.Fatalf("query = %q", h.m.at.query)
	}
}

func TestArchiveMovesClosedWorkUnderAHeading(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tBuild:\n\t\tShipped $done\n\t\t\tA detail\n\t\tHalf $done\n\t\t\tStill open $open\n\t\tLive\n\tDropped $canceled\nBeta\n\tOld $done")
	h.press("g", "g")
	steps := len(h.m.undo)
	h.command("archive")
	if h.m.msg != "archived 3 rows under Archive:" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.want(
		"▌ ▾ Alpha",
		"    ▾ Build: 1/3",
		"      ✓ Half",
		"        ○ Still open",
		"      ○ Live",
		"    ▾ Archive: 1/1",
		"      ✓ Shipped",
		"        · A detail",
		"      ✕ Dropped",
		"  ▾ Beta",
		"    ▾ Archive: 1/1",
		"      ✓ Old",
	)
	if len(h.m.undo) != steps+1 {
		t.Fatal("archive is one undo step")
	}
	// What is archived stays there, and a second pass reuses the heading.
	h.command("archive")
	if h.m.msg != "nothing closed to archive" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("j", "j", "j", "x")
	h.command("archive")
	h.want(
		"  ▾ Alpha",
		"    ▾ Build: 0/1",
		"      ○ Live",
		"    ▾ Archive: 3/3",
		"      ✓ Shipped",
		"        · A detail",
		"      ✕ Dropped",
		"      ✓ Half",
		"▌       ✓ Still open",
		"  ▾ Beta",
		"    ▾ Archive: 1/1",
		"      ✓ Old",
	)
	h.press("u", "u", "u", "g", "g")
	h.want(
		"▌ ▾ Alpha",
		"    ▾ Build: 2/4",
		"      ✓ Shipped",
		"        · A detail",
		"      ✓ Half",
		"        ○ Still open",
		"      ○ Live",
		"    ✕ Dropped",
		"  ▾ Beta",
		"    ✓ Old",
	)

	// Inside a project only that project is swept; lists and the Inbox refuse.
	h.press("G", "k", "enter")
	h.command("archive")
	h.want("  ▾ Archive: 1/1", "▌   ✓ Old")
	h.press("k", "enter")
	h.command("archive")
	if h.m.msg != "nothing closed to archive" {
		t.Fatalf("inside the archive: msg = %q", h.m.msg)
	}
	h.press("f", `"alpha"`, "enter")
	h.want("▌ ▾ Build: 2/4", "    ✓ Shipped", "      · A detail", "    ✓ Half", "      ○ Still open", "    ○ Live", "  ✕ Dropped")
	for _, place := range []string{"inbox", "waiting"} {
		h.press("f", `"`+place+`"`, "enter")
		gen := h.m.tree.Gen()
		h.command("archive")
		if h.m.tree.Gen() != gen || !strings.Contains(h.m.msg, "archive") {
			t.Fatalf("%s: msg = %q", place, h.m.msg)
		}
	}
}

func TestAWebAddressIsALink(t *testing.T) {
	lines := wrap([]word{{"", "see"}, {darkPalette.dim, "https://example.com/abcdef."}, {"", "http"}}, 12)
	if len(lines) != 3 {
		t.Fatalf("lines = %q", lines)
	}
	open := ansi.SetHyperlink("https://example.com/abcdef")
	for i, l := range lines {
		if strings.Count(l, open) != 1 || strings.Count(l, ansi.ResetHyperlink()) != 1 {
			t.Fatalf("line %d does not carry the whole address: %q", i, l)
		}
	}
	if !strings.HasPrefix(lines[0], "see "+open) || !strings.HasSuffix(lines[2], ansi.ResetHyperlink()+" http") {
		t.Fatalf("a plain word was linked: %q", lines)
	}
	if got := ansi.Strip(strings.Join(lines, "|")); got != "see https://|example.com/|abcdef. http" {
		t.Fatalf("text = %q", got)
	}
	for i, l := range lines {
		if ansi.StringWidth(l) > 12 {
			t.Fatalf("line %d is %d wide", i, ansi.StringWidth(l))
		}
	}
}

func TestCaptureFromOutsideTheApp(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Alpha\n\tTUI:\n\t\tExisting")
	// The command is another process: its own handle on the same database.
	st, err := store.Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tree, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	said, err := Capture(tree, st, []string{"Buy milk @fri #home", "", "Polish %tui !", "Stays %nowhere"})
	if err != nil || strings.Join(said, "|") != "added to Inbox|added to Alpha › TUI|added to Inbox" {
		t.Fatalf("said %q, err %v", said, err)
	}
	// The open window sees the rows at its next key.
	h.press("g", "g")
	h.want("▌ ▾ Alpha", "    ▾ TUI: 0/2", "      ○ Existing", "      ○ Polish !")
	h.press("f", `"inbox"`, "enter")
	h.want("▌ ○ Buy milk @fri #home", "  ○ Stays %nowhere")

	if said, err := Capture(tree, st, []string{"%tui"}); err == nil || len(said) != 0 {
		t.Fatalf("a line of nothing was added: %q", said)
	}
}
