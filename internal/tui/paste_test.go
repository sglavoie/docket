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
		{0, "First"}, {1, "Child a"}, {2, "Deep"}, {1, "Child b"},
		{0, "Second"}, {0, "ticked $done"}, {0, "open"}, {1, "nested"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestPastingLinesWhileTypingAddsRows(t *testing.T) {
	h := project(t)
	h.press("o", `"Plan: "`)
	h.pasteText("the release\n  Write notes #docs\n  Tag it @fri\n    after CI $note\nAnnounce\n")
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
		"▌     ○ detail",
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
