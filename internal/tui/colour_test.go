package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/sigil"
)

func (h *harness) raw() string { return h.m.View().Content }

// color is the stored colour of the row that reads text, at any depth.
func (h *harness) color(text string) string {
	h.t.Helper()
	var find func(parent string) (string, bool)
	find = func(parent string) (string, bool) {
		for _, id := range h.m.tree.Children(parent) {
			r := h.m.tree.Row(id)
			if r.Text() == text {
				return r.Fields[sigil.FieldColor], true
			}
			if c, ok := find(id); ok {
				return c, true
			}
		}
		return "", false
	}
	c, ok := find("")
	if !ok {
		h.t.Fatalf("no row reads %q", text)
	}
	return c
}

func TestAProjectIsDrawnInTheColourItsSigilNames(t *testing.T) {
	h := two(t)
	teal := namedColors["teal"][0]

	// Typed on the project's line, the colour is drawn in itself and offered
	// by name before it is kept.
	h.press("j", "A", `" ~te"`)
	if !strings.Contains(h.status(), "~teal") {
		t.Fatalf("status = %q, want the colours ~te could become", h.status())
	}
	if !strings.Contains(h.raw(), teal+"~te") {
		t.Fatal("a colour being typed is not drawn in itself")
	}
	h.press("right", "esc")
	if got := h.color("Alpha"); got != "teal" {
		t.Fatalf("color = %q", got)
	}
	// The colour is said by the colour: the row shows no sigil for it.
	h.want("  · Beta", "▌ ▾ Alpha", "    ○ One @tomorrow", "    ◔ Two")
	raw := h.raw()
	if !strings.Contains(raw, teal+sgrBold+"Alpha") {
		t.Fatal("the project's row is not drawn in its colour")
	}
	if !strings.Contains(raw, teal+"Alpha") || strings.Count(raw, teal) < 2 {
		t.Fatal("the sidebar entry is not drawn in the project's colour")
	}
	// A opens the canonical line, colour and all.
	h.press("A")
	if line, _ := h.typing(); line != "Alpha ~teal" {
		t.Fatalf("line = %q", line)
	}
	h.press("esc")

	// Inside the project, the header names it in its colour.
	h.press("tab", "j", "j")
	if lines := strings.Split(h.raw(), "\n"); !strings.Contains(lines[0], teal+"Alpha") {
		t.Fatalf("header = %q", lines[0])
	}
	// In a flat list, the project a row lives in is named in its colour.
	h.press("k", "k", "k", "k")
	if !strings.Contains(h.raw(), "· "+teal+"Alpha") {
		t.Fatal("Waiting does not name the project in its colour")
	}
}

func TestAColourIsStampedAndTakenOff(t *testing.T) {
	h := two(t)
	h.press("#", "backspace", `"~#FF8800"`, "enter")
	if got := h.color("Beta"); got != "#ff8800" {
		t.Fatalf("color = %q", got)
	}
	if !strings.Contains(h.raw(), "\x1b[38;2;255;136;0m"+sgrBold+"Beta") {
		t.Fatal("a hex colour is not drawn as it is")
	}
	h.press("#", "backspace", `"~tael"`, "enter")
	if !strings.Contains(h.status(), "a colour is a name") {
		t.Fatalf("status = %q", h.status())
	}
	h.press("esc", "-", `"~"`, "enter")
	if got := h.color("Beta"); got != "" {
		t.Fatalf("color = %q after -~", got)
	}
	h.press("u")
	if got := h.color("Beta"); got != "#ff8800" {
		t.Fatalf("color = %q after undo", got)
	}
}

func TestOnlyAProjectIsDrawnInItsColour(t *testing.T) {
	h := two(t)
	red := namedColors["red"][0]
	h.press("j", "j", "A", `" ~red"`, "esc")
	if got := h.color("One"); got != "red" {
		t.Fatalf("color = %q", got)
	}
	if strings.Contains(h.raw(), red) {
		t.Fatal("a task is drawn in its colour")
	}
}

func TestTagsAreToldApartByColour(t *testing.T) {
	h := start(t).wide()
	h.press("o", `"Alpha #home"`, "enter", "tab", `"One #home #work"`, "esc")
	home, work := h.m.tagColor("home"), h.m.tagColor("work")
	if home == work {
		t.Fatal("pick two tags that hash apart")
	}
	raw := h.raw()
	if strings.Count(raw, home+"#home") != 2 || !strings.Contains(raw, work+"#work") {
		t.Fatalf("tags are not drawn in their own colours:\n%q", raw)
	}
	// A tag's colour is its name's, whatever value it carries, and the same
	// in every session.
	if h.m.tagColor("est=2h") != h.m.tagColor("est") {
		t.Fatal("a value changes the tag's colour")
	}
	seen := map[string]bool{}
	for _, tag := range []string{"personal", "tradition", "parked", "vta"} {
		seen[h.m.tagColor(tag)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("common tags share too few colours: %d", len(seen))
	}
}

// popup is the screen's lines that show the colour list, trimmed.
func (h *harness) popup() []string {
	var out []string
	for _, l := range h.lines() {
		for _, name := range sigil.ColorNames {
			if strings.Contains(l, " "+name+" ") {
				out = append(out, name)
			}
		}
	}
	return out
}

func TestAColourIsPickedFromAList(t *testing.T) {
	t.Run("on a row in the pane, under the word", func(t *testing.T) {
		h := two(t)
		h.press("j", "A", `" ~p"`)
		if got := h.popup(); strings.Join(got, " ") != "purple pink" {
			t.Fatalf("listed %q", got)
		}
		// The list hangs under the row being typed.
		c := h.m.View().Cursor
		if l := h.lines()[c.Y+1]; !strings.Contains(l, "purple") {
			t.Fatalf("row under the caret = %q", l)
		}
		h.press("tab")
		if line, _ := h.typing(); line != "Alpha ~purple" {
			t.Fatalf("line = %q", line)
		}
		// Each name is drawn in itself; the one written in reverse.
		if !strings.Contains(h.raw(), sgrReverse+sgrBold+" "+namedColors["purple"][0]+"purple") {
			t.Fatal("the name written is not shown reversed in its colour")
		}
		h.press("down")
		if line, _ := h.typing(); line != "Alpha ~pink" {
			t.Fatalf("line = %q", line)
		}
		// Past the end, the word is as it was typed; back again from there.
		h.press("tab")
		if line, _ := h.typing(); line != "Alpha ~p" {
			t.Fatalf("line = %q", line)
		}
		h.press("shift+tab")
		if line, _ := h.typing(); line != "Alpha ~pink" {
			t.Fatalf("line = %q", line)
		}
		// Any other key ends the walk, and a finished name lists nothing.
		h.press("enter")
		if got := h.color("Alpha"); got != "pink" {
			t.Fatalf("color = %q", got)
		}
		if h.popup() != nil {
			t.Fatalf("listed %q after Enter", h.popup())
		}
		// Without a list, Tab indents and Down goes to the next row, as ever.
		h.press("A", `" x"`, "down")
		if line, _ := h.typing(); line != "One @2026-11-16" {
			t.Fatalf("line = %q after Down", line)
		}
	})

	t.Run("in the sidebar entry", func(t *testing.T) {
		h := two(t)
		h.press("tab", "j", "j", "A", `" ~te"`)
		if got := h.popup(); strings.Join(got, " ") != "teal" {
			t.Fatalf("listed %q", got)
		}
		h.press("tab", "enter")
		if got := h.color("Alpha"); got != "teal" || !h.m.sideFocus {
			t.Fatalf("color = %q, focus = %v", got, h.m.sideFocus)
		}
	})

	t.Run("on the stamp line, above it", func(t *testing.T) {
		h := two(t)
		h.press("#", "backspace", `"~g"`)
		if got := h.popup(); strings.Join(got, " ") != "green gray" {
			t.Fatalf("listed %q", got)
		}
		h.press("ctrl+n", "ctrl+n", "enter")
		if got := h.color("Beta"); got != "gray" {
			t.Fatalf("color = %q", got)
		}
	})

	t.Run("not on the filter, where ~ is a time", func(t *testing.T) {
		h := two(t)
		h.press("/", `"~g"`)
		if h.popup() != nil {
			t.Fatalf("listed %q", h.popup())
		}
	})
}

func TestTildeColoursEverySelectedRowAtOnce(t *testing.T) {
	h := two(t)
	// v picks Beta and Alpha; ~ opens the stamp line holding it, with every
	// colour listed.
	h.press("v", "v", "~")
	if got := h.popup(); len(got) != min(listDepth, len(sigil.ColorNames)) {
		t.Fatalf("listed %q", got)
	}
	h.press(`"te"`, "tab", "enter")
	for _, name := range []string{"Beta", "Alpha"} {
		if got := h.color(name); got != "teal" {
			t.Fatalf("%s color = %q", name, got)
		}
	}
	// One undo step takes both back.
	h.press("u")
	for _, name := range []string{"Beta", "Alpha"} {
		if got := h.color(name); got != "" {
			t.Fatalf("%s color = %q after undo", name, got)
		}
	}
	// A count takes that many rows from the cursor instead.
	h.press("esc", "g", "g", "2", "~", `"#abc"`, "enter")
	if h.color("Beta") != "#aabbcc" || h.color("Alpha") != "#aabbcc" {
		t.Fatalf("colors = %q %q", h.color("Beta"), h.color("Alpha"))
	}
}
