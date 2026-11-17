package tui

import (
	"maps"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

func TestSigilsAreColouredAsTheyAreTyped(t *testing.T) {
	h := work(t)
	h.press("o", `"Ship @fri #v2 $doing ! @firday #123 \#lit plain"`)
	raw := h.m.View().Content
	for _, w := range []string{"@fri", "#v2", "$doing", "!"} {
		if !strings.Contains(raw, darkPalette.accent+w+sgrReset) {
			t.Errorf("%s is not shown as a sigil:\n%q", w, raw)
		}
	}
	// A word that starts like a sigil and cannot be read is flagged: it
	// will stay in the text.
	for _, w := range []string{"@firday", "#123"} {
		if !strings.Contains(raw, darkPalette.warn+w+sgrReset) {
			t.Errorf("%s is not flagged as unread:\n%q", w, raw)
		}
	}
	for _, w := range []string{`\#lit`, "plain", "Ship"} {
		if strings.Contains(raw, darkPalette.accent+w) || strings.Contains(raw, darkPalette.warn+w) {
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
		"▌   ○ Tidy #bug",
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
	h.press("A", "right", "left", "left", "right", `"X"`, "esc", "u")
	h.press("A", `" #zz"`, "right", "esc")
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
	if !strings.Contains(raw, darkPalette.accent+"#docs"+sgrReset) || !strings.Contains(raw, darkPalette.warn+"oops"+sgrReset) {
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
	if !strings.Contains(raw, darkPalette.accent+"@overdue"+sgrReset) || !strings.Contains(raw, darkPalette.warn+"@nope"+sgrReset) {
		t.Fatalf("filter line is not coloured:\n%q", raw)
	}
	h.press("esc")

	// Capture: a %name is seen to resolve, or not, before Enter.
	h.press("a", `"Call back %alp"`)
	if !strings.HasSuffix(h.bottom(), "files in Alpha") || !strings.Contains(h.m.View().Content, darkPalette.accent+"%alp"+sgrReset) {
		t.Fatalf("capture line = %q", h.bottom())
	}
	h.press(`"x"`)
	if !strings.HasSuffix(h.bottom(), "files in Inbox") || !strings.Contains(h.m.View().Content, darkPalette.warn+"%alpx"+sgrReset) {
		t.Fatalf("capture line = %q", h.bottom())
	}
	h.press("backspace", "backspace", "backspace", "backspace", "backspace", "esc")
}

func TestEnterSplitsAndBackspaceJoins(t *testing.T) {
	// Check fields and positions, including absence of a deletion marker;
	// Modified is intentionally not compared because undo is another write.
	type rowState struct {
		parent, pos string
		fields      map[string]string
	}
	snapshot := func(h *harness) map[string]rowState {
		rows := map[string]rowState{}
		var visit func(string)
		visit = func(parent string) {
			for _, id := range h.m.tree.Children(parent) {
				r := h.m.tree.Row(id)
				rows[id] = rowState{r.Parent, r.Pos, maps.Clone(r.Fields)}
				visit(id)
			}
		}
		visit("")
		return rows
	}
	restored := func(t *testing.T, h *harness, before map[string]rowState) {
		t.Helper()
		if got := snapshot(h); !reflect.DeepEqual(got, before) {
			t.Fatalf("rows = %#v; want %#v", got, before)
		}
	}
	t.Run("literal split and whole undo", func(t *testing.T) {
		h := start(t)
		h.press("o", `"Café Ship #bug $doing !"`, "esc")
		id := h.m.current()
		before, steps := snapshot(h), len(h.m.undo)
		h.press("A", "ctrl+a", "right", "right", "right", "right", "right", "enter")
		e := h.m.edit
		if e == nil || e.caret != 0 || string(e.buf) != "Ship ! #bug $doing" || h.m.tree.Row(id).Text() != "Café" {
			t.Fatalf("split: editor=%+v, original=%+v", e, h.m.tree.Row(id))
		}
		h.press("esc")
		h.want("  · Café", "▌ ● Ship ! #bug")
		if len(h.m.undo) != steps+1 {
			t.Fatal("split was not one undo step")
		}
		h.press("u")
		restored(t, h, before)
		h.press("ctrl+r")
		h.want("  · Café", "▌ ● Ship ! #bug")
	})
	t.Run("split then backspace", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First Second"`, "esc")
		before := snapshot(h)
		h.press("A", "ctrl+a")
		for range 6 {
			h.press("right")
		}
		h.press("enter")
		fresh := h.m.edit.row
		h.press("backspace")
		if h.m.tree.Row(fresh) != nil || string(h.m.edit.buf) != "First Second" || h.m.edit.caret != 5 {
			t.Fatalf("joined editor = %+v", h.m.edit)
		}
		h.press("esc")
		restored(t, h, before)
		h.press("u")
		restored(t, h, before)
	})
	t.Run("existing join fields and undo survive reopen", func(t *testing.T) {
		h := start(t).wide()
		h.press("o", `"First #old $open"`, "enter", `"Second #new $waiting @fri !"`, "esc")
		second := h.m.current()
		first := h.m.vis[h.m.cursor-1].id
		before, steps := snapshot(h), len(h.m.undo)
		h.press("A", "ctrl+a", "backspace")
		if h.m.edit.row != first || h.m.edit.caret != len([]rune("First $open #old")) || !h.m.tree.Row(second).Deleted() {
			t.Fatalf("join editor = %+v", h.m.edit)
		}
		h.press("esc")
		h.want("▌ ◔ First Second ! @fri #old #new")
		fields := h.m.tree.Row(first).Fields
		if fields[sigil.FieldState] != sigil.Waiting || fields[sigil.FieldTags] != "old new" || fields[sigil.FieldDue] != "2026-11-20" || fields[sigil.FieldFlag] != "1" {
			t.Fatalf("joined fields = %v", fields)
		}
		if len(h.m.undo) != steps+1 {
			t.Fatal("join was not one undo step")
		}
		h = reopen(t, h)
		h.press("u")
		restored(t, h, before)
		if _, found := h.m.tree.Row(second).Fields[outline.FieldDeleted]; found {
			t.Fatal("undo kept deletion marker")
		}
	})
	t.Run("fresh join undo", func(t *testing.T) {
		h := project(t)
		before, steps := snapshot(h), len(h.m.undo)
		h.press("o", `"More #new $doing"`)
		fresh := h.m.edit.row
		h.press("ctrl+a", "backspace", "esc")
		if h.m.tree.Row(fresh) != nil {
			t.Fatal("fresh joined row was not forgotten")
		}
		h.want("  ▾ Project", "    ○ First", "▌   ● Second More #new")
		if len(h.m.undo) != steps+1 {
			t.Fatal("fresh join was not one undo step")
		}
		h.press("u")
		restored(t, h, before)
	})
	t.Run("displayed predecessor rather than sibling", func(t *testing.T) {
		h := start(t)
		h.press("o", `"Parent"`, "enter", "tab", `"Child"`, "enter", "shift+tab", `"Next"`, "esc")
		before := snapshot(h)
		h.press("A", "ctrl+a", "backspace", "esc")
		h.want("  ▾ Parent", "▌   ○ Child Next")
		h.press("u")
		restored(t, h, before)
	})
	t.Run("split placement open and folded parent", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First Second"`, "enter", "tab", `"Child"`, "esc", "k")
		before := snapshot(h)
		for _, folded := range []bool{false, true} {
			if folded {
				h.press("h")
			}
			h.press("A", "ctrl+a")
			for range 6 {
				h.press("right")
			}
			h.press("enter", "esc")
			if folded {
				h.want("  ▸ First", "▌ · Second")
			} else {
				h.want("  ▾ First", "▌   ○ Second", "    ○ Child")
			}
			h.press("u")
			restored(t, h, before)
		}
	})
	t.Run("children and first row refuse silently", func(t *testing.T) {
		h := project(t)
		h.press("k", "o", `"Parent"`, "enter", "tab", `"Child"`, "esc", "k")
		for _, first := range []bool{false, true} {
			if first {
				h.press("esc", "g", "g")
			}
			h.press("A", "ctrl+a")
			before, steps, ed := snapshot(h), len(h.m.undo), *h.m.edit
			h.press("backspace")
			restored(t, h, before)
			if !reflect.DeepEqual(*h.m.edit, ed) || len(h.m.undo) != steps || h.m.msg != "" {
				t.Fatal("refusal changed editor or history")
			}
		}
	})
	t.Run("blank fresh disappears", func(t *testing.T) {
		h := project(t)
		before, steps := snapshot(h), len(h.m.undo)
		h.press("o")
		fresh := h.m.edit.row
		h.press("backspace")
		restored(t, h, before)
		if h.m.edit != nil || h.m.tree.Row(fresh) != nil || len(h.m.undo) != steps {
			t.Fatal("blank fresh row did not disappear cleanly")
		}
	})
	for _, view := range []string{"list", "filter"} {
		t.Run(view+" refuses both operations", func(t *testing.T) {
			h := start(t)
			h.press("o", `"First #match $waiting"`, "enter", `"Second #match $waiting"`, "esc")
			if view == "list" {
				h.press("f", `"Waiting"`, "enter")
				if h.m.at.list == "" {
					t.Fatal("fixture is not a list")
				}
			} else {
				h.press("/", `"#match"`, "enter")
				if h.m.at.query == "" {
					t.Fatal("fixture is not a filter")
				}
			}
			h.press("G")
			if h.m.cursor == 0 {
				t.Fatal("fixture has no visible predecessor")
			}
			before, steps := snapshot(h), len(h.m.undo)
			h.press("A", "ctrl+a", "right", "enter")
			restored(t, h, before)
			if h.m.edit != nil || len(h.m.undo) != steps {
				t.Fatal("Enter split or failed to keep row")
			}
			h.press("A", "ctrl+a")
			ed, drawn := *h.m.edit, h.rows()
			h.press("backspace")
			restored(t, h, before)
			if !reflect.DeepEqual(*h.m.edit, ed) || h.rows() != drawn || len(h.m.undo) != steps || h.m.msg != "" {
				t.Fatal("Backspace changed editor, rows or undo")
			}
		})
	}
	t.Run("endpoint enter behavior", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First"`, "esc")
		before := snapshot(h)
		h.press("A", "ctrl+a", "enter", "A", "enter")
		restored(t, h, before)
		if h.m.edit != nil {
			t.Fatal("existing endpoint created a row")
		}
		h.press("o", `"Second"`, "ctrl+a", "enter")
		if h.m.edit == nil || !h.m.edit.fresh || len(h.m.edit.buf) != 0 {
			t.Fatal("fresh endpoint did not chain")
		}
	})
	t.Run("fresh split and repeated split share undo", func(t *testing.T) {
		h := start(t)
		before, steps := snapshot(h), len(h.m.undo)
		h.press("o", `"One Two Three"`, "ctrl+a")
		for range 4 {
			h.press("right")
		}
		h.press("enter")
		for range 4 {
			h.press("right")
		}
		h.press("enter", "esc")
		h.want("  · One", "  · Two", "▌ · Three")
		if len(h.m.undo) != steps+1 {
			t.Fatal("fresh splits made multiple undo steps")
		}
		h.press("u")
		restored(t, h, before)
	})
	t.Run("first leaf has no predecessor", func(t *testing.T) {
		h := start(t)
		h.press("o", `"Only"`, "esc", "A", "ctrl+a")
		before, steps, ed := snapshot(h), len(h.m.undo), *h.m.edit
		h.press("backspace")
		restored(t, h, before)
		if !reflect.DeepEqual(*h.m.edit, ed) || len(h.m.undo) != steps || h.m.msg != "" {
			t.Fatal("first leaf changed")
		}
	})
	t.Run("join save failure restores both rows", func(t *testing.T) {
		h := project(t)
		before, steps := snapshot(h), len(h.m.undo)
		h.m.backend = &flaky{Backend: h.m.backend, fail: true}
		h.press("A", "ctrl+a", "backspace", "esc")
		restored(t, h, before)
		if len(h.m.undo) != steps || !strings.Contains(h.m.msg, "disk full") {
			t.Fatal("failed join changed history or hid error")
		}
	})
	t.Run("paste after split remains one undo", func(t *testing.T) {
		h := start(t)
		h.press("o", `"First Second"`, "esc")
		before, steps := snapshot(h), len(h.m.undo)
		h.press("A", "ctrl+a")
		for range 6 {
			h.press("right")
		}
		h.press("enter")
		h.m.Update(tea.PasteMsg{Content: "More\nLast"})
		if len(h.m.undo) != steps+1 {
			t.Fatal("paste broke split undo")
		}
		h.press("u")
		restored(t, h, before)
	})
}
