package tui

import (
	"strings"
	"testing"
)

func TestSigilsAreColouredAsTheyAreTyped(t *testing.T) {
	h := work(t)
	h.press("o", `"Ship @fri #v2 $doing ! @firday #123 \#lit plain"`)
	raw := h.m.View().Content
	for _, w := range []string{"@fri", "#v2", "$doing", "!"} {
		if !strings.Contains(raw, sgrAccent+w+sgrReset) {
			t.Errorf("%s is not shown as a sigil:\n%q", w, raw)
		}
	}
	// A word that starts like a sigil and cannot be read is flagged: it
	// will stay in the text.
	for _, w := range []string{"@firday", "#123"} {
		if !strings.Contains(raw, sgrWarn+w+sgrReset) {
			t.Errorf("%s is not flagged as unread:\n%q", w, raw)
		}
	}
	for _, w := range []string{`\#lit`, "plain", "Ship"} {
		if strings.Contains(raw, sgrAccent+w) || strings.Contains(raw, sgrWarn+w) {
			t.Errorf("%s is coloured:\n%q", w, raw)
		}
	}
}

func TestTheRightArrowCompletesASigil(t *testing.T) {
	h := work(t)
	// #bug is used three times and #docs once, so #bug is offered first.
	h.press("o", `"Tidy #"`)
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"▌     Tidy #bug",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
	if !strings.HasSuffix(h.status(), "→ completes") {
		t.Fatalf("status = %q", h.status())
	}
	if got := string(h.m.edit.buf); got != "Tidy #" {
		t.Fatalf("the suggestion was typed for me: %q", got)
	}
	h.press(`"d"`, "right", `" $wa"`, "right", `" @to"`, "right", "esc")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"▌   ◔ Tidy @today #docs",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)

	// With nothing to offer, or the caret inside the line, the arrow moves.
	h.press("e", "right", "left", "left", "right", `"X"`, "esc", "u")
	h.press("e", `" #zz"`, "right", "esc")
	h.want(
		"  ▾ Beta",
		"    ○ Fix cache #bug",
		"▌   ◔ Tidy @today #docs #zz",
		"  ▾ Alpha",
		"    ○ Fix login ! #bug",
		"    ○ Write docs @fri #docs",
		"    ○ Refactor auth #bug",
	)
}

func TestPromptsColourAndCompleteInTheirOwnGrammar(t *testing.T) {
	h := work(t)

	// Stamp: the line is sigils only, so a plain word is flagged too.
	h.press("#", `"d"`, "right", `" oops -#b"`)
	raw := h.m.View().Content
	if !strings.Contains(raw, sgrAccent+"#docs"+sgrReset) || !strings.Contains(raw, sgrWarn+"oops"+sgrReset) {
		t.Fatalf("stamp line is not coloured:\n%q", raw)
	}
	if !strings.HasPrefix(h.bottom(), "Stamp › #docs oops -#bug ") {
		t.Fatalf("stamp line = %q", h.bottom())
	}
	h.press("esc")

	// Filter: completing a term narrows the pane at once.
	h.press("/", `"$"`, "tab", `" -#d"`, "right")
	if h.m.at.query != "$ -#docs" {
		t.Fatalf("query = %q", h.m.at.query)
	}
	h.press("esc", "/", `"#do"`, "right")
	h.want("▌ ○ Write docs @fri #docs · Alpha")
	h.press("esc", "/", `"@ov"`, "right", `" @nope"`)
	raw = h.m.View().Content
	if !strings.Contains(raw, sgrAccent+"@overdue"+sgrReset) || !strings.Contains(raw, sgrWarn+"@nope"+sgrReset) {
		t.Fatalf("filter line is not coloured:\n%q", raw)
	}
	h.press("esc")

	// Capture: a %name is seen to resolve, or not, before Enter.
	h.press("a", `"Call back %alp"`)
	if !strings.HasSuffix(h.bottom(), "files in Alpha") || !strings.Contains(h.m.View().Content, sgrAccent+"%alp"+sgrReset) {
		t.Fatalf("capture line = %q", h.bottom())
	}
	h.press(`"x"`)
	if !strings.HasSuffix(h.bottom(), "files in Inbox") || !strings.Contains(h.m.View().Content, sgrWarn+"%alpx"+sgrReset) {
		t.Fatalf("capture line = %q", h.bottom())
	}
	h.press("backspace", "backspace", "backspace", "backspace", "backspace", "esc")
}
