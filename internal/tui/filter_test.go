package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/sigil"
)

// work is a wide screen over two projects holding four tasks.
func work(t *testing.T) *harness {
	h := start(t).wide()
	h.press("o", `"Alpha"`, "enter", "tab", `"Fix login #bug !"`, "enter", `"Write docs #docs @fri"`, "enter", `"Refactor auth #bug"`, "esc")
	h.press("g", "g", "O", `"Beta"`, "enter", "tab", `"Fix cache #bug"`, "esc")
	h.want(
		"  ▾ Beta",
		"▌   ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	return h
}

func (h *harness) bottom() string {
	lines := h.lines()
	return strings.TrimSpace(lines[len(lines)-1])
}

func TestFilterNarrowsAsItIsTypedAndKeepsContext(t *testing.T) {
	h := work(t)
	h.press("/", `"#bug"`)
	h.want(
		"▌ ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Refactor auth #bug",
	)
	if h.bottom() != "Filter › #bug   3 match · in context · Tab switches" {
		t.Fatalf("prompt = %q", h.bottom())
	}
	h.press("tab")
	h.want(
		"▌ ○ Fix cache #bug · Beta",
		"  ○ Fix login ! #bug · Alpha",
		"  ○ Refactor auth #bug · Alpha",
	)
	h.press("enter")
	if h.header() != "Projects  / #bug  (flat)" {
		t.Fatalf("header = %q", h.header())
	}

	// A row that stops matching stays until the query changes, and the
	// tree cannot be restructured through a filter.
	h.press("x", "-", `"#bug"`, "enter")
	h.want(
		"▌ ✓ Fix cache · Beta",
		"  ○ Fix login ! #bug · Alpha",
		"  ○ Refactor auth #bug · Alpha",
	)
	h.press("o")
	if h.m.msg != "clear the filter first (Esc)" {
		t.Fatalf("msg = %q", h.m.msg)
	}

	// Enter works inside a match and Esc returns to the same filter; a
	// second Esc clears it and keeps the cursor on its row.
	h.press("j", "enter")
	if h.header() != "Alpha › Fix login" {
		t.Fatalf("header = %q", h.header())
	}
	h.press("esc")
	h.want(
		"  ✓ Fix cache · Beta",
		"▌ ○ Fix login ! #bug · Alpha",
		"  ○ Refactor auth #bug · Alpha",
	)
	h.press("esc")
	h.want(
		"  ▾ Beta",
		"    ✓ Fix cache",
		"  ▾ Alpha",
		"▌   ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestFilterGrammar(t *testing.T) {
	h := work(t)
	h.press("x", "/", "tab", "esc") // Fix cache is done; filters are flat from here
	cases := map[string][]string{
		"fix":          {"Fix cache", "Fix login"},
		"FIX -cache":   {"Fix login"},
		"fix !":        {"Fix login"},
		"-! #bug":      {"Fix cache", "Refactor auth"},
		"#bug -$done":  {"Fix login", "Refactor auth"},
		"$done":        {"Fix cache"},
		"$note":        {"Beta", "Alpha"},
		"@":            {"Write docs"},
		"-@ $open":     {"Fix login", "Refactor auth"},
		"@fri":         {"Write docs"},
		"@week":        {"Write docs"},
		"@overdue":     nil,
		"@nonsense":    nil,
		"#docs #bug":   nil,
		"auth -#docs":  {"Refactor auth"},
		"refactor   $": {"Refactor auth"},
	}
	for query, want := range cases {
		h.press("/", `"`+query+`"`)
		var got []string
		for _, v := range h.m.vis {
			got = append(got, h.m.tree.Row(v.id).Text())
		}
		if strings.Join(got, ", ") != strings.Join(want, ", ") {
			t.Errorf("filter %q = %v, want %v", query, got, want)
		}
		h.press("esc")
	}
	h.press("/", `"@overdue"`, "enter")
	h.want("  Nothing matches.")
}

func TestSelectionActsOnEveryRowAndStampsSigils(t *testing.T) {
	h := work(t)
	h.press("j", "j", "v", "v")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		" ┃  ○ Fix login ! #bug",
		" ┃  ○ Write docs @fri #docs",
		"▌   ○ Refactor auth #bug",
	)
	if !strings.HasSuffix(h.bottom(), "2 selected · Esc clears") {
		t.Fatalf("status = %q", h.bottom())
	}

	h.press("#")
	// The bare sigil already offers the most used tag.
	if h.bottom() != "Stamp › #bug   2 rows · -#tag and -@ remove" {
		t.Fatalf("prompt = %q", h.bottom())
	}
	h.press(`"urgent @tom $w"`, "enter", "-", `"#bug -@ -#docs"`, "enter", "!")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		" ┃  ◔ Fix login ! #urgent",
		" ┃  ◔ Write docs ! #urgent",
		"▌   ○ Refactor auth #bug",
	)
	// All flagged now, so ! clears both; x closes both and reopens both.
	h.press("!", "x")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		" ┃  ✓ Fix login #urgent",
		" ┃  ✓ Write docs #urgent",
		"▌   ○ Refactor auth #bug",
	)
	h.press("x")

	// What works on one row is refused, not quietly aimed at the cursor.
	for _, k := range []string{"A", "o", "O", "enter"} {
		h.press(k)
		if !strings.Contains(h.m.msg, "clear the selection") {
			t.Fatalf("%s: msg = %q", k, h.m.msg)
		}
	}

	// A bulk act is one undo step.
	h.press("d", "d")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		"▌   ○ Refactor auth #bug",
	)
	h.press("u", "esc")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		"▌   ○ Fix login #urgent",
		"    ○ Write docs #urgent",
		"    ○ Refactor auth #bug",
	)
}

func TestAStampThatCannotBeReadChangesNothing(t *testing.T) {
	h := work(t)
	h.press("@", `"fri #ok @nonsense"`, "enter")
	if h.m.msg != "cannot read @nonsense" || h.m.ask == nil {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("esc")
	h.want(
		"  ▾ Beta",
		"▌   ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	h.press("$", `"p"`, "enter")
	h.want(
		"  ▾ Beta",
		"▌   ◌ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestSelectedRowsMoveTogetherInOrder(t *testing.T) {
	h := work(t)
	h.press("j", "j", "v", "v", "m", `"beta"`, "enter")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		" ┃  ○ Fix login ! #bug",
		" ┃  ○ Write docs @fri #docs",
		"  ▾ Alpha",
		"▌   ○ Refactor auth #bug",
	)
	h.press("u")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"  ▾ Alpha",
		"▌┃  ○ Fix login ! #bug",
		" ┃  ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestFilteringDropsTheSelection(t *testing.T) {
	h := work(t)
	h.press("j", "j", "v", "v", "/", `"cache"`, "enter", "j", "x")
	h.want(
		"  ▾ Beta",
		"▌   ✓ Fix cache #bug",
	)
	h.press("esc")
	h.want(
		"  ▾ Beta",
		"▌   ✓ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestAListCanBeFiltered(t *testing.T) {
	h := work(t)
	h.press("$", `"w"`, "enter", "j", "j", "$", `"w"`, "enter", "f", `"waiting"`, "enter")
	h.want(
		"▌ ◔ Fix cache #bug · 0d · Beta",
		"  ◔ Fix login ! #bug · 0d · Alpha",
	)
	h.press("/", `"login"`, "enter")
	h.want("▌ ◔ Fix login ! #bug · 0d · Alpha")
	h.press("esc")
	h.want(
		"  ◔ Fix cache #bug · 0d · Beta",
		"▌ ◔ Fix login ! #bug · 0d · Alpha",
	)
}

func TestCommands(t *testing.T) {
	h := work(t)
	out := filepath.Join(t.TempDir(), "out.txt")
	h.press(":", `"export `+out+`"`, "enter")
	if h.m.msg != "exported 7 rows to "+out {
		t.Fatalf("msg = %q", h.m.msg)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "Inbox\nBeta\n\tFix cache #bug\nAlpha\n\tFix login ! #bug\n\tWrite docs @2026-11-20 #docs\n\tRefactor auth #bug\n"
	if string(got) != want {
		t.Fatalf("export:\n%s\nwant:\n%s", got, want)
	}
	h.press(":", `"nonsense"`, "enter")
	if h.m.msg != "commands: q, help, export [file], closed, numbers, all, archive, fold [level], unfold, sort [keys], review, save name, forget name, sync [folder|off]" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press(":", `"q"`, "enter")
	if !h.m.quit {
		t.Fatal(":q did not quit")
	}
}

func TestExportSpellsStatesAgainstParentDefaults(t *testing.T) {
	h := project(t)
	h.press("o", "tab", `"Detail"`, "enter", `"Subtask $open"`, "esc")
	out := filepath.Join(t.TempDir(), "out.txt")
	h.press(":", `"export `+out+`"`, "enter")
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := "Inbox\nProject\n\tFirst\n\tSecond\n\t\tDetail\n\t\tSubtask $open\n"
	if string(got) != want {
		t.Fatalf("export:\n%s\nwant:\n%s", got, want)
	}
}

func TestExportPreservesEmptyNotesAndEscapes(t *testing.T) {
	h, _, _ := copyFixture(t, sigil.Open, []copyRow{
		{-1, sigil.Line{}},
		{0, sigil.Line{Text: "Child", State: sigil.Open}},
		{-1, sigil.Line{Text: `\#tag`}},
		{-1, sigil.Line{Text: `prefix \#tag`}},
		{-1, sigil.Line{Text: "[ ] literal"}},
	})
	path := filepath.Join(t.TempDir(), "outline.txt")
	h.press(":", `"export `+path+`"`, "enter")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "Inbox\nSource: $open\n\t$note\n\t\tChild\n\t" + `\\\#tag` + "\n\t" + `prefix \\#tag` + "\n\t" + `\[ ] literal` + "\nDestination: $open\n"
	if string(got) != want {
		t.Fatalf("export = %q, want %q", got, want)
	}
}

// projectsWithNext is four projects whose rows test what a next action is.
func projectsWithNext(t *testing.T) *harness {
	h := start(t)
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	h.press("o", `"Alpha"`, "enter", "tab",
		`"Later thing @@+3d"`, "enter",
		`"Parked thing $parked"`, "enter", "tab", `"Hidden child $open"`, "enter", "shift+tab",
		`"Chase Bob $waiting"`, "enter",
		`"Fix login $doing #home"`, "enter",
		`"Write docs #home"`, "enter", "shift+tab",
		`"Beta"`, "enter", "tab", `"Spec:"`, "enter", "tab", `"Draft spec"`, "enter", "shift+tab", `"Other"`, "enter", "shift+tab",
		`"Gamma $parked"`, "enter", "tab", `"Ship it $open"`, "enter", "shift+tab",
		`"Delta"`, "enter", "tab", `"Old thing $done"`, "esc",
		"a", `"Inbox task"`, "enter", "esc")
	h.want(
		"  ▾ Alpha",
		"    ○ Later thing @@wed",
		"    ◌ Parked thing",
		"      ○ Hidden child",
		"    ◔ Chase Bob",
		"    ● Fix login #home",
		"    ○ Write docs #home",
		"  ▾ Beta",
		"    ▾ Spec:",
		"      ○ Draft spec",
		"    ○ Other",
		"  ◌ Gamma",
		"    ○ Ship it",
		"  ▾ Delta",
		"▌   ✓ Old thing",
	)
	return h
}

func TestNextIsTheFirstWorkableRowOfEachProject(t *testing.T) {
	h := projectsWithNext(t)
	h.press("/", `"$next"`)
	h.want(
		"▌ ▾ Alpha",
		"    ● Fix login #home",
		"  ▾ Beta",
		"    ▾ Spec:",
		"      ○ Draft spec",
	)
	h.press("tab")
	h.want(
		"▌ ● Fix login #home · Alpha",
		"  ○ Draft spec · Beta",
	)
	h.press("esc")

	cases := map[string][]string{
		"$next":         {"Fix login", "Draft spec"},
		"$next #home":   {"Fix login"},
		"-$next #home":  {"Write docs"},
		"$next|waiting": {"Chase Bob", "Fix login", "Draft spec"},
		"$n -$parked":   {"Alpha", "Beta", "Spec:", "Delta"}, // $n is still $note
		"$ne":           nil,                                 // a prefix of next is not next
	}
	for query, want := range cases {
		h.press("/", `"`+query+`"`) // flat since the Tab above
		var got []string
		for _, v := range h.m.vis {
			got = append(got, h.m.tree.Row(v.id).Text())
		}
		if strings.Join(got, ", ") != strings.Join(want, ", ") {
			t.Errorf("filter %q = %v, want %v", query, got, want)
		}
		h.press("esc")
	}
}

func TestClosingTheNextActionKeepsItUntilTheQueryIsRead(t *testing.T) {
	h := projectsWithNext(t)
	h.press("/", `"$next"`, "tab", "enter", "x")
	h.want(
		"▌ ✓ Fix login #home · Alpha",
		"  ○ Draft spec · Beta",
	)
	h.press("/", "backspace", `"t"`, "enter")
	h.want(
		"▌ ○ Write docs #home · Alpha",
		"  ○ Draft spec · Beta",
	)
}

func TestNextIsOfferedOnTheFilterLineOnly(t *testing.T) {
	h := projectsWithNext(t)
	h.press("/", `"$ne"`)
	if !strings.Contains(h.bottom(), "$next") {
		t.Fatalf("filter = %q", h.bottom())
	}
	h.press("esc", "o", `"$ne"`)
	if strings.Contains(strings.Join(h.lines(), "\n"), "$next") {
		t.Fatalf("a row offers $next")
	}
}

// deadlines is two projects whose rows are due on either side of Friday.
// The clock reads Sunday 2026-11-15, so fri is 2026-11-20.
func deadlines(t *testing.T) *harness {
	h := start(t)
	h.m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	h.press("o", `"Alpha"`, "enter", "tab",
		`"Late @12-31"`, "enter",
		`"Soon @fri"`, "enter",
		`"None"`, "enter",
		`"Now @today"`, "enter",
		`"Shut @tom $done"`, "enter",
		`"Also fri @11-20"`, "enter", "shift+tab",
		`"Beta"`, "enter", "tab",
		`"Past @2026-11-01"`, "enter",
		`"Week @+1w"`, "esc")
	return h
}

func (h *harness) shown() string {
	var got []string
	for _, v := range h.m.vis {
		got = append(got, h.m.tree.Row(v.id).Text())
	}
	return strings.Join(got, ", ")
}

func TestDeadlineRangesHoldForOpenRowsSoonestFirst(t *testing.T) {
	h := deadlines(t)
	h.press("/", "tab", "esc") // flat from here
	byFri := "Past, Now, Soon, Also fri"
	cases := map[string]string{
		"@<fri":           byFri,
		"@<11-20":         byFri,
		"@<2026-11-20":    byFri,
		"@<+5d":           byFri,
		"@<+0w":           "Past, Now", // the boundary day is included
		"@<today":         "Past, Now",
		"@<tom":           "Past, Now", // a closed row is left out
		"@>fri":           "Soon, Also fri, Week, Late",
		"@>+1w":           "Week, Late",
		"@>TOM":           "Soon, Also fri, Week, Late",
		"@<fri|overdue":   byFri,
		"-@<fri":          "Alpha, Late, None, Shut, Beta, Week", // no deadline included, outline order
		"@<fri -@overdue": "Now, Soon, Also fri",
		"@fri":            "Soon, Also fri",
		"@ -$done":        "Past, Now, Soon, Also fri, Week, Late",
		"$open -@<fri":    "Late, None, Week", // negated: outline order
		"@<firday":        "",
		"@<":              "",
	}
	for query, want := range cases {
		h.press("/", `"`+query+`"`)
		if got := h.shown(); got != want {
			t.Errorf("filter %q = %q, want %q", query, got, want)
		}
		h.press("esc")
	}

	// In context the order is the outline's, ancestors kept.
	h.press("/", `"@<fri"`, "tab")
	if got := h.shown(); got != "Alpha, Soon, Now, Also fri, Beta, Past" {
		t.Errorf("in context = %q", got)
	}
	h.press("esc")

	// Nothing in the outline moves.
	h.want(
		"▌ ▾ Alpha",
		"    ○ Late @Dec 31",
		"    ○ Soon @fri",
		"    ○ None",
		"    ○ Now @today",
		"    ✓ Shut @tomorrow",
		"    ○ Also fri @fri",
		"  ▾ Beta",
		"    ○ Past @Nov 1",
		"    ○ Week @Nov 22",
	)
}

func TestAListFilterKeepsTheListOrder(t *testing.T) {
	h := deadlines(t)
	h.press("k", "t", "g", "g", "j", "j", "t", "f", `"Today"`, "enter")
	if got := h.shown(); got != "Past, Soon" {
		t.Fatalf("today = %q", got)
	}
	h.press("/", `"@<fri"`)
	if got := h.shown(); got != "Past, Soon" {
		t.Errorf("filtered today = %q", got)
	}
}

func TestASavedDeadlineFilterReopensSorted(t *testing.T) {
	h := deadlines(t)
	h.press("/", `"@>fri"`, "tab", "enter", ":", `"save ahead"`, "enter", "esc")
	again := reopen(t, h)
	again.press("f", `"/ahead"`, "enter")
	if got := again.shown(); got != "Soon, Also fri, Week, Late" {
		t.Errorf("reopened = %q", got)
	}
}

func TestDeadlineRangesAreColouredOnTheFilterLine(t *testing.T) {
	h := deadlines(t)
	h.press("/", `"@<+30d @<firday"`)
	raw := h.m.View().Content
	if !strings.Contains(raw, sgrAccent+"@<+30d"+sgrReset) {
		t.Errorf("@<+30d is not shown as a sigil:\n%q", raw)
	}
	if !strings.Contains(raw, sgrWarn+"@<firday"+sgrReset) {
		t.Errorf("@<firday is not flagged:\n%q", raw)
	}
}

// selected names the selected rows in the order the pane shows them.
func (h *harness) selected() string {
	var got []string
	for _, v := range h.m.vis {
		if h.m.picked[v.id] {
			got = append(got, h.m.tree.Row(v.id).Text())
		}
	}
	if len(got) != len(h.m.picked) {
		h.t.Fatalf("%d selected, %d of them shown", len(h.m.picked), len(got))
	}
	return strings.Join(got, ", ")
}

func TestAllSelectsEveryRowThePaneShows(t *testing.T) {
	t.Run("outline leaves folded rows out", func(t *testing.T) {
		h := work(t)
		h.press("g", "g", "h")
		h.command("all")
		if got := h.selected(); got != "Beta, Alpha, Fix login, Write docs, Refactor auth" {
			t.Fatalf("selected = %q", got)
		}
		if !strings.HasSuffix(h.bottom(), "5 selected · Esc clears") {
			t.Fatalf("status = %q", h.bottom())
		}
		h.press("esc")
		if len(h.m.picked) != 0 {
			t.Fatal("Esc left the selection")
		}
	})

	t.Run("filter in context leaves ancestors out", func(t *testing.T) {
		h := work(t)
		h.press("/", `"#bug"`, "enter")
		h.command("all")
		if got := h.selected(); got != "Fix cache, Fix login, Refactor auth" {
			t.Fatalf("selected = %q", got)
		}
		// Retagging is the selection and one stamp, in one undo step.
		steps := len(h.m.undo)
		h.press("-", `"#bug #auth"`, "enter")
		if len(h.m.undo) != steps+1 {
			t.Fatalf("%d undo steps for one stamp", len(h.m.undo)-steps)
		}
		h.press("esc", "esc")
		h.want(
			"▌ ▾ Beta",
			"    ○ Fix cache #auth",
			"  ▾ Alpha",
			"    ○ Fix login ! #auth",
			"    ○ Write docs @fri #docs",
			"    ○ Refactor auth #auth",
		)
		h.press("u")
		h.want(
			"  ▾ Beta",
			"▌   ○ Fix cache #bug",
			"  ▾ Alpha",
			"    ○ Fix login ! #bug",
			"    ○ Write docs @fri #docs",
			"    ○ Refactor auth #bug",
		)
	})

	t.Run("flat filter replaces an earlier selection", func(t *testing.T) {
		h := work(t)
		h.press("j", "j", "j", "v") // Write docs
		h.press("/", `"#bug"`, "tab", "enter")
		h.command("all")
		if got := h.selected(); got != "Fix cache, Fix login, Refactor auth" {
			t.Fatalf("selected = %q", got)
		}
		steps := len(h.m.undo)
		h.press("x")
		if len(h.m.undo) != steps+1 {
			t.Fatalf("%d undo steps for x", len(h.m.undo)-steps)
		}
		h.want(
			"▌┃✓ Fix cache #bug · Beta",
			" ┃✓ Fix login ! #bug · Alpha",
			" ┃✓ Refactor auth #bug · Alpha",
		)
	})

	t.Run("list", func(t *testing.T) {
		h := work(t)
		h.press("t", "j", "j", "t", "f", `"Today"`, "enter")
		h.command("all")
		if got := h.selected(); got != "Fix cache, Fix login" {
			t.Fatalf("selected = %q", got)
		}
	})

	t.Run("empty pane", func(t *testing.T) {
		h := work(t)
		h.press("/", `"nothing"`, "enter")
		h.command("all")
		if h.m.msg != "nothing to select" || len(h.m.picked) != 0 {
			t.Fatalf("msg = %q, %d selected", h.m.msg, len(h.m.picked))
		}
	})
}
