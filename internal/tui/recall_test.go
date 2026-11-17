package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/sigil"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (h *harness) field(text, name string) string {
	h.t.Helper()
	for _, e := range h.m.rowNames {
		if e.label == text {
			return h.m.tree.Row(e.id).Fields[name]
		}
	}
	h.t.Fatalf("no row reads %q", text)
	return ""
}

func TestAStampMovesADayFromWhereItIs(t *testing.T) {
	h := project(t)
	h.press("@", `"2026-12-30"`, "enter")
	h.press("@", "backspace", `"@++1w"`, "enter")
	if got := h.field("Second", sigil.FieldDue); got != "2027-01-06" {
		t.Fatalf("due = %q", got)
	}
	// The dot key reads the stamp again against the row, not against today.
	h.press(".")
	if got := h.field("Second", sigil.FieldDue); got != "2027-01-13" {
		t.Fatalf("after .: due = %q", got)
	}
	h.press("@", "backspace", `"@--3d"`, "enter")
	if got := h.field("Second", sigil.FieldDue); got != "2027-01-10" {
		t.Fatalf("after --: due = %q", got)
	}
	h.press("u")
	if got := h.field("Second", sigil.FieldDue); got != "2027-01-13" {
		t.Fatalf("after undo: due = %q", got)
	}
	// A month keeps the day, or the last day of a shorter month.
	h.press("@", `"2027-01-31"`, "enter", "@", "backspace", `"@++1m"`, "enter")
	if got := h.field("Second", sigil.FieldDue); got != "2027-02-28" {
		t.Fatalf("a month on: due = %q", got)
	}
	// A row with no day is left alone, among rows that have one.
	h.press("k", "V", "j", "@", "backspace", `"@++1d"`, "enter")
	if h.field("First", sigil.FieldDue) != "" || h.field("Second", sigil.FieldDue) != "2027-03-01" {
		t.Fatalf("due = %q, %q", h.field("First", sigil.FieldDue), h.field("Second", sigil.FieldDue))
	}
	h.press("esc", "k", "@", "backspace", `"@++1d"`, "enter")
	if !strings.Contains(h.status(), "no day to move") {
		t.Fatalf("status = %q", h.status())
	}
	// The start day moves the same way, and a removal cannot.
	h.press("j", "@", `"@mon"`, "enter", "@", `"@++2w"`, "enter")
	if got := h.field("Second", sigil.FieldStart); got != "2026-11-30" {
		t.Fatalf("start = %q", got)
	}
	h.press("-", `"@++1w"`, "enter")
	if h.m.msg != "cannot read -@++1w: -@, -@@ and -* take the whole field off" {
		t.Fatalf("msg = %q", h.m.msg)
	}
}

func TestPromptsRecallWhatTheyWereGiven(t *testing.T) {
	h := branches(t)
	typed := func() string { return string(h.m.ask.ed.buf) }
	h.press("/", `"milk"`, "enter", "esc", "/", `"gamma"`, "enter", "esc")
	h.press("/", "up")
	if typed() != "gamma" || h.shown() != "Gamma" {
		t.Fatalf("up gave %q showing %q", typed(), h.shown())
	}
	h.press("up")
	if typed() != "milk" {
		t.Fatalf("second up gave %q", typed())
	}
	h.press("up", "down", "down")
	if typed() != "" || len(h.m.vis) != 9 {
		t.Fatalf("down gave %q", typed())
	}
	// Only lines beginning with what was typed come back.
	h.press(`"m"`, "up")
	if typed() != "milk" {
		t.Fatalf("m then up gave %q", typed())
	}
	h.press("up", "down")
	if typed() != "m" {
		t.Fatalf("back down gave %q", typed())
	}
	h.press("esc")

	h.command("numbers")
	h.command("numbers")
	// A command line recalls once its name is finished; before that the
	// same keys complete the name.
	h.command("sort @")
	h.press(":", `"sort "`, "up")
	if typed() != "sort @" {
		t.Fatalf("command up gave %q", typed())
	}
	h.press("esc", "#", `"home"`, "enter", "@", `"fri"`, "enter", "#", "up")
	if typed() != "#home" {
		t.Fatalf("stamp up gave %q", typed())
	}
	// A picker's arrows still walk its matches.
	h.press("esc", "f", `"a"`, "down")
	if h.m.ask.choice != 1 || typed() != "a" {
		t.Fatalf("picker: choice %d, typed %q", h.m.ask.choice, typed())
	}
	h.press("esc")

	// The lines are this machine's and outlast the session.
	h = reopen(t, h)
	h.press("/", "up")
	if got := string(h.m.ask.ed.buf); got != "gamma" {
		t.Fatalf("after a restart up gave %q", got)
	}
}

func TestAFilterFindsTheRowsThatLinkHere(t *testing.T) {
	h := branches(t)
	h.press("j", "L", `"Gamma"`, "enter") // a1 → Gamma
	h.press("3", "j", "L", `"Gamma"`, "enter")
	h.press("2", "j", "L", `"a1"`, "enter") // b1 milk → a1
	for _, step := range []struct{ on, query, want string }{
		{"Gamma", "&.", "a1, a3"},
		{"a1", "&.", "b1 milk"},
		{"Beta:", "&.", ""},
		{"Beta:", "&", "a1, a3, b1 milk"},
		{"Beta:", "&gam", "a1, a3"},
		{"Beta:", "&gam|a1", "a1, a3, b1 milk"},
		{"Beta:", "-& milk", "Delta milk:"},
	} {
		h.press("g", "g")
		for h.on() != step.on {
			h.press("j")
		}
		h.press("/", `"`+step.query+`"`, "tab")
		if got := h.shown(); got != step.want {
			t.Fatalf("on %s, /%s showed %q, want %q", step.on, step.query, got, step.want)
		}
		h.press("tab", "esc")
	}
	// n walks the same rows once the filter is dropped.
	h.press("g", "g", "7", "j", "/", `"&."`, "enter", "esc", "g", "g", "n")
	if h.on() != "a1" {
		t.Fatalf("n landed on %q", h.on())
	}
	h.press("n")
	if h.on() != "a3" {
		t.Fatalf("second n landed on %q", h.on())
	}
}

func TestTheLastCopyOutlivesADelete(t *testing.T) {
	h := outlineOfThree(t)
	h.press("g", "p")
	if !strings.Contains(h.status(), "nothing copied to put") {
		t.Fatalf("status = %q", h.status())
	}
	h.press("y", "y", "k", "d", "d")
	h.want("  · Alpha", "▌ · Gamma")
	h.press("g", "p")
	h.want("  · Alpha", "  · Gamma", "▌ · Gamma")
	h.press("p")
	h.want("  · Alpha", "  · Gamma", "  · Gamma", "▌ · Beta")
	h.press("g", "g", "g", "P")
	h.want("▌ · Gamma", "  · Alpha", "  · Gamma", "  · Gamma", "  · Beta")
	h.press("G", ".")
	h.want("  · Gamma", "  · Alpha", "  · Gamma", "  · Gamma", "▌ · Gamma", "  · Beta")
}

func TestOperatorsTakeRowObjects(t *testing.T) {
	h := branches(t)
	h.press("y", "i", "c")
	if h.status() != "copied 4 rows" || len(h.m.vis) != 9 {
		t.Fatalf("yic: %q", h.status())
	}
	h.press("d", "i", "c")
	if got := h.shown(); got != "Alpha:, Beta:, b1 milk, Gamma, Delta milk:" || h.on() != "Alpha:" {
		t.Fatalf("dic left %q on %q", got, h.on())
	}
	h.press("u", "g", "g", "<", "i", "c")
	h.want(
		"▌ · Alpha:",
		"  ○ a1",
		"  ○ a2",
		"    · deep",
		"  ○ a3",
		"  ▾ Beta: 0/1",
		"    ○ b1 milk",
		"  ○ Gamma",
		"  · Delta milk:",
	)
	h.press("u", "g", "g", "j", "y", "i", "s")
	if h.status() != "copied 4 rows" {
		t.Fatalf("yis: %q", h.status())
	}
	h.press("d", "i", "s")
	if got := h.shown(); got != "Alpha:, Beta:, b1 milk, Gamma, Delta milk:" {
		t.Fatalf("dis left %q", got)
	}
	// The dot key takes the object again where the cursor is now.
	h.press("j", ".")
	if got := h.shown(); got != "Alpha:, Beta:, Gamma, Delta milk:" {
		t.Fatalf(". left %q", got)
	}
	h.press("d", "i", "c")
	if h.status() != "no rows under this one" {
		t.Fatalf("status = %q", h.status())
	}
	// A pending i says so, and anything but an object cancels.
	h.press("d", "i")
	if h.m.pending() != "di" {
		t.Fatalf("pending = %q", h.m.pending())
	}
	h.press("x")
	if got := h.shown(); got != "Alpha:, Beta:, Gamma, Delta milk:" || h.m.operator != "" {
		t.Fatalf("dix changed %q", got)
	}
	// A selection and a filter refuse.
	h.press("v", "d", "i", "s")
	if !strings.Contains(h.status(), "cannot act on a selection") {
		t.Fatalf("status = %q", h.status())
	}
	h.press("esc", "/", `"gamma"`, "enter", "d", "i", "s")
	if !strings.Contains(h.m.msg, "open the row first") || len(h.m.tree.Children(h.m.at.root)) != 4 {
		t.Fatalf("msg = %q", h.m.msg)
	}
}

func TestARowReachedFromAfarIsShownMidPane(t *testing.T) {
	h := countOutline(t, 40)
	at := func() int {
		for i, l := range strings.Split(h.rows(), "\n") {
			if strings.HasPrefix(l, "▌") {
				return i
			}
		}
		return -1
	}
	// Ten rows fit. A motion scrolls no further than it must.
	h.press("2", "5", "j")
	if at() != 9 {
		t.Fatalf("25j drew the cursor on line %d", at())
	}
	h.press("g", "g", "/", `"Task 30"`, "enter", "esc")
	if h.on() != "Task 30" || at() != 4 {
		t.Fatalf("dropping the filter drew %q on line %d", h.on(), at())
	}
	h.press("g", "g", "n")
	if h.on() != "Task 30" || at() != 4 {
		t.Fatalf("n drew %q on line %d", h.on(), at())
	}
	// A row already on screen is not scrolled.
	h.press("/", `"30|31"`, "enter", "esc", "n")
	if h.on() != "Task 31" || at() != 5 {
		t.Fatalf("n to the next row drew %q on line %d", h.on(), at())
	}
	// A jump, and the trail back, land the same way.
	h.press("f", `"Task 12"`, "enter")
	if h.on() != "Task 12" || at() != 4 {
		t.Fatalf("f drew %q on line %d", h.on(), at())
	}
	h.press("ctrl+o")
	if h.on() != "Task 31" || at() != 4 {
		t.Fatalf("ctrl+o drew %q on line %d", h.on(), at())
	}
	// Near an end there is no gap to leave.
	h.press("f", `"Task 39"`, "enter")
	if h.on() != "Task 39" || at() != 9 {
		t.Fatalf("the last row drew on line %d", at())
	}
}

func TestImportReadsWhatExportWrote(t *testing.T) {
	h := work(t)
	h.press("a", `"Loose end #home"`, "enter", "esc")
	out := filepath.Join(t.TempDir(), "out.txt")
	h.command("export " + out)

	fresh := start(t).wide()
	fresh.press("a", `"Already here"`, "enter", "esc")
	fresh.command("import " + out)
	if fresh.m.msg != "imported 7 rows from "+out {
		t.Fatalf("msg = %q", fresh.m.msg)
	}
	fresh.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"▌   ○ Refactor auth #bug",
	)
	// The Inbox line is the Inbox again, not a project of that name.
	fresh.press("f", "enter")
	fresh.want("▌ ○ Already here", "  ○ Loose end #home")
	back := filepath.Join(t.TempDir(), "back.txt")
	fresh.command("export " + back)
	if a, b := readFile(t, out), readFile(t, back); strings.Replace(b, "\tAlready here\n", "", 1) != a {
		t.Fatalf("round trip:\n%s\nwant:\n%s", b, a)
	}
	// It is one undo step.
	fresh.press("u", "f", `"Projects"`, "enter")
	if len(fresh.m.vis) != 0 || len(fresh.m.tree.Children(inboxID)) != 1 {
		t.Fatalf("undo left:\n%s", fresh.rows())
	}

	// Inside a project the lines are rows after the cursor, Inbox included.
	h.press("f", `"Alpha"`, "enter")
	h.command("import " + out)
	if got := h.shown(); !strings.HasPrefix(got, "Fix login, Inbox, Loose end, Beta, Fix cache, Alpha, Fix login") {
		t.Fatalf("imported into a project: %q", got)
	}
	h.press("u")
	for _, refused := range []string{"import", "import " + filepath.Join(t.TempDir(), "missing.txt")} {
		h.command(refused)
		if len(h.m.vis) != 3 || h.m.msg == "" {
			t.Fatalf(":%s gave %q", refused, h.m.msg)
		}
	}
	h.press("/", `"docs"`, "enter")
	h.command("import " + out)
	if h.m.msg != "import into an outline, with nothing selected" {
		t.Fatalf("msg = %q", h.m.msg)
	}
}
