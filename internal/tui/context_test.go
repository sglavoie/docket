package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/sigil"
)

func TestAContextLeadsTheTitleInItsColour(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Errands:"`, "enter", "tab", `"Call Ann ##phone #ann"`, "enter", `"Buy milk ##town"`, "enter", `"Plan ##ph"`)
	// Typed, the contexts in use are named with their counts and offered.
	if got := h.above(); !strings.Contains(got, "phone  1") || strings.Contains(h.status(), "##phone") {
		t.Fatalf("typing: %q\n%s", h.status(), got)
	}
	h.press("right", "esc")
	if got := h.field("Plan", sigil.FieldContext); got != "phone" {
		t.Fatalf("context = %q", got)
	}
	h.want("  ▾ Errands: 0/3", "    ○ [phone] Call Ann #ann", "    ○ [town] Buy milk", "▌   ○ [phone] Plan")
	raw := h.raw()
	if strings.Count(raw, h.m.tagColor("phone")+"[phone]") != 2 || !strings.Contains(raw, h.m.tagColor("town")+"[town]") {
		t.Fatalf("contexts are not drawn in their colours:\n%q", raw)
	}
	// A opens the canonical line, with the context after the tags.
	h.press("k", "k", "A")
	if line, _ := h.typing(); line != "Call Ann #ann ##phone" {
		t.Fatalf("line = %q", line)
	}
	h.press("esc")

	// ## alone lists them all, most used first; a new one says so.
	h.press("/", `"##"`)
	if got := h.status(); !strings.HasSuffix(got, "##phone 2 · ##town 1") {
		t.Fatalf("filter: %q", got)
	}
	h.press(`"x"`)
	if got := h.status(); !strings.HasSuffix(got, "a new context") {
		t.Fatalf("filter: %q", got)
	}
	h.press("backspace", "backspace", "backspace", `"##t"`, "tab")
	if got := string(h.m.ask.ed.buf); got != "##town" {
		t.Fatalf("tab wrote %q", got)
	}
	h.press("esc", "esc")
}

func TestTheFilterAndStampReadAContext(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Errands:"`, "enter", "tab", `"Call Ann ##phone"`, "enter", `"Buy milk ##town"`, "enter", `"Rest"`, "esc")
	matched := func(q string) []string {
		h.t.Helper()
		var out []string
		for _, id := range h.m.tree.Children(h.m.tree.Children("")[len(h.m.tree.Children(""))-1]) {
			r := h.m.tree.Row(id)
			if h.m.matches(r, h.m.query(q, "", h.m.today()), h.m.today(), make([]int, 8), nil) {
				out = append(out, r.Text())
			}
		}
		return out
	}
	for q, want := range map[string]string{
		"##phone":      "Call Ann",
		"##":           "Call Ann,Buy milk",
		"-##":          "Rest",
		"##phone|town": "Call Ann,Buy milk",
		"-##town":      "Call Ann,Rest",
	} {
		if got := strings.Join(matched(q), ","); got != want {
			t.Errorf("%s: %q, want %q", q, got, want)
		}
	}
	if got := parseQuery("##phone", h.m.today())[0].any[0].words(h.m.today()); got != "##phone" {
		t.Errorf("read back %q", got)
	}

	// The stamp line sets a context, and takes it off.
	h.press("#", `"#desk"`, "enter")
	if got := h.field("Rest", sigil.FieldContext); got != "desk" {
		t.Fatalf("stamped %q", got)
	}
	h.press("#", "backspace", `"-##town"`, "enter")
	if got := h.field("Rest", sigil.FieldContext); got != "desk" {
		t.Fatalf("-##town took off %q", got)
	}
	h.press("#", "backspace", `"-##"`, "enter")
	if got := h.field("Rest", sigil.FieldContext); got != "" {
		t.Fatalf("-## left %q", got)
	}
}
