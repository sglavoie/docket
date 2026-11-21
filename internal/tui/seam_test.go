package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindStepsIntoTheTextOnTheMatch(t *testing.T) {
	h := work(t)
	h.press("g", "n")
	h.wantStatus("nothing to find: keep a filter with / and Enter first")
	h.press("/", `"cache|login"`, "enter", "esc", "g", "g", "g", "n")
	h.wantTyping("Fix cache #bug", 4)
	h.press("n")
	h.wantTyping("Fix login ! #bug", 4)
	h.press("N")
	h.wantTyping("Fix cache #bug", 4)
	// The change made on one match is made again on the next.
	h.press("c", "w", `"queue"`, "esc", "n", ".")
	h.wantTyping("Fix queue ! #bug", 8)
	h.press("n")
	h.wantStatus("no match further on")
	h.press("esc")
	h.want(
		"  ▾ Beta",
		"    ○ Fix queue #bug",
		"  ▾ Alpha",
		"▌   ○ Fix queue ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	// Each row was its own undo step.
	h.press("u")
	if got := h.on(); got != "Fix login" {
		t.Fatalf("one undo took back %q", got)
	}

	// A tag the filter names is a place to land, and gN reads backwards.
	h.press("/", `"#docs"`, "enter", "esc", "G", "g", "N")
	line, at := h.typing()
	if !strings.HasPrefix(line, "Write docs") || at != strings.Index(line, "#docs") {
		t.Fatalf("gN landed at %d of %q", at, line)
	}
	// As a motion for an operator it stays in the row.
	h.press("esc", "/", `"docs|#docs"`, "enter", "esc", "g", "n", "d", "n")
	if line, _ := h.typing(); !strings.HasPrefix(line, "Write #docs") {
		t.Fatalf("dn took %q", line)
	}
	h.press("u", "esc", "/", `"$open"`, "enter", "esc", "g", "n")
	h.wantStatus("the filter names no word or tag to land on: w steps into the text")
}

func TestAFilterReadsItselfBack(t *testing.T) {
	today := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for q, want := range map[string]string{
		"":                      "",
		"$done ~7d":             "done and changed since Nov 8",
		"-_$open|doing":         "no row under it: open or doing",
		"#home $open | #work !": "#home and open, or #work and flagged",
		"fix ( #docs | cache )": `"fix" and (#docs, or "cache")`,
		"#bug -( cache | ! )":   `#bug and not ("cache", or flagged)`,
		"-$":                    "a note or heading",
		"^#work $next":          "a row above it: #work and its project's next action",
		"#est<3 -milk|eggs":     `#est at most 3 and not ("milk" or "eggs")`,
		"@<+30d <$waiting":      "due by Dec 15 and its parent: waiting",
		`"per the spec" &.`:     `"per the spec" and links to this row`,
		"$first ->$ @@later":    "first workable row under its parent and no row directly under it: a task and starts later",
	} {
		if got := readBack(q, today); got != want {
			t.Errorf("%q reads %q, want %q", q, got, want)
		}
	}
	h := work(t)
	h.press("/", `"-_$open|doing"`)
	if got := h.status(); !strings.Contains(got, "in context · no row under it: open or doing") {
		t.Fatalf("status = %q", got)
	}
	// A tag being typed still names the tags there are.
	h.press("esc", "/", `"#b"`)
	if got := h.status(); !strings.Contains(got, "#bug 3") {
		t.Fatalf("status = %q", got)
	}
}

func TestAChangedFilterSaysWhenAndReadsNewestFirst(t *testing.T) {
	h := work(t)
	h.press("x")
	h.clock = h.clock.Add(3 * 24 * time.Hour)
	h.press("G", "x", "/", `"$done ~1w"`, "tab")
	h.want(
		"▌ ✓ Refactor auth #bug ~today · Alpha",
		"  ✓ Fix cache #bug ~Nov 15 · Beta",
	)
	// In context the order is the outline's, and each match still says when.
	h.press("tab")
	h.want(
		"  ▾ Beta",
		"▌   ✓ Fix cache #bug ~Nov 15",
		"  ▾ Alpha",
		"    ✓ Refactor auth #bug ~today",
	)
	// A deadline the filter asks about still orders the list.
	h.press("esc", "/", `"@ ~1m"`, "tab")
	h.want("▌ ○ Write docs @fri #docs ~Nov 15 · Alpha")
}

func TestRecentListsWhatIsLeftNewestFirst(t *testing.T) {
	h := work(t)
	h.clock = h.clock.Add(2 * 24 * time.Hour)
	h.press("x", "G", "o", `"Newest"`, "esc")
	// A ~ alone holds for every row and only orders the list.
	h.press("/", `"~"`)
	h.wantStatus("Filter › ~   7 matches · in context · newest change first")
	h.press("esc", "f", `"recent"`, "enter")
	h.want(
		"▌ ○ Newest ~today · Alpha",
		"  ○ Fix login ! #bug ~Nov 15 · Alpha",
		"  ○ Write docs @fri #docs ~Nov 15 · Alpha",
		"  ○ Refactor auth #bug ~Nov 15 · Alpha",
	)
}

func TestSavedFiltersFollowTheListsInTheSidebar(t *testing.T) {
	h := work(t)
	h.press("/", `"#bug"`, "enter")
	h.command("save bugs")
	if !strings.Contains(h.side(), "Stale\n/bugs\n") {
		t.Fatalf("sidebar:\n%s", h.side())
	}
	h.press("esc", "tab", "g", "g", "j", "j", "j", "j", "enter")
	if h.m.at.query != "#bug" || h.m.sideFocus {
		t.Fatalf("the sidebar opened %+v", h.m.at)
	}
	h.press("ctrl+o")
	if h.m.at.query != "" {
		t.Fatal("Ctrl+O did not come back from the filter")
	}
	h.command("forget bugs")
	if strings.Contains(h.side(), "/bugs") {
		t.Fatalf("sidebar kept a forgotten filter:\n%s", h.side())
	}
	h.press("u")
	if !strings.Contains(h.side(), "/bugs") {
		t.Fatalf("undo did not bring the filter back:\n%s", h.side())
	}
}

func TestImportReadsOtherOutliners(t *testing.T) {
	today := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	files := map[string][2]string{
		"list.taskpaper": {
			"Home:\n" +
				"\t- Call Bob @due(2026-12-01) @flagged @phone\n" +
				"\t- Paid rent @done(2026-11-01)\n" +
				"\t\ta note about it\n" +
				"\t- Ask about #5 and a@b @priority(1) @waiting on(x)\n",
			"Home:\n" +
				"\tCall Bob ! @2026-12-01 #phone $open\n" +
				"\tPaid rent $done\n" +
				"\t\t// a note about it\n" +
				"\tAsk about #5 and a@b on(x) #priority=1 #waiting $open\n",
		},
		"trip.opml": {
			`<?xml version="1.0"?><opml version="2.0"><head><title>x</title></head><body>` +
				`<outline text="Trip &amp; plans" _note="first line&#10;second line">` +
				`<outline text="Book flights" _status="checked"/><outline text="Pack #bag"/></outline></body></opml>`,
			"// Trip & plans\n" +
				"\t// first line\n" +
				"\t// second line\n" +
				"\tBook flights $done\n" +
				"\t// Pack \\#bag\n",
		},
		"plan.bike": {
			`<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><meta charset="utf-8"/></head>` +
				`<body><ul id="r"><li id="a" data-type="heading"><p>Plan</p><ul>` +
				`<li id="b" data-type="task"><p>Write <strong>draft</strong> now</p></li>` +
				`<li id="c" data-type="task" data-done="2026-11-01T10:00:00Z"><p>Outline</p></li>` +
				`<li id="d"><p>Just a thought</p></li></ul></li></ul></body></html>`,
			"Plan:\n" +
				"\tWrite draft now $open\n" +
				"\tOutline $done\n" +
				"\t// Just a thought\n",
		},
	}
	dir := t.TempDir()
	for name, f := range files {
		got, err := readForeign(name, []byte(f[0]), today)
		if err != nil || got != f[1] {
			t.Errorf("%s reads (%v):\n%s\nwant:\n%s", name, err, got, f[1])
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(f[0]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readForeign("bad.opml", []byte("<opml><body><outline"), today); err == nil {
		t.Error("a broken file was read")
	}

	h := start(t).wide()
	h.press("o", `"Here"`, "esc")
	h.command("import " + filepath.Join(dir, "list.taskpaper"))
	if want := "imported 5 rows from " + filepath.Join(dir, "list.taskpaper"); h.m.msg != want {
		t.Fatalf("msg = %q, want %q", h.m.msg, want)
	}
	h.press("g", "g")
	h.command("import " + filepath.Join(dir, "plan.bike"))
	h.want(
		"  · Here",
		"  ▾ Plan: 1/2",
		"    ○ Write draft now",
		"    ✓ Outline",
		"▌   · Just a thought",
		"  ▾ Home: 1/3",
		"    ○ Call Bob ! @Dec 1 #phone",
		"    ✓ Paid rent",
		"      · a note about it",
		"    ○ Ask about #5 and a@b on(x) #priority=1 #waiting",
	)
	// One undo takes a whole file back.
	h.press("u")
	if strings.Contains(h.rows(), "Plan:") || !strings.Contains(h.rows(), "Home:") {
		t.Fatalf("undo left:\n%s", h.rows())
	}
}

// A word that starts like a sigil and cannot be read is looked for as text.
// It is drawn in the warning colour while typed; read back, it says so and
// how a word of its kind is written, as the stamp line does.
func TestAFilterSaysWhenASigilIsReadAsText(t *testing.T) {
	today := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for q, want := range map[string]string{
		"@tomorow":     `"@tomorow" as text: a day reads as @fri, @+3d, @dec4 or @2026-12-04`,
		"$wating #bug": `"$wating" as text: a state is $open, $doing, $waiting, $done, $canceled, $parked or $note and #bug`,
		"-~7x":         `not "~7x" as text: a time reads as ~today, ~7d, ~2w, ~1m or ~1y, and ~~7d for an edit`,
		`"@home"`:      `"@home"`,
		"milk":         `"milk"`,
	} {
		if got := readBack(q, today); got != want {
			t.Errorf("%q reads %q, want %q", q, got, want)
		}
	}
	h := work(t)
	h.press("/", `"@tomorow"`)
	if got := h.status(); !strings.Contains(got, `no match · in context · "@tomorow" as text: a day reads as`) {
		t.Fatalf("status = %q", got)
	}
}
