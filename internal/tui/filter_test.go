package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	for _, k := range []string{"e", "o", ">", "J", "enter"} {
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
	h.press("$", `"w"`, "enter", "j", "j", "$", `"w"`, "enter", "p", `"waiting"`, "enter")
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
	if h.m.msg != "commands: q, export [file], closed, review, save name, forget name, sync [folder|off]" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press(":", `"q"`, "enter")
	if !h.m.quit {
		t.Fatal(":q did not quit")
	}
}
