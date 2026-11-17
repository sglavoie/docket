package tui

import (
	"errors"
	"image/color"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

func TestTutorReads(t *testing.T) {
	h := start(t).wide()
	lines := readPaste(tutorText)
	if len(lines) < 35 || len(lines) > 50 || len(lines) != strings.Count(tutorText, "\n") {
		t.Fatalf("tutorial has %d parsed rows for %d lines", len(lines), strings.Count(tutorText, "\n"))
	}
	for i, line := range lines {
		if i == 0 {
			if line.level != 0 || line.text != "Welcome" {
				t.Fatalf("tutorial root = %+v", line)
			}
		} else if line.level < 1 || line.level > 2 {
			t.Fatalf("stray tutorial line %d: %+v", i+1, line)
		}
		for _, word := range strings.Fields(line.text) {
			for _, span := range h.m.styler(surfaceRow)(word) {
				if span.style == h.m.colors().warn {
					t.Errorf("tutorial line %d warns on %q", i+1, word)
				}
			}
		}
	}
	h.press(":", `"tutor"`, "enter")
	text, n := h.m.writeRows([]string{h.m.at.root})
	if n != len(lines) || text != tutorText {
		t.Fatalf("tutorial does not round-trip through import/export (%d rows):\n%s", n, text)
	}
	if got := h.m.tree.Row(h.m.current()).Text(); got != "Move, fold and zoom:" || !strings.Contains(h.rows(), "Press j and k") {
		t.Fatalf("tutorial did not open its first exercises: %s", h.rows())
	}
}

func TestTutorStepsHold(t *testing.T) {
	step := func(t *testing.T, prefix string) *harness {
		t.Helper()
		h := start(t).wide()
		h.press(":", `"tutor"`, "enter")
		for range readPaste(tutorText) {
			if strings.HasPrefix(h.m.tree.Row(h.m.current()).Text(), prefix) {
				return h
			}
			h.press("j")
		}
		t.Fatalf("tutorial step %q missing", prefix)
		return nil
	}
	for _, tc := range []struct {
		name, prefix, field, want string
		keys                      []string
	}{
		{"repeat flag", "Press ! here, j then .", "flag", "1", []string{"!", "j", "."}},
		{"sweep close", "Press V then j", "state", "done", []string{"V", "j", "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := step(t, tc.prefix)
			first := h.m.current()
			h.press(tc.keys...)
			second := h.m.current()
			a, b := h.m.tree.Row(first), h.m.tree.Row(second)
			if first == second || a.Parent != b.Parent || b.Fields["state"] == "" || strings.HasSuffix(b.Text(), ":") {
				t.Fatalf("step reached %q instead of a sibling task", b.Text())
			}
			for _, row := range []*outline.Row{a, b} {
				if row.Fields[tc.field] != tc.want {
					t.Errorf("%q: %s = %q, want %q", row.Text(), tc.field, row.Fields[tc.field], tc.want)
				}
			}
			if !strings.Contains(h.rows(), tc.prefix) || !strings.Contains(h.rows(), b.Text()) {
				t.Fatalf("exercise rows disappeared:\n%s", h.rows())
			}
		})
	}
	t.Run("split and join", func(t *testing.T) {
		h := step(t, "Press o and type two words")
		h.press("o", `"two words"`, "left", "enter", "backspace", "esc")
		if got := h.m.tree.Row(h.m.current()).Text(); got != "two word s" {
			t.Fatalf("join should insert a space at the split: %q", got)
		}
	})
	t.Run("sidebar travel", func(t *testing.T) {
		h := step(t, "Press Tab to focus the sidebar")
		welcome, row := h.m.at.root, h.m.current()
		h.press("tab", "k", "enter")
		if h.m.sideFocus || h.m.at.root == welcome {
			t.Fatal("k did not visit another place from Welcome")
		}
		h.press("ctrl+o")
		if h.m.at.root != welcome || h.m.current() != row {
			t.Fatal("Ctrl+O did not return to the tutorial step")
		}
	})
}

func TestTutorCommand(t *testing.T) {
	for _, where := range []string{"projects", "project", "zoom", "inbox", "list", "filter", "selection", "sidebar"} {
		t.Run(where, func(t *testing.T) {
			h := start(t).wide()
			h.pasteText("Existing\n\tHeading:\n\t\tTask\nLast")
			h.press("g", "g")
			switch where {
			case "project":
				h.press("enter")
			case "zoom":
				h.press("enter", "enter")
			case "inbox":
				h.press("f", "enter")
			case "list":
				h.press("f", `"Today"`, "enter")
			case "filter":
				h.press("/", `"Task"`, "tab", "enter")
			case "selection":
				h.press("v")
			case "sidebar":
				h.press("tab")
			}
			before, count := h.m.writeRows(h.m.tree.Children(""))
			steps := len(h.m.undo)
			h.press(":", `"tutor"`, "enter")
			root := h.m.at.root
			roots := h.m.tree.Children("")
			if root != roots[len(roots)-1] || h.m.tree.Row(root).Text() != "Welcome" || h.m.tree.Row(h.m.current()).Parent != root {
				t.Fatalf("tutorial not opened as last project: root=%q current=%q", root, h.m.current())
			}
			if h.m.sideFocus || h.m.at.query != "" || h.m.at.list != "" || len(h.m.picked) != 0 || len(h.m.zoom) != 0 {
				t.Fatal("tutorial retained the previous place or selection")
			}
			if len(h.m.undo) != steps+1 {
				t.Fatalf("tutorial added %d undo steps", len(h.m.undo)-steps)
			}
			h.press("u")
			if got, n := h.m.writeRows(h.m.tree.Children("")); got != before || n != count {
				t.Fatalf("one undo did not restore the outline:\n%s", got)
			}
			h.press("ctrl+r", ":", `"tutor"`, "enter")
			if h.m.at.root == root || !h.m.live(root) || len(h.m.undo) != steps+2 {
				t.Fatal("a second tutorial was not a fresh independent copy")
			}
		})
	}
	t.Run("save failure", func(t *testing.T) {
		h := start(t).wide()
		before, _ := h.m.writeRows(h.m.tree.Children(""))
		h.m.backend = &flaky{Backend: h.st, fail: true}
		h.press(":", `"tutor"`, "enter")
		after, _ := h.m.writeRows(h.m.tree.Children(""))
		if after != before || len(h.m.undo) != 0 || !strings.Contains(h.bottom(), "disk full") {
			t.Fatal("failed tutorial import was not rolled back")
		}
	})
}

func TestFirstLaunchSeedsTutor(t *testing.T) {
	launch := func(h *harness) {
		t.Helper()
		tree, err := h.st.Load()
		if err != nil {
			t.Fatal(err)
		}
		h.m = Launch(tree, h.st, nil)
		h.wide()
	}
	t.Run("empty and restart", func(t *testing.T) {
		h := start(t).wide()
		if h.shown() != "" {
			t.Fatal("New seeded the model")
		}
		launch(h)
		if h.m.tree.Row(h.m.at.root).Text() != "Welcome" || len(h.m.undo) != 1 || !strings.Contains(h.rows(), "Press j and k") {
			t.Fatal("first launch did not open the tutorial in one undo step")
		}
		before, n := h.m.writeRows(h.m.tree.Children(""))
		h = reopen(t, h)
		launch(h)
		if got, count := h.m.writeRows(h.m.tree.Children("")); got != before || count != n || len(h.m.undo) != 1 {
			t.Fatal("restart seeded a second tutorial")
		}
		h.press("u")
		if got := h.shown(); got != "" {
			t.Fatalf("seed undo was not kept across restart: %s", got)
		}
	})
	for _, fixture := range []string{"project", "closed project", "inbox"} {
		t.Run(fixture, func(t *testing.T) {
			h := start(t).wide()
			switch fixture {
			case "project":
				h.pasteText("Existing")
			case "closed project":
				h.pasteText("Existing $done")
			case "inbox":
				h.press("a", `"Captured"`, "enter", "esc")
			}
			before, _ := h.m.writeRows(h.m.tree.Children(""))
			steps := len(h.m.undo)
			launch(h)
			if got, _ := h.m.writeRows(h.m.tree.Children("")); got != before || len(h.m.undo) != steps {
				t.Fatal("launch changed an occupied database")
			}
		})
	}
	t.Run("shell commands", func(t *testing.T) {
		h := start(t)
		List(h.m.tree, h.st, "")
		if got := h.m.tree.Children(""); len(got) != 2 {
			t.Fatalf("List added projects: %v", got)
		}
		if _, err := Capture(h.m.tree, h.st, []string{"Captured"}); err != nil {
			t.Fatal(err)
		}
		if len(h.m.tree.Children("")) != 2 || len(h.m.tree.Children(inboxID)) != 1 {
			t.Fatal("Capture added a tutorial")
		}
	})
}

func TestHelpSections(t *testing.T) {
	want := []string{"Essentials", "Add and type", "States and sigils", "Counts and operators", "Selection", "Filter", "Capture and travel", "Undo and commands"}
	for _, size := range [][2]int{{100, 12}, {100, 24}, {100, 60}, {60, 24}, {30, 12}} {
		width, height := size[0], size[1]
		h := start(t)
		h.m.Update(tea.WindowSizeMsg{Width: width, Height: height})
		h.press(":", `"help"`, "enter")
		first := ansi.Strip(h.m.View().Content)
		for _, key := range []string{"j k", "o O", "x  !  t", "/", "u  Ctrl+R", "Ctrl+C", ":tutor"} {
			if width >= 60 && !strings.Contains(first, key) {
				t.Errorf("%dx%d: first help screen omits %q", width, height, key)
			}
		}
		check := func(heading string) {
			t.Helper()
			lines := strings.Split(ansi.Strip(h.m.View().Content), "\n")
			// The header names the section, its title is the first line
			// under it, and the rail marks it when there is one.
			ok := h.m.help && len(lines) == height &&
				strings.Contains(lines[0], "Help › "+heading) &&
				strings.Contains(lines[1], heading) &&
				strings.Contains(lines[len(lines)-1], "] [ section")
			if width >= sidebarFrom {
				ok = ok && strings.Contains(strings.Join(lines, "\n"), "▌ "+heading)
			}
			if !ok {
				t.Fatalf("%dx%d: expected %q and navigation footer:\n%s", width, height, heading, strings.Join(lines, "\n"))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > width {
					t.Fatalf("%dx%d: line wider than the screen: %q", width, height, line)
				}
			}
		}
		h.press("[")
		check(want[0])
		for _, heading := range want[1:] {
			h.press("]")
			check(heading)
		}
		h.press("]")
		check(want[len(want)-1])
		for i := len(want) - 2; i >= 0; i-- {
			h.press("[")
			check(want[i])
		}
		h.press("j", "[")
		check(want[0])
		h.press("esc")
		if h.m.help {
			t.Fatal("Esc did not close help")
		}
	}
}

// Every line of the reference is keys and what they do, a note, or a
// heading, and no key is left wider than its column without a reason.
func TestHelpSource(t *testing.T) {
	sections := 0
	for _, e := range helpDoc {
		switch {
		case e.kind == helpSection:
			sections++
		case e.kind == helpItem && e.text == "":
			t.Errorf("%q says nothing", e.keys)
		case e.kind == helpItem && strings.Count(e.text, "`")%2 != 0:
			t.Errorf("%q leaves a key open: %s", e.keys, e.text)
		case e.kind == helpItem && ansi.StringWidth(e.keys) > helpKeyCols:
			t.Errorf("%q is wider than the key column", e.keys)
		}
	}
	if sections != 8 {
		t.Fatalf("%d sections", sections)
	}
	for _, line := range strings.Split(helpSource, "\n") {
		if line != "" && !strings.Contains(line, "\t") && !strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, ":: ") && !strings.HasPrefix(line, "== ") {
			t.Errorf("line is neither keys nor a heading: %q", line)
		}
	}
}

func TestFilterPhrase(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tPer the spec: $note\n\t\tBuild it $open\n\tWork PER THE SPEC today $open\n\tThe spec per review $open\n\tCut 5\" board $open\n\tRead a|b ( c ) $open")
	filter := func(query string) {
		h.press("/")
		// Send quotes as keys: the harness's quoted tokens trim edge quotes.
		for _, r := range query {
			h.press(string(r))
		}
	}
	h.press("/", "tab", "esc")
	for query, want := range map[string]string{
		`"per the spec"`:                        "Per the spec:, Work PER THE SPEC today",
		`"PER THE SPEC"`:                        "Per the spec:, Work PER THE SPEC today",
		`$ -"per the spec"`:                     `Build it, The spec per review, Cut 5" board, Read a|b ( c )`,
		`"per the spec`:                         "Per the spec:, Work PER THE SPEC today",
		`^"per the spec"`:                       "Build it",
		`<"per the spec"`:                       "Build it",
		`_"build it"`:                           "Project, Per the spec:",
		`>"build it"`:                           "Per the spec:",
		`"per the spec" $open`:                  "Work PER THE SPEC today",
		`"per the spec"|"build it"`:             "Per the spec:, Build it, Work PER THE SPEC today",
		`"a|b ( c )"`:                           "Read a|b ( c )",
		`5" board`:                              `Cut 5" board`,
		`"per  the spec"`:                       "",
		`( "per the spec" | "build it" ) $open`: "Build it, Work PER THE SPEC today",
	} {
		filter(query)
		if got := h.shown(); got != want {
			t.Errorf("filter %q = %q, want %q", query, got, want)
		}
		h.press("esc")
	}
	// Keep, recall, and find with the same phrase, including after restart.
	filter(`"per the spec"`)
	h.press("enter", ":", `"save phrase"`, "enter", "esc", "g", "g", "n")
	if got := h.m.tree.Row(h.m.current()).Text(); got != "Per the spec:" {
		t.Fatalf("n found %q", got)
	}
	h.press("n")
	if got := h.m.tree.Row(h.m.current()).Text(); got != "Work PER THE SPEC today" {
		t.Fatalf("second n found %q", got)
	}
	h.press("N")
	if got := h.m.tree.Row(h.m.current()).Text(); got != "Per the spec:" {
		t.Fatalf("N found %q", got)
	}
	again := reopen(t, h)
	again.press("f", `"/phrase"`, "enter")
	if got := again.shown(); got != "Per the spec:, Work PER THE SPEC today" {
		t.Fatalf("saved phrase = %q", got)
	}
	if got := strings.Join(List(h.m.tree, h.st, `"per the spec" $open`), "|"); got != "Work PER THE SPEC today $open\tProject" {
		t.Fatalf("List phrase = %q", got)
	}
}

func TestFilterPhraseEmptyPassedOver(t *testing.T) {
	h := work(t)
	filter := func(query string) {
		h.press("/")
		for _, r := range query {
			h.press(string(r))
		}
	}
	for _, flat := range []bool{false, true} {
		filter("$")
		if flat {
			h.press("tab")
		}
		want := h.rows()
		h.press("esc")
		for _, prefix := range []string{"", "-", "^", "-^", "_", "-_", "<", "-<", ">", "->"} {
			for _, phrase := range []string{`"`, `""`, `""|""`} {
				query := "$ " + prefix + phrase
				filter(query)
				if got := h.rows(); got != want {
					t.Errorf("filter %q (flat %t) =\n%s\nwant:\n%s", query, flat, got, want)
				}
				h.press("enter")
				if got := h.rows(); got != want {
					t.Errorf("kept filter %q changed rows: %s", query, got)
				}
				h.press("esc")
				if got, want := strings.Join(List(h.m.tree, h.st, query), "\n"), strings.Join(List(h.m.tree, h.st, "$"), "\n"); got != want {
					t.Errorf("List %q = %q, want %q", query, got, want)
				}
			}
		}
	}
	// An empty alternative must not override a populated phrase. As soon
	// as a negated phrase has text, it narrows the pane, even before closing.
	for query, want := range map[string]string{
		`$ ""|"fix"`: "Fix cache, Fix login",
		`$ "fix"|"`:  "Fix cache, Fix login",
		`$ -"fix`:    "Write docs, Refactor auth",
		`$ -"fix"`:   "Write docs, Refactor auth",
	} {
		filter(query)
		if got := h.shown(); got != want {
			t.Errorf("filter %q = %q, want %q", query, got, want)
		}
		h.press("esc")
	}
}

func TestFilterPhraseNotStyledAsSigils(t *testing.T) {
	for _, tc := range []struct {
		query, styled, rest string
	}{
		{query: `"costs $5 total`},
		{query: `"costs $5 total"`},
		{query: `"call $wa`},
		{query: `"call $wa"`},
		{query: `"call #do`},
		{query: `"call @to`},
		{query: `"call $open #docs @today ! ( ) | @nope"`},
		{query: `-^"call $wa`},
		{query: `_"call $wa"`},
		{query: `<"call $wa`},
		{query: `>"call $wa"`},
		{query: `plain|"call $wa`},
		{query: `"call |$wa`},
		{query: `"call |$wa"`},
		{query: `"call $wa"|"costs $5 total"`},
		{query: `"café  $open   #docs"`},
		{query: `"call $wa" $open`, styled: `"call $wa" ` + darkPalette.accent + "$open" + sgrReset},
		{query: `"call $wa"|$open`, styled: `"call $wa"|` + darkPalette.accent + "$open" + sgrReset},
		{query: `"call $wa" $wa`, styled: `"call $wa" ` + darkPalette.accent + "$wa" + sgrReset, rest: "iting"},
		{query: `"call $wa"|$wa`, styled: `"call $wa"|` + darkPalette.accent + "$wa" + sgrReset, rest: "iting"},
		{query: `"call $wa"|@nope`, styled: `"call $wa"|` + darkPalette.warn + "@nope" + sgrReset},
		{query: `5" $wa`, styled: `5" ` + darkPalette.accent + "$wa" + sgrReset, rest: "iting"},
	} {
		// TempDir uses the test name; keep URI fragments out of SQLite paths.
		t.Run(strings.ReplaceAll(tc.query, "#", "tag"), func(t *testing.T) {
			h := work(t)
			h.press("/")
			for _, r := range tc.query {
				h.press(string(r))
			}
			want := tc.styled
			if want == "" {
				want = tc.query
			}
			if tc.rest != "" {
				want += darkPalette.dim + tc.rest + sgrReset
			}
			lines := strings.Split(h.m.View().Content, "\n")
			if got := strings.ReplaceAll(lines[len(lines)-1], darkPalette.text, ""); !strings.HasPrefix(got, darkPalette.accent+"Filter › "+sgrReset+want+"   ") {
				t.Errorf("filter line = %q; want typed text %q", got, want)
			}
			h.press("right")
			if got := h.m.at.query; got != tc.query+tc.rest {
				t.Errorf("right arrow produced %q; want %q", got, tc.query+tc.rest)
			}
		})
	}
	t.Run("phrase begins before visible window", func(t *testing.T) {
		h := start(t)
		h.press("/")
		for _, r := range `"a very long phrase with a price of $5 and state $open` {
			h.press(string(r))
		}
		line, _ := h.m.statusLine(28)
		text := strings.TrimPrefix(line, darkPalette.accent+"Filter › "+sgrReset)
		if !strings.Contains(text, "$open") || strings.Contains(text, darkPalette.accent) || strings.Contains(text, darkPalette.warn) {
			t.Fatalf("scrolled phrase = %q", text)
		}
	})
}

func TestFilterFirst(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tLater @@9999-12-31 $open\n\tParked $parked\n\t\tHidden $open\n\tWaiting $waiting\n\tFirst #home $doing\n\t\tChild $open\n\t\tSecond child $open\n\tSecond #home $open\n\tHeading: $note\n\t\tFuture @@9999-12-31 $open\n\t\tFinished $done\n\t\tCanceled $canceled\n\t\tHeading first $open\n\t\tHeading second $doing\nOther\n\tOther first $open\nClosed $done\n\tClosed child $open\nCanceled project $canceled\n\tCanceled child $open\nParked project $parked\n\tParked child $open")
	h.press("a", `"Inbox first"`, "enter", `"Inbox second"`, "enter", "esc")
	h.filters(map[string]string{
		"$first":         "First, Child, Heading first, Other first",
		"$FIRST":         "First, Child, Heading first, Other first",
		"$first -<$":     "First, Heading first, Other first",
		"$first #home":   "First",
		"-$first #home":  "Second",
		"$first|waiting": "Waiting, First, Child, Heading first, Other first",
		"$next":          "First, Other first",
	})
	h.press("/", `"$fir"`)
	if !strings.Contains(h.bottom(), "$first") {
		t.Fatalf("completion = %q", h.bottom())
	}
	h.press("right")
	if !strings.Contains(h.m.View().Content, darkPalette.accent+"$first"+sgrReset) {
		t.Fatal("$first is not coloured as a filter word")
	}
	h.press("enter", ":", `"save first"`, "enter", "esc", "g", "g", "n")
	if got := h.m.tree.Row(h.m.current()).Text(); got != "First" {
		t.Fatalf("n found %q", got)
	}
	h.press("n", "N")
	if got := h.m.tree.Row(h.m.current()).Text(); got != "First" {
		t.Fatalf("N found %q", got)
	}
	again := reopen(t, h)
	again.press("f", `"/first"`, "enter")
	if got := again.shown(); got != "First, Child, Heading first, Other first" {
		t.Fatalf("saved first = %q", got)
	}
	want := "Inbox first $open\tInbox|First #home $doing\tProject|Heading first $open\tProject › Heading|Other first $open\tOther"
	if got := strings.Join(List(h.m.tree, h.st, "$first -<$"), "|"); got != want {
		t.Fatalf("List first = %q, want %q", got, want)
	}
	// A held filter keeps its rows; rereading it advances each parent's pick.
	again.press("g", "g", "x")
	if got := again.shown(); got != "First, Child, Heading first, Other first" {
		t.Fatalf("held first = %q", got)
	}
	again.press("/", "backspace", `"t"`, "enter")
	if got := again.shown(); got != "Child, Second, Heading first, Other first" {
		t.Fatalf("reread first = %q", got)
	}
	// Filter words remain text on the row and stamp surfaces.
	if _, bad := parseStamp("$first", h.m.today()); bad != "$first" {
		t.Fatal("$first became a stamp state")
	}
}

// observeCommits makes the absence of a log write observable without
// reaching into SQLite; every stored operation passes through Commit.
type observeCommits struct {
	Backend
	calls int
}

func (b *observeCommits) Commit(ops []outline.Op, now int64) error {
	b.calls++
	return b.Backend.Commit(ops, now)
}

func TestGlOpensAddress(t *testing.T) {
	const address = "https://example.com/first"
	fixture := func(t *testing.T) (*harness, *[]string) {
		t.Helper()
		h := start(t).wide()
		h.pasteText("Project\n\tRead " + address + ".,;:!?) http://example.org/second $open\n\tDestination $open")
		h.press("g", "g", "j")
		opened := []string{}
		h.m.openURL = func(url string) error {
			opened = append(opened, url)
			return nil
		}
		return h, &opened
	}
	for _, place := range []string{"outline", "list", "filter"} {
		t.Run(place, func(t *testing.T) {
			h, opened := fixture(t)
			switch place {
			case "list":
				h.press("t", "f", `"Today"`, "enter")
			case "filter":
				h.press("/", `"Read"`, "tab", "enter")
			}
			// Leave both a remembered change and redo history to preserve.
			h.press("!", "u")
			rows, header := h.rows(), h.header()
			gen, undo, redo := h.m.tree.Gen(), len(h.m.undo), len(h.m.redo)
			older, newer, repeat := slices.Clone(h.m.older), slices.Clone(h.m.newer), h.m.repeat
			backend := &observeCommits{Backend: h.m.backend}
			h.m.backend = backend
			h.press("g", "l")
			if !slices.Equal(*opened, []string{address}) || h.status() != "opened "+address {
				t.Fatalf("opened = %v, status = %q", *opened, h.status())
			}
			if h.rows() != rows || h.header() != header || h.m.tree.Gen() != gen ||
				len(h.m.undo) != undo || len(h.m.redo) != redo || backend.calls != 0 ||
				!slices.Equal(h.m.older, older) || !slices.Equal(h.m.newer, newer) || h.m.repeat != repeat {
				t.Fatal("opening an address changed rows, history, the log, the place or dot repeat")
			}
			h.press(".")
			if len(*opened) != 1 || h.m.tree.Row(h.m.current()).Fields["flag"] != "1" {
				t.Fatal("dot did not retain the earlier flag change")
			}
		})
	}
	t.Run("opener error", func(t *testing.T) {
		h, opened := fixture(t)
		h.m.openURL = func(url string) error {
			*opened = append(*opened, url)
			return errors.New("opener unavailable")
		}
		rows, steps, gen := h.rows(), len(h.m.undo), h.m.tree.Gen()
		h.press("g", "l")
		if !slices.Equal(*opened, []string{address}) || h.status() != "opener unavailable" ||
			h.rows() != rows || len(h.m.undo) != steps || h.m.tree.Gen() != gen {
			t.Fatalf("failed opener changed rows or lost its error: %q", h.status())
		}
	})
	t.Run("row link wins even when gone", func(t *testing.T) {
		h, opened := fixture(t)
		h.press("L", `"Destination"`, "enter")
		rows, steps := h.rows(), len(h.m.undo)
		h.press("g", "l")
		h.want("  ○ Read "+address+".,;:!?) http://example.org/second", "    → Destination", "▌ ○ Destination")
		if len(*opened) != 0 || len(h.m.undo) != steps {
			t.Fatal("following a row link opened an address or made an undo step")
		}
		h.press("ctrl+o")
		if h.rows() != rows {
			t.Fatal("the row link did not leave a jump back")
		}
		h.press("g", "l", "d", "d", "ctrl+o", "g", "l")
		if len(*opened) != 0 || h.status() != "the linked row is gone" {
			t.Fatalf("gone link fell through to the address: %q", h.status())
		}
	})
	t.Run("selection and no address", func(t *testing.T) {
		h, opened := fixture(t)
		h.press("v")
		rows, steps := h.rows(), len(h.m.undo)
		h.press("g", "l")
		if !strings.HasPrefix(h.status(), "that works on one row: press Esc to clear the selection") ||
			h.rows() != rows || len(h.m.undo) != steps || len(*opened) != 0 {
			t.Fatalf("selection was not refused: %q", h.status())
		}
		h.press("esc", "j", "g", "l")
		if h.status() != "this row has no link: L makes one" || len(*opened) != 0 {
			t.Fatalf("row without a link or address = %q", h.status())
		}
	})
}

func TestRepeatReopensRowsUnder(t *testing.T) {
	for _, route := range []struct {
		name string
		keys []string
	}{
		{"x", []string{"x"}},
		{"stamp", []string{"$", `"done"`, "enter"}},
		{"line", []string{"A", `" $done"`, "esc"}},
	} {
		for _, dated := range []bool{false, true} {
			t.Run(route.name+"/dated="+strconv.FormatBool(dated), func(t *testing.T) {
				h := start(t).wide()
				parent := "Weekly *weekly $open"
				beforeParent, afterParent := "▌ ○ Weekly *1w", "▌ ○ Weekly @@Nov 22 *1w"
				if dated {
					parent = "Weekly @today @@today *weekly $doing"
					beforeParent = "▌ ● Weekly @today @@today *1w"
					afterParent = "▌ ○ Weekly @Nov 22 @@Nov 22 *1w"
				}
				h.pasteText(parent + "\n" + strings.Join([]string{
					"\tDirect @tomorrow @@today $done",
					"\tNotes: $note",
					"\t\tNested $done",
					"\t\t\tDeep @fri @@tomorrow $done",
					"\tCanceled $canceled",
					"\t\tUnder canceled $done",
					"\tParked $parked",
					"\tWaiting $waiting",
					"\tDoing $doing",
					"\tOpen $open",
					"\tRepeating @fri @@tomorrow *daily $open",
					"Outside $done",
				}, "\n"))
				h.press("g", "g")
				before := []string{
					beforeParent,
					"    ✓ Direct @tomorrow @@today",
					"    ▾ Notes: 2/2",
					"      ✓ Nested",
					"        ✓ Deep @fri @@tomorrow",
					"    ✕ Canceled",
					"      ✓ Under canceled",
					"    ◌ Parked",
					"    ◔ Waiting",
					"    ● Doing",
					"    ○ Open",
					"    ○ Repeating @fri @@tomorrow *1d",
					"  ✓ Outside",
				}
				h.want(before...)
				after := append([]string(nil), before...)
				after[0] = afterParent
				after[2] = "    ▾ Notes: 0/2"
				for _, i := range []int{1, 3, 4, 6} {
					after[i] = strings.Replace(after[i], "✓", "○", 1)
				}

				// Fields verify clocks and untouched dates that the drawn rows
				// cannot fully show; the interaction itself stays on the keys.
				original := map[string]map[string]string{}
				for _, v := range h.m.vis {
					row := h.m.tree.Row(v.id)
					original[row.ID] = maps.Clone(row.Fields)
				}
				parentID, steps := h.m.current(), len(h.m.undo)
				h.clock = h.clock.Add(time.Hour)
				for _, keys := range [][]string{route.keys, {"."}} {
					h.press(keys...)
					h.want(after...)
					if h.status() != "repeats: next Nov 22" {
						t.Fatalf("status = %q", h.status())
					}
					if len(h.m.undo) != steps+1 {
						t.Fatal("closing the checklist must be one undo step")
					}
					for id, fields := range original {
						want := maps.Clone(fields)
						if id == parentID {
							want["start"] = "2026-11-22"
							if dated {
								want["due"] = "2026-11-22"
								want["state"] = "open"
								want["changed"] = strconv.FormatInt(h.clock.UnixMilli(), 10)
							}
						} else if h.m.tree.Contains(parentID, id) && fields["state"] == "done" {
							want["state"] = "open"
							want["changed"] = strconv.FormatInt(h.clock.UnixMilli(), 10)
						}
						if got := h.m.tree.Row(id).Fields; !maps.Equal(got, want) {
							t.Fatalf("%s fields = %v, want %v", fields["text"], got, want)
						}
					}
					h.press("u")
					h.want(before...)
					for id, want := range original {
						if got := h.m.tree.Row(id).Fields; !maps.Equal(got, want) {
							t.Fatalf("undo %s fields = %v, want %v", want["text"], got, want)
						}
					}
				}
			})
		}
	}
}

func TestHeaderCountsAndSums(t *testing.T) {
	t.Run("held rows count themselves, not subtrees or context", func(t *testing.T) {
		h := start(t).wide()
		h.pasteText("Parent #est=1 $open\n\tChild #est=30m $open")
		h.press("g", "g", "t", "f", `"Today"`, "enter")
		if got := h.header(); got != "Today  · 1 to do · 1h" {
			t.Fatalf("list counted descendants: %q", got)
		}
		h.press("f", `"Projects"`, "enter", "/", `"Child"`, "enter")
		if got := h.header(); got != "Projects  · 1 to do · 30m  / Child" {
			t.Fatalf("filter counted context ancestor: %q", got)
		}
		h.press("esc", "/", `"#est"`, "enter")
		if got := h.header(); got != "Projects  · 2 to do · 1h30m  / #est" {
			t.Fatalf("filter counted descendants twice: %q", got)
		}
		h.press("esc", "g", "g", "enter")
		if got := h.header(); got != "Parent  · 1 to do · 30m" {
			t.Fatalf("outline counted its root: %q", got)
		}
		h.press("#", `"est=2"`, "enter")
		if got := h.header(); got != "Parent  · 1 to do · 2h" {
			t.Fatalf("estimate edit left stale totals: %q", got)
		}
		h.press("u")
		if got := h.header(); got != "Parent  · 1 to do · 30m" {
			t.Fatalf("undo left stale totals: %q", got)
		}
	})
	h := start(t).wide()
	h.pasteText("Project\n\tStorage:\n\t\tBuild #est=1.5h $doing\n\t\tWait #est=30m $waiting\n\t\tReview #est=2d $open\n\t\tBare #est=2 $open\n\t\tUnreadable #est=oops $open\n\t\tDone #est=99h $done\n\t\tCanceled #est=99h $canceled\n\t\tParked #est=99h $parked\n\t\tNote #est=99h $note\n\tOutside $open")
	wantHeader := func(want string) {
		t.Helper()
		if got := h.header(); got != want {
			t.Fatalf("header = %q, want %q", got, want)
		}
	}
	wantHeader("Projects  · 6 to do · 20h")
	h.press("g", "g", "enter", "h")
	wantHeader("Project  · 6 to do · 20h")
	if !strings.Contains(h.rows(), "▸ Storage: 1/6") || strings.Contains(h.rows(), "Build") {
		t.Fatalf("folded outline =\n%s", h.rows())
	}
	h.press("enter")
	wantHeader("Project › Storage:  · 5 to do · 20h")
	h.press("g", "g", "t", "j", "t", "f", `"Today"`, "enter")
	wantHeader("Today  · 2 to do · 2h")
	h.press("g", "g", "x")
	wantHeader("Today  · 1 to do · 30m")
	h.press("j", "x")
	wantHeader("Today") // Held closed rows contribute neither count nor estimate.
	h.press("f", `"Projects"`, "enter", "/", `"#est"`, "enter")
	wantHeader("Projects  · 3 to do · 18h  / #est")
	h.press("/", "tab", "enter")
	wantHeader("Projects  · 3 to do · 18h  / #est  (flat)")
	h.press("esc", "/", `"Unreadable"`, "enter")
	wantHeader("Projects  · 1 to do  / Unreadable  (flat)")
	h.press("x")
	wantHeader("Projects  / Unreadable  (flat)")

	for _, tc := range []struct{ value, sum string }{
		{"30m", "30m"}, {"1.5h", "1h30m"}, {"2d", "16h"}, {"2", "2h"},
		{"0", "0m"}, {"0.01h", "1m"}, {"oops", ""}, {"NaN", ""}, {"Inf", ""}, {"-2h", ""},
	} {
		t.Run(tc.value, func(t *testing.T) {
			h := start(t).wide()
			h.pasteText("Work #est=" + tc.value + " $open\nOther $note")
			want := "Projects  · 1 to do"
			if tc.sum != "" {
				want += " · " + tc.sum
			}
			if got := h.header(); got != want {
				t.Fatalf("header = %q, want %q", got, want)
			}
		})
	}
}

func TestHeadingProgress(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Project\n\tStorage:\n\t\tBuild $open\n\t\tNested:\n\t\t\tDone $done\n\t\t\tWait $waiting\n\t\t\tCanceled $canceled\n\t\t\tParked $parked\n\tEmpty:\n\t\tNote $note")
	h.press("g", "g", "enter")
	wantProgress := func(storage, nested string) {
		t.Helper()
		rows := h.rows()
		for _, want := range []string{"Storage: " + storage, "Nested: " + nested, "Empty:"} {
			if !strings.Contains(rows, want) {
				t.Fatalf("missing %q in\n%s", want, rows)
			}
		}
		if strings.Contains(rows, "Empty: 0/") {
			t.Fatalf("empty heading has progress:\n%s", rows)
		}
	}
	wantProgress("1/3", "1/2")
	if !strings.Contains(h.m.View().Content, darkPalette.dim+"1/3"+sgrReset) {
		t.Fatal("heading progress must be dimmed")
	}
	h.press("j", "x")
	wantProgress("2/3", "1/2")
	h.press("j", "j", "x")
	wantProgress("1/3", "0/2")
	h.press("u")
	wantProgress("2/3", "1/2")
	h.press("g", "g", "h")
	if rows := h.rows(); !strings.Contains(rows, "▸ Storage: 2/3") || strings.Contains(rows, "Nested:") {
		t.Fatalf("folded progress:\n%s", rows)
	}
	h.press("l")
	wantProgress("2/3", "1/2")
}

func TestCursorRowTint(t *testing.T) {
	for _, light := range []bool{false, true} {
		t.Run(strconv.FormatBool(light), func(t *testing.T) {
			h := start(t).wide()
			if light {
				h.m.Update(tea.BackgroundColorMsg{Color: color.White})
			}
			h.pasteText("Project\n\tA long task with styled fields and enough words to wrap across several drawn lines ! #work @today $doing\n\tOther $open")
			h.press("g", "g", "enter")
			h.m.Update(tea.WindowSizeMsg{Width: 80, Height: 16})
			width := 80 - sidebarCols - 3
			check := func(want string) int {
				t.Helper()
				lines, _ := h.m.paneLines(width, 15)
				var background color.Color
				if want != "" {
					background = uv.NewStyledString(want + " ").Lines(ansi.WcWidth)[0][0].Style.Bg
				}
				count := 0
				for _, line := range lines {
					if !strings.Contains(line, "▌") {
						if strings.Contains(line, h.m.colors().cursor) || strings.Contains(line, h.m.colors().selection) {
							t.Fatal("background leaked to another row")
						}
						continue
					}
					count++
					cells := uv.NewStyledString(line).Lines(ansi.WcWidth)[0]
					if want != "" && len(cells) != width {
						t.Fatalf("tinted line width = %d, want %d", len(cells), width)
					}
					for x, cell := range cells {
						if !reflect.DeepEqual(cell.Style.Bg, background) {
							t.Fatalf("line %d column %d background = %v, want %v", count, x, cell.Style.Bg, background)
						}
					}
				}
				return count
			}
			if n := check(h.m.colors().cursor); n < 2 {
				t.Fatalf("expected wrapped cursor row, got %d lines", n)
			}
			h.press("v", "k")
			check(h.m.colors().selection)
			if !strings.Contains(h.rows(), "┃") {
				t.Fatal("selection lost its mark")
			}
			h.press("esc", "tab")
			check("")
			for _, line := range h.m.sideLines(sidebarCols, 15) {
				if strings.Contains(line, "▌") {
					cells := uv.NewStyledString(line).Lines(ansi.WcWidth)[0]
					for _, cell := range cells {
						if cell.Style.Bg == nil {
							t.Fatal("focused sidebar background has a gap")
						}
					}
				}
			}
			h.press("tab", "A")
			check(h.m.colors().cursor) // Canonical fields and text selection reset styles too.
		})
	}
}

func TestFarDeadlineNamesYear(t *testing.T) {
	h := start(t).wide()
	h.pasteText("Future @2027-01-05 @@2027-01-02 $open\nPast @2025-12-31 $open")
	h.press("g", "g")
	for _, want := range []string{"@Jan 5 2027", "@@Jan 2 2027", "@Dec 31 2025"} {
		if !strings.Contains(h.rows(), want) {
			t.Fatalf("missing %q in %s", want, h.rows())
		}
	}
	h.press("A")
	if got := string(h.m.edit.buf); !strings.Contains(got, "@2027-01-05") || !strings.Contains(got, "@@2027-01-02") {
		t.Fatalf("canonical editing date = %q", got)
	}
	if got, _ := sigil.DueLabel("2027-01-01", time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)); got != "@Jan 1 2027" {
		t.Fatalf("year boundary = %q", got)
	}
}

func TestPaletteFollowsBackground(t *testing.T) {
	h := start(t)
	h.pasteText("Work ! @today $doing")
	if h.m.colors() != darkPalette {
		t.Fatal("no terminal answer must use dark")
	}
	requested := false
	for _, cmd := range h.m.Init()().(tea.BatchMsg) {
		if reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.RequestBackgroundColor).Pointer() {
			requested = true
		}
	}
	if !requested {
		t.Fatal("launch did not request the terminal background")
	}
	for _, tc := range []struct {
		background color.Color
		want       palette
	}{{color.White, lightPalette}, {color.Black, darkPalette}, {color.RGBA{245, 240, 230, 255}, lightPalette}} {
		before := ansi.Strip(h.m.View().Content)
		h.m.Update(tea.BackgroundColorMsg{Color: tc.background})
		view := h.m.View().Content
		if h.m.colors() != tc.want || !strings.Contains(view, tc.want.accent+"●") || !strings.Contains(view, tc.want.alert+sgrBold+"!") {
			t.Fatalf("background %v did not paint its palette", tc.background)
		}
		if ansi.Strip(view) != before {
			t.Fatal("palette changed layout")
		}
	}
}
