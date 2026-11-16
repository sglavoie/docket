package tui

import (
	"strings"
	"testing"
)

// filters reads each query flat against the pane and compares the rows.
func (h *harness) filters(cases map[string]string) {
	h.t.Helper()
	h.press("/", "tab", "esc") // flat from here
	for query, want := range cases {
		h.press("/", `"`+query+`"`)
		if got := h.shown(); got != want {
			h.t.Errorf("filter %q = %q, want %q", query, got, want)
		}
		h.press("esc")
	}
}

func TestFilterGroupsWithBarsAndBrackets(t *testing.T) {
	h := work(t)
	h.filters(map[string]string{
		// A bar standing alone splits the terms around it into alternatives.
		"#docs | !":          "Fix login, Write docs",
		"#bug cache | #docs": "Fix cache, Write docs",
		"fix | docs | auth":  "Fix cache, Fix login, Write docs, Refactor auth",
		// Brackets standing alone set a part off as one term.
		"fix ( #docs | cache )":      "Fix cache",
		"#bug -( cache | ! )":        "Refactor auth",
		"( #bug ! ) | ( docs @fri )": "Write docs, Fix login", // a deadline term: soonest first
		"( ( fix ) -( login ) )":     "Fix cache",
		"-( #bug ) $":                "Write docs",
		// A bar inside a word still offers alternatives for that word.
		"#docs|bug -( fix )": "Write docs, Refactor auth",
		// A part left open or empty while typing is passed over.
		"#docs |":   "Write docs",
		"| #docs":   "Write docs",
		"( #docs":   "Write docs",
		"#docs )":   "Write docs",
		"#docs ( )": "Write docs",
		"( )":       "",
	})
	// The marks are coloured as part of the grammar.
	h.press("/", `"( #bug | ! )"`)
	if !strings.Contains(h.m.View().Content, sgrAccent+"("+sgrReset) || !strings.Contains(h.m.View().Content, sgrAccent+"|"+sgrReset) {
		t.Fatal("a bracket or a bar standing alone is not drawn as grammar")
	}
}

func TestFilterReadsTheParentAndTheChildren(t *testing.T) {
	h := branches(t)
	h.filters(map[string]string{
		// ^ is any ancestor and < the parent only.
		"^alpha": "a1, a2, deep, a3",
		"<alpha": "a1, a2, a3",
		"<a2":    "deep",
		// _ is any row beneath and > the rows directly under.
		"_deep":                  "Alpha:, a2",
		">deep":                  "a2",
		">a1|b1":                 "Alpha:, Beta:",
		"-<alpha|beta":           "Alpha:, deep, Beta:, Gamma, Delta milk:",
		"->a a":                  "a1, a2, a3, Beta:, Gamma, Delta milk:", // holding an a, with no child that does
		"<alpha -a2":             "a1, a3",
		"< a1":                   "a1", // a bare mark is still being typed
		"<<a":                    "",   // doubled, it is text
		"( <alpha | <beta ) -a1": "a2, a3, b1 milk",
	})
	// The parent may sit above the pane's root.
	h.press("g", "g", "enter")
	h.filters(map[string]string{"<alpha": "a1, a2, a3", "<project": ""})
}

func TestFilterReadsToday(t *testing.T) {
	h := work(t)
	h.press("t")
	h.filters(map[string]string{
		"$today":         "Fix cache",
		"-$today #bug":   "Fix login, Refactor auth",
		"$today|next":    "Fix cache, Fix login",
		"_$today":        "Beta",
		"$today | #docs": "Fix cache, Write docs",
	})
	h.press("/", `"$to"`)
	if !strings.Contains(h.bottom(), "$today") {
		t.Fatalf("$today is not offered: %q", h.bottom())
	}
}

func TestSelectingInsideARow(t *testing.T) {
	t.Run("motions extend it and an operator takes it", func(t *testing.T) {
		h := textRow(t, "one two three")
		h.press("w", "v")
		if !strings.HasPrefix(h.status(), "selecting") {
			t.Fatalf("status = %q", h.status())
		}
		h.press("e")
		if !strings.Contains(h.m.View().Content, sgrReverse+"one"+sgrReset) {
			t.Fatal("the selection is not drawn")
		}
		h.press("d")
		h.wantTyping(" two three", 0)
		if h.m.textRegister != "one" || h.m.edit.sel {
			t.Fatalf("register %q, still selecting %v", h.m.textRegister, h.m.edit.sel)
		}
		// The dot key takes as many characters again from the caret.
		h.press("w", ".")
		h.wantTyping("  three", 1)
		h.press("u", "u")
		h.wantTyping("one two three", 0)
	})

	t.Run("backwards, text objects, change, case and copy", func(t *testing.T) {
		h := textRow(t, "one two three")
		h.press("b", "v", "b", "b", "y")
		h.wantTyping("one two three", 0)
		if h.m.textRegister != "one two t" {
			t.Fatalf("register = %q", h.m.textRegister)
		}
		h.press("w", "v", "i", "w", "c", `"2"`, "esc")
		h.wantTyping("one 2 three", 4)
		h.press("w", "v", "$", "~")
		h.wantTyping("one 2 THREE", 10)
		h.press("0", "v", "2", "l", "x")
		h.wantTyping(" 2 THREE", 0)
		h.press("esc")
		h.want("▌ ○ 2 THREE")
	})

	t.Run("Esc and v end it without leaving the row", func(t *testing.T) {
		h := textRow(t, "one two")
		h.press("w", "v", "w", "esc")
		h.wantTyping("one two", 4)
		if h.m.edit.sel {
			t.Fatal("Esc left the selection on")
		}
		h.press("v", "v", "v", "p", "u", "D")
		if !h.m.edit.sel {
			t.Fatal("a key that is no part of selecting ended it")
		}
		h.wantTyping("one two", 4)
		h.press("esc", "esc")
		if h.m.edit != nil {
			t.Fatal("the row was not left")
		}
	})
}

func TestStarFiltersByTheWordUnderTheCaret(t *testing.T) {
	h := work(t)
	h.press("b", "*")
	if h.m.edit != nil || h.header() != "Projects  · 3 to do  / #bug" {
		t.Fatalf("header = %q", h.header())
	}
	h.want(
		"  ▾ Beta",
		"▌   ○ Fix cache #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Refactor auth #bug",
	)
	// It is the filter n and N read, and one the filter line recalls.
	h.press("esc", "n")
	if h.on() != "Fix login" {
		t.Fatalf("n went to %q", h.on())
	}
	h.press("/", "up")
	if !strings.HasPrefix(h.bottom(), "Filter › #bug") {
		t.Fatalf("recall = %q", h.bottom())
	}
	h.press("esc")
	// A word of the text is read without what is around it, and a change
	// made on the way in is kept.
	h.press("g", "g", "j", "w", "x", "w", "*")
	if h.header() != "Projects  · 1 to do  / cache" || h.on() != "ix cache" {
		t.Fatalf("header %q on %q", h.header(), h.on())
	}
	h.press("esc", "j", "j", "w", "f", " ", "*")
	if !strings.HasPrefix(h.status(), "no word under the caret") || h.m.edit == nil {
		t.Fatalf("status = %q", h.status())
	}
}

func TestFoldingABranchAllTheWayDown(t *testing.T) {
	h := branches(t)
	h.press("z", "c")
	if got := h.shown(); got != "Alpha:, Beta:, b1 milk, Gamma, Delta milk:" {
		t.Fatalf("folded = %q", got)
	}
	// Every level under it was folded, not only the first.
	h.press("l")
	if got := h.shown(); got != "Alpha:, a1, a2, a3, Beta:, b1 milk, Gamma, Delta milk:" {
		t.Fatalf("one level = %q", got)
	}
	h.press("z", "o")
	if len(h.m.vis) != 9 {
		t.Fatalf("unfolded:\n%s", h.rows())
	}
	// A count and a selection take several branches; the cursor stays shown.
	h.press("}", "v", "g", "g", "v", "z", "c")
	if got := h.shown(); got != "Alpha:, Beta:, Gamma, Delta milk:" {
		t.Fatalf("selection = %q", got)
	}
	h.press("esc", "g", "g", "9", "z", "o")
	if len(h.m.vis) != 9 {
		t.Fatalf("counted:\n%s", h.rows())
	}
	h.press("G", "z", "c")
	if !strings.HasPrefix(h.status(), "no rows under this one") {
		t.Fatalf("status = %q", h.status())
	}
	// Folds survive a restart, as every fold does.
	h.press("g", "g", "z", "c")
	if folds, _ := h.st.Folds(); len(folds) != 2 {
		t.Fatalf("stored folds = %v", folds)
	}
	h.press("/", `"a"`, "enter", "z", "c")
	if !strings.HasPrefix(h.status(), "nothing to fold") {
		t.Fatalf("status = %q", h.status())
	}
}

func TestPlacingTheRowInThePane(t *testing.T) {
	h := countOutline(t, 40) // the pane shows ten rows
	h.press("2", "0", "j")
	if h.m.top != 11 {
		t.Fatalf("top = %d", h.m.top)
	}
	for _, tc := range []struct {
		key string
		top int
	}{{"t", 20}, {"z", 16}, {"b", 11}, {"t", 20}} {
		h.press("z", tc.key)
		if h.m.top != tc.top || h.m.cursor != 20 {
			t.Fatalf("z%s: top %d cursor %d, want top %d", tc.key, h.m.top, h.m.cursor, tc.top)
		}
	}
	// The row stays where it was put while the cursor moves inside the pane.
	h.press("j")
	if h.m.top != 20 {
		t.Fatalf("top after j = %d", h.m.top)
	}
	// At either end there is only so far to go, and a sweep is not ended.
	h.press("G", "z", "t")
	if h.m.top != 30 {
		t.Fatalf("top at the end = %d", h.m.top)
	}
	h.press("g", "g", "V", "j", "z", "z", "j")
	if h.m.top != 0 || len(h.m.picked) != 3 {
		t.Fatalf("top %d, picked %d", h.m.top, len(h.m.picked))
	}
}

func TestSubstituteChangesTextAcrossRows(t *testing.T) {
	h := work(t)
	steps := len(h.m.undo)
	h.command("s/Fix/Mend")
	if h.on() != "Mend cache" || h.status() != "changed 1 row" {
		t.Fatalf("on %q, status %q", h.on(), h.status())
	}
	// The dot key makes it again, on the row or the selection.
	h.press("j", "j", ".")
	if h.on() != "Mend login" {
		t.Fatalf("dot: %q", h.on())
	}
	h.press("u", "u")
	h.command("all")
	h.command("s/Fix/Mend/")
	if !strings.HasPrefix(h.status(), "changed 2 rows") || len(h.m.undo) != steps+1 {
		t.Fatalf("status %q", h.status())
	}
	h.press("esc")
	h.want(
		"  ▾ Beta",
		"▌   ○ Mend cache #bug",
		"  ▾ Alpha",
		"    ○ Mend login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	h.press("u")
	// Any mark may stand for the slash, every match in a row is changed,
	// and new may be nothing or end in a blank.
	h.press("g", "g", "j")
	h.command("s,c,k,")
	h.command("s|kak|a |")
	h.command("s/Fix //")
	if h.on() != "a he" {
		t.Fatalf("on %q", h.on())
	}
	// Fields are not text, and a line that is not whole changes nothing.
	for line, want := range map[string]string{
		"s/bug/pest": "no row here holds bug",
		"s/he":       "write it as s/old/new",
		"s//x":       "write it as s/old/new",
		"s/a/b/c":    "write it as s/old/new",
	} {
		h.command(line)
		if !strings.HasPrefix(h.status(), want) {
			t.Fatalf("%s: status = %q", line, h.status())
		}
	}
	if h.on() != "a he" || h.field("a he", "tags") != "bug" {
		t.Fatal("a refused line changed the row")
	}
}

func TestListAnswersAFilterFromAShell(t *testing.T) {
	h := work(t)
	h.press("a", `"Buy milk #bug"`, "enter", "esc", "t")
	ops := len(h.m.undo)
	for filter, want := range map[string]string{
		"#bug !":             "Fix login ! #bug $open\tAlpha",
		"#bug":               "Buy milk #bug $open\tInbox|Fix cache #bug $open\tBeta|Fix login ! #bug $open\tAlpha|Refactor auth #bug $open\tAlpha",
		"$today":             "Fix cache #bug $open\tBeta",
		">#docs":             "Alpha\t",
		"@ | ( milk -beta )": "Buy milk #bug $open\tInbox|Write docs @2026-11-20 #docs $open\tAlpha",
		"nothing":            "",
	} {
		if got := strings.Join(newModel(h.m.tree, h.st, nil, h.m.now).list(filter), "|"); got != want {
			t.Errorf("ls %q = %q, want %q", filter, got, want)
		}
	}
	// With no filter it is what :export writes.
	want := "Inbox|\tBuy milk #bug|Beta|\tFix cache #bug|Alpha|\tFix login ! #bug|\tWrite docs @2026-11-20 #docs|\tRefactor auth #bug"
	if got := strings.Join(newModel(h.m.tree, h.st, nil, h.m.now).list(" "), "|"); got != want {
		t.Errorf("ls = %q", got)
	}
	h.press("j")
	if len(h.m.undo) != ops {
		t.Fatal("listing changed the outline")
	}
}
