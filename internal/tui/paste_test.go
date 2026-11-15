package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func (h *harness) pasteText(s string) *harness {
	h.m.Update(tea.PasteMsg{Content: s})
	h.m.View()
	return h
}

func TestReadPaste(t *testing.T) {
	got := readPaste("  First\r\n\r\n      Child   a\n\t\tDeep\n      Child b\nSecond\n- [x] ticked\n* [ ] open\n   - nested\n")
	want := []pasted{
		{0, "First", false}, {1, "Child a", false}, {2, "Deep", false}, {1, "Child b", false},
		{0, "Second", false}, {0, "ticked $done", true}, {0, "open $open", true}, {1, "nested", false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestPastingLinesWhileTypingAddsRows(t *testing.T) {
	h := project(t)
	h.press("o", `"Plan: "`)
	h.pasteText("the release\n  Write notes #docs $open\n  Tag it @fri $open\n    after CI $note\nAnnounce\n")
	if h.m.msg != "pasted 4 rows" || h.m.edit != nil {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ○ Plan: the release",
		"      ○ Write notes #docs",
		"      ○ Tag it @fri",
		"          after CI",
		"▌   ○ Announce",
	)
	// The typed row and everything pasted under it are one undo step.
	h.press("u")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ○ Second",
	)
	h.press("ctrl+r")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ○ Second",
		"    ○ Plan: the release",
		"      ○ Write notes #docs",
		"      ○ Tag it @fri",
		"          after CI",
		"▌   ○ Announce",
	)
}

func TestPastingAListOnARowAddsItBelow(t *testing.T) {
	h := project(t)
	h.press("k")
	h.pasteText("- [x] shipped\n- [ ] pending\n\t- detail\n")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"    ✓ shipped",
		"    ○ pending",
		"▌       detail",
		"    ○ Second",
	)
	h.press("u")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ○ Second",
	)
}

func TestPastingOneLineOrIntoAPromptIsTyping(t *testing.T) {
	h := project(t)
	h.press("e", "ctrl+a")
	h.pasteText("Very ")
	h.press("esc")
	h.want(
		"  ▾ Project",
		"    ○ First",
		"▌   ○ Very Second",
	)
	h.press("a")
	h.pasteText("Buy\n  milk\n")
	if got := string(h.m.ask.ed.buf); got != "Buy milk" {
		t.Fatalf("capture line = %q", got)
	}
	h.press("esc", "/")
	h.pasteText("very")
	h.want(
		"▌ ▾ Project",
		"    ○ Very Second",
	)
	// A filter shows only part of the tree, so rows cannot be pasted in.
	h.press("enter")
	h.pasteText("one\ntwo\n")
	if h.m.msg != "clear the filter first (Esc)" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.want(
		"▌ ▾ Project",
		"    ○ Very Second",
	)
}

func TestCheckboxPasteNormalization(t *testing.T) {
	for _, tc := range []struct {
		content, text string
		checkbox      bool
	}{
		{"[ ] pending $done", "pending $done $open", true},
		{"[x] shipped $open", "shipped $open $done", true},
		{"+ [X] shipped", "shipped $done", true},
		{`\[ ] literal`, `\[ ] literal`, false},
		{`\- [ ] literal`, `\- [ ] literal`, false},
		{"- plain", "plain", false},
	} {
		got := readPaste(tc.content)
		want := []pasted{{level: 0, text: tc.text, checkbox: tc.checkbox}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("readPaste(%q) = %+v, want %+v", tc.content, got, want)
		}
	}

	h := project(t)
	h.press("e", "ctrl+a")
	h.pasteText("[ ] prefix ")
	if got := string(h.m.edit.buf); got != "prefix $openSecond" {
		t.Fatalf("checkbox text should insert at the caret: %q", got)
	}
	h.press("esc", "a")
	h.pasteText("- [ ] literal ")
	if got := string(h.m.ask.ed.buf); got != "- [ ] literal " {
		t.Fatalf("prompt checkbox must remain typed text: %q", got)
	}
	h.pasteText("\n[x] another\n[ ] last")
	if got := string(h.m.ask.ed.buf); got != "- [ ] literal [x] another [ ] last" {
		t.Fatalf("multiline prompt checkbox must remain typed text: %q", got)
	}
}

func TestPastedRowsUseTheirParentsDefault(t *testing.T) {
	h := project(t)
	h.pasteText("[ ] Parent\n  Detail\n    Under note\n  [ ] Subtask $done\n  [x] Finished $open\nHeading:\n  Child")
	h.want("  ▾ Project", "    ○ First", "    ○ Second", "    ○ Parent",
		"      ▾ Detail", "        ○ Under note", "      ○ Subtask", "      ✓ Finished",
		"    ▾ Heading:", "▌     ○ Child")
}
