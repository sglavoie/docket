package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (h *harness) wantStatus(want string) {
	h.t.Helper()
	if got := h.status(); got != want {
		h.t.Fatalf("status: %q, want %q", got, want)
	}
}

func TestUndoAndRedoSayWhatTheyReversed(t *testing.T) {
	h := project(t)
	h.press("x")
	h.press("u")
	h.wantStatus("undo: state changed on 1 row")
	h.press("ctrl+r")
	h.wantStatus("redo: state changed on 1 row")

	h.press("k", "d", "d")
	h.press("u")
	h.wantStatus("undo: 1 row back")
	h.press("ctrl+r")
	h.wantStatus("redo: 1 row gone")
	h.press("u")

	h.press("J")
	h.press("u")
	h.wantStatus("undo: 1 row moved")

	h.press("#", `"home"`, "enter", "!")
	h.press("2", "u")
	h.wantStatus("undo: 2 steps")
	h.press("u", "u", "u", "u", "u", "u", "u", "u")
	h.wantStatus("nothing to undo")
}

// marked lists the words of the pane drawn as a filter's find.
func (h *harness) marked() []string {
	h.t.Helper()
	var out []string
	for _, part := range strings.Split(h.m.View().Content, sgrUnderline)[1:] {
		end := strings.Index(part, sgrReset)
		out = append(out, ansi.Strip(part[:end]))
	}
	return out
}

func TestAFilterDrawsWhatItFoundOnItsMatches(t *testing.T) {
	h := start(t)
	h.press("o", `"Plan"`, "enter", "tab", `"Ask about the grant budget #money"`, "enter",
		`"Grant writing workshop"`, "enter", `"Pay rent -- per the spec"`, "esc")
	h.press("/", `"grant"`)
	if got := h.marked(); !slices.Equal(got, []string{"grant", "Grant"}) {
		t.Fatalf("marked %q", got)
	}
	// A phrase marks every word it covers, and a tag the tag it names.
	h.press("esc", "/", `"`, `"per the"`, `"`, `" spec | #money"`)
	if got := h.marked(); !slices.Equal(got, []string{"#money", "per", "the", "spec"}) {
		t.Fatalf("marked %q", got)
	}
	// What a term excludes, or finds on other rows, is not drawn.
	h.press("esc", "/", `"-grant ^plan rent"`)
	if got := h.marked(); !slices.Equal(got, []string{"rent"}) {
		t.Fatalf("marked %q", got)
	}
	h.press("esc", "/", `"budget -( rent | workshop )"`)
	if got := h.marked(); !slices.Equal(got, []string{"budget"}) {
		t.Fatalf("marked %q", got)
	}
	// Without a filter nothing is marked.
	h.press("esc", "esc")
	if got := h.marked(); len(got) != 0 {
		t.Fatalf("marked %q", got)
	}
}

func TestWaitingKeysSayWhatMayFollow(t *testing.T) {
	h := project(t).wide()
	h.m.Update(tea.WindowSizeMsg{Width: 200, Height: 16})
	for _, c := range []struct{ keys, want string }{
		{"g", "gg top · gf project · gl follow link · gn gN into the match · gp gP put the copy · g; g, last changes"},
		{"z", "zc zo shut, open the branch · zm zr shut, open a level · zM zR shut, open all · zj zk next, previous parent · zt zz zb draw at top, middle, bottom"},
		{"dz", "dzj dzk take through the next, previous row with rows under it"},
		{"d", "dd the row · dj } ) H ] n G… through a motion · dic dis objects"},
		{"2y", "yy the row · yj } ) H ] n G… through a motion · yic yis objects"},
		{"di", "dic the rows under it · dis the row and its siblings"},
		{">g", ">gg takes through the first row"},
		{"c", "cc retypes the text, keeping its fields"},
	} {
		h.press(strings.Split(c.keys, "")...)
		if got := h.status(); !strings.HasPrefix(got, c.want) {
			t.Fatalf("after %s: %q, want %q", c.keys, got, c.want)
		}
		h.press("esc")
		if strings.Contains(h.status(), c.want) {
			t.Fatalf("after %s esc: %q", c.keys, h.status())
		}
	}
}

func TestHeadingsAndProjectsCarryWeight(t *testing.T) {
	h := start(t)
	h.press("o", `"Project"`, "enter", "tab", `"Storage:"`, "enter", "tab", `"Task"`, "esc")
	view := h.m.View().Content
	for _, want := range []string{
		darkPalette.accent + sgrBold + "Storage:",
		sgrBold + "Project",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("no %q in %q", want, view)
		}
	}
	// Closed, a heading is dimmed like any closed row.
	h.press("k", "$", `"done"`, "enter")
	if strings.Contains(h.m.View().Content, darkPalette.accent+sgrBold+"Storage:") {
		t.Fatal("a closed heading keeps its weight")
	}
}

func TestTypingATagNamesTheTagsInUse(t *testing.T) {
	h := start(t)
	h.m.Update(tea.WindowSizeMsg{Width: 120, Height: 16})
	h.press("o", `"Plan #home #est=2h"`, "enter", `"Shop #home #est=1h"`, "enter", `"Call #work"`, "esc")
	h.press("/", `"#"`)
	if got := h.status(); !strings.HasSuffix(got, "#home 2 · #est 2 · #work 1") {
		t.Fatalf("filter: %q", got)
	}
	h.press(`"h"`)
	if got := h.status(); !strings.HasSuffix(got, "#home 2") {
		t.Fatalf("filter: %q", got)
	}
	// Typed in full, the one tag it names offers nothing more: its count
	// would stand beside the count of matches as if one contradicted the
	// other, so the line is read back in words.
	h.press(`"ome"`)
	if got := h.status(); strings.Contains(got, "#home 2") || !strings.HasSuffix(got, "   2 matches · in context · "+readBack("#home", h.m.today())) {
		t.Fatalf("filter: %q", got)
	}
	// The stamp line lists the values over the rows, each with its count,
	// so the line itself does not repeat them.
	h.press("esc", "esc", "#")
	h.press(`"w"`)
	if got := h.status(); strings.Contains(got, "#work 1") || !strings.Contains(h.above(), "work  1") {
		t.Fatalf("stamp: %q\n%s", got, h.above())
	}
	// The stamp line has no count of matches, so a tag typed in full
	// still says how many rows carry it already.
	h.press(`"ork"`)
	if got := h.status(); !strings.HasSuffix(got, "#work 1") {
		t.Fatalf("stamp: %q", got)
	}
	h.press("backspace", "backspace", "backspace")
	h.press(`"x"`)
	if got := h.status(); !strings.HasSuffix(got, "a new tag") {
		t.Fatalf("stamp: %q", got)
	}
	h.press("esc", "A", `" #e"`)
	if got := h.status(); got != "Enter next row · Esc done · Tab indent · → completes" || !strings.Contains(h.above(), "est=1h  1") || !strings.Contains(h.above(), "est=2h  1") {
		t.Fatalf("typing: %q\n%s", got, h.above())
	}
}

func TestANewOutlineComesWithFiltersWorthKeeping(t *testing.T) {
	h := start(t)
	tree, err := h.st.Load()
	if err != nil {
		t.Fatal(err)
	}
	h.m = Launch(tree, h.st, nil)
	h.wide()
	var names []string
	for _, f := range h.m.filters {
		names = append(names, f.Name+"="+f.Query)
	}
	if !slices.Equal(names, []string{"next=$next", "week=@week", "done=$done ~7d"}) || len(h.m.undo) != 1 {
		t.Fatalf("filters %q, %d undo steps", names, len(h.m.undo))
	}
	h.press("f", `"/next"`, "enter")
	if h.m.at.query != "$next" || !h.m.at.flat {
		t.Fatalf("opened %q", h.m.at.query)
	}
	// They are made with the tutorial and go with it.
	h.press("esc", "u")
	if len(h.m.filters) != 0 {
		t.Fatalf("undo kept %d filters", len(h.m.filters))
	}
	// An outline that has saved a filter already is given none.
	h.m.filters = []savedFilter{{Name: "mine"}}
	h.m.undo = append(h.m.undo, nil)
	h.m.starterFilters()
	if len(h.m.filters) != 1 {
		t.Fatal("starters added beside a saved filter")
	}
}

func TestAVimKeyTheOutlineDoesNotBindSaysWhatDoes(t *testing.T) {
	h := project(t).wide()
	before := h.rows()
	for k, want := range strayKeys {
		h.press(k)
		if h.status() != want || h.rows() != before {
			t.Fatalf("%s: %q, rows\n%s", k, h.status(), h.rows())
		}
	}
	h.press("M")
	if h.status() != "" {
		t.Fatalf("M: %q", h.status())
	}
}

func TestGSemicolonWalksBackThroughChanges(t *testing.T) {
	h := project(t).wide()
	h.press("k", "x", "j", "!")
	// The change the cursor is on is passed over.
	h.press("g", ";")
	if h.on() != "First" || h.status() != "change 4 of 5" {
		t.Fatalf("g; on %q, %q", h.on(), h.status())
	}
	h.press("2", "g", ";")
	if h.on() != "First" || h.status() != "change 2 of 5" {
		t.Fatalf("2g; on %q, %q", h.on(), h.status())
	}
	h.press("g", ",")
	if h.on() != "Second" || h.status() != "change 3 of 5" {
		t.Fatalf("g, on %q, %q", h.on(), h.status())
	}
	h.press("9", "g", ",")
	if h.on() != "Second" || h.status() != "change 5 of 5" {
		t.Fatalf("9g, on %q, %q", h.on(), h.status())
	}
	h.press("g", ",")
	if h.status() != "at the newest change" {
		t.Fatalf("g, at the end: %q", h.status())
	}

	// A change in another place opens it there, as a jump.
	h.press("f", `"Projects"`, "enter", "gg", "O", `"Other"`, "enter", "tab", `"Elsewhere"`, "esc")
	h.press("G", "H", "enter")
	if h.header() != "Project  · 1 to do" {
		t.Fatalf("zoomed into %q", h.header())
	}
	h.press("g", ";")
	if h.on() != "Elsewhere" {
		t.Fatalf("g; across places on %q", h.on())
	}
	h.press("ctrl+o")
	if h.header() != "Project  · 1 to do" {
		t.Fatalf("Ctrl+O came back to %q", h.header())
	}

	// A deleted row stands for its nearest live ancestor.
	h.press("g", ";")
	if h.on() != "Other" {
		t.Fatalf("g; again on %q", h.on())
	}
	h.press("j", "d", "d", "ctrl+o", "g", ";")
	if h.on() != "Other" {
		t.Fatalf("g; to a deleted row on %q", h.on())
	}
}

func TestGSemicolonWithNothingChanged(t *testing.T) {
	h := start(t)
	h.press("g", ";")
	h.wantStatus("no change made yet this session")
}

func TestARefusalNamesTheWayThatWorks(t *testing.T) {
	h := project(t).wide()
	h.press("J")
	h.wantStatus("already the last of its siblings: >> or m moves it elsewhere")
	h.press("k", "K")
	h.wantStatus("already the first of its siblings: << or m moves it elsewhere")
	h.press(":", `"fold x"`, "enter")
	h.wantStatus("invalid fold level: x: fold 2 shows two levels")
	h.press(":", `"sort ?"`, "enter")
	h.wantStatus("unknown sort key: ?: the keys are @ ! $, or none for text")
	h.press("y", "y")
	h.wantStatus("copied 1 row")
}

// Enter on a name that only one command starts runs that command; a name
// several start, or one given with words after it, is still refused.
func TestCommandEnterTakesTheOneLeft(t *testing.T) {
	h := start(t)
	h.command("hel")
	if !h.m.help {
		t.Fatal(":hel did not open help")
	}
	h = start(t)
	h.command("num")
	if !h.m.numbers {
		t.Fatal(":num did not show row numbers")
	}
	h = start(t)
	h.command("s")
	if !strings.HasPrefix(h.m.msg, "no command s: ") {
		t.Fatalf(":s msg = %q", h.m.msg)
	}
	h = start(t)
	h.command("hel x")
	if h.m.help || !strings.HasPrefix(h.m.msg, "no command hel: ") {
		t.Fatalf(":hel x help=%v msg = %q", h.m.help, h.m.msg)
	}
}

func TestTabCompletesACommandName(t *testing.T) {
	h := start(t)
	typed := func() string { return string(h.m.ask.ed.buf) }
	step := func(key, want string) {
		t.Helper()
		h.press(key)
		if typed() != want {
			t.Fatalf("%s gave %q, want %q", key, typed(), want)
		}
	}

	// Each key writes the next command the letters could become, and past
	// the last the line as it was typed.
	h.press(":", `"s"`)
	step("tab", "s/")
	h.wantList([]string{"s/old/new", "sort [keys]", "save name", "sync [folder|off]"}, "s/old/new")
	if got, want := h.bottom(), ": s/   s/old/new: change text in the ro…"; got != want {
		t.Fatalf("walking shows %q, want %q", got, want)
	}
	step("down", "sort ")
	h.wantList([]string{"s/old/new", "sort [keys]", "save name", "sync [folder|off]"}, "sort [keys]")
	step("ctrl+n", "save ")
	step("ctrl+j", "sync ")
	step("tab", "s")
	step("shift+tab", "sync ")
	step("up", "save ")
	step("ctrl+p", "sort ")
	step("ctrl+k", "s/")
	step("ctrl+k", "s")
	// Back at the typed line the list stays, with nothing written from it.
	h.wantList([]string{"s/old/new", "sort [keys]", "save name", "sync [folder|off]"}, "")

	// Typing starts the walk again from the new line.
	h.press("tab", "tab", `"@"`)
	if typed() != "sort @" || h.m.ask.choice != 0 {
		t.Fatalf("typed %q, choice %d", typed(), h.m.ask.choice)
	}
	h.wantList(nil, "")
	h.press("esc")

	// A name one command answers is finished at once, and runs.
	h.press(":", `"hel"`)
	step("tab", "help")
	h.press("esc", ":", `"arc"`)
	step("down", "archive")
	h.press("esc", ":", `"so"`)
	step("tab", "sort ")
	h.press("esc", ":", `"num"`, "tab", "enter")
	if !h.m.numbers {
		t.Fatal("the completed command did not run")
	}

	// A blank line walks every command; one that starts none stays put.
	h.press(":")
	step("tab", "q")
	step("up", "")
	step("up", "sync ")
	h.press("esc", ":", `"zz"`)
	step("tab", "zz")
	h.press("esc")
}

// A prompt's list keeps to the pane: the line's caret stands under the
// sidebar, and a list hung there would hide the names it is read beside.
func TestAPromptsListLeavesTheSidebarWhole(t *testing.T) {
	h := work(t).wide()
	before := h.side()
	h.press("#")
	if !strings.Contains(h.above(), "bug   3") {
		t.Fatalf("no list over the rows:\n%s", h.above())
	}
	if got := h.side(); got != before {
		t.Fatalf("the list covers the sidebar:\n%s\n\nwas:\n%s", got, before)
	}
	h.press("esc", ":")
	if got := h.side(); got != before {
		t.Fatalf("the command list covers the sidebar:\n%s\n\nwas:\n%s", got, before)
	}
}
