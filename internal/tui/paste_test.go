package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

func (h *harness) pasteText(s string) *harness {
	h.m.Update(tea.PasteMsg{Content: s})
	h.m.View()
	return h
}

func TestReadPaste(t *testing.T) {
	got := readPaste("  First\r\n\r\n      Child   a\n\t\tDeep\n      Child b\nSecond\n- [x] ticked\n* [ ] open\n   - nested\n")
	want := []pasted{
		{0, "First", false, false}, {1, "Child a", false, false}, {2, "Deep", false, false}, {1, "Child b", false, false},
		{0, "Second", false, false}, {0, "ticked $done", true, false}, {0, "open $open", true, false}, {1, "nested", false, false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestPastingLinesWhileTypingAddsRows(t *testing.T) {
	h := project(t)
	h.press("o", `"Plan: "`)
	h.pasteText("the release\n  Write notes #docs $open\n  Tag it @fri $open\n    after CI $note\nAnnounce\n")
	if h.m.msg != "pasted 5 rows" || h.m.edit != nil {
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
		content, text       string
		checkbox, protected bool
	}{
		{"[ ] pending $done", "pending $done $open", true, false},
		{"[x] shipped $open", "shipped $open $done", true, false},
		{"+ [X] shipped", "shipped $done", true, false},
		{`\[ ] literal`, `[ ] literal`, false, true},
		{`\- [ ] literal`, `- [ ] literal`, false, true},
		{"- plain", "plain", false, false},
	} {
		got := readPaste(tc.content)
		want := []pasted{{level: 0, text: tc.text, checkbox: tc.checkbox, protected: tc.protected}}
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

// These fixtures use stored fields, deliberately bypassing Parse so a lossy
// parser cannot silently erase the input before copy is exercised.
type copyRow struct {
	parent int // -1 is the containing heading/task; otherwise a preceding row
	line   sigil.Line
}

func copyFixture(t *testing.T, parentState string, rows []copyRow) (*harness, []string, string) {
	t.Helper()
	h := start(t)
	var ops []outline.Op
	last := map[string]string{}
	add := func(parent string, line sigil.Line) string {
		id := h.m.newID()
		pos := outline.Between(last[parent], "")
		last[parent] = pos
		ops = append(ops, outline.Op{Kind: outline.Insert, Row: id, Parent: parent, Pos: pos})
		for name, value := range line.Fields() {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: name, Value: value})
		}
		return id
	}
	source := add("", sigil.Line{Text: "Source:", State: parentState})
	dest := add("", sigil.Line{Text: "Destination:", State: parentState})
	var ids []string
	for _, r := range rows {
		parent := source
		if r.parent >= 0 {
			parent = ids[r.parent]
		}
		ids = append(ids, add(parent, r.line))
	}
	if !h.m.do(ops...) {
		t.Fatal(h.m.msg)
	}
	h.m.open(entry{kind: kindProject, id: source})
	return h, ids, dest
}

func copyThroughUpdate(t *testing.T, h *harness, expected string, count int) {
	t.Helper()
	if h.m.syncIO != nil || h.m.syncDir != "" {
		t.Fatal("clipboard fixture must have sync inactive")
	}
	gen, undo, redo := h.m.tree.Gen(), len(h.m.undo), len(h.m.redo)
	picked := fmt.Sprint(h.m.picked)
	_, cmd := h.m.Update(key("y"))
	if cmd == nil {
		t.Fatal("Update dropped the clipboard command")
	}
	want := tea.SetClipboard(expected)()
	matches := 0
	var inspect func(tea.Cmd)
	inspect = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, child := range batch {
				inspect(child)
			}
		} else if reflect.DeepEqual(msg, want) {
			matches++
		} else {
			t.Errorf("unexpected clipboard command message: %#v, want %#v", msg, want)
		}
	}
	inspect(cmd)
	if matches != 1 {
		t.Fatalf("got %d matching clipboard messages, want exactly one", matches)
	}
	if h.m.msg != fmt.Sprintf("copied %d rows", count) {
		t.Fatalf("copy status: %q", h.m.msg)
	}
	if h.m.tree.Gen() != gen || len(h.m.undo) != undo || len(h.m.redo) != redo || fmt.Sprint(h.m.picked) != picked {
		t.Fatal("copy changed the tree, history or selection")
	}
	// The command must be drained, not delivered again on the next event.
	if _, next := h.m.Update(tea.WindowSizeMsg{Width: h.m.width, Height: h.m.height}); next != nil {
		t.Fatal("clipboard command leaked into the next Update")
	}
}

func assertCopiedRows(t *testing.T, h *harness, dest string, want []copyRow) []string {
	t.Helper()
	var got []copyRow
	var ids []string
	var walk func(string, int)
	walk = func(parent string, index int) {
		for _, id := range h.m.tree.Children(parent) {
			row := h.m.tree.Row(id)
			if row.Parent != parent {
				t.Fatalf("row %s has parent %s, want %s", id, row.Parent, parent)
			}
			i := len(got)
			got = append(got, copyRow{index, sigil.FromFields(row.Fields)})
			ids = append(ids, id)
			walk(id, i)
		}
	}
	walk(dest, -1)
	if len(got) != len(want) {
		t.Fatalf("got %d stored rows, want %d", len(got), len(want))
	}
	for i, row := range got {
		if row.parent != want[i].parent || !reflect.DeepEqual(row.line.Fields(), want[i].line.Fields()) {
			t.Fatalf("stored row %d: got %+v, want %+v", i, row, want[i])
		}
	}
	return ids
}

func roundTripCopy(t *testing.T, parentState string, rows []copyRow, expected string) {
	t.Helper()
	for _, mode := range []string{"outline", "editor"} {
		t.Run(mode, func(t *testing.T) {
			h, ids, dest := copyFixture(t, parentState, rows)
			// Select every root when there is more than one.
			roots := 0
			for _, row := range rows {
				if row.parent < 0 {
					roots++
				}
			}
			if roots > 1 {
				for i, row := range rows {
					if row.parent < 0 {
						h.m.picked[ids[i]] = true
					}
				}
			}
			copyThroughUpdate(t, h, expected, len(rows))
			h.m.open(entry{kind: kindProject, id: dest})
			steps := len(h.m.undo)
			if mode == "editor" {
				h.press("o")
			}
			h.pasteText(expected)
			if len(rows) == 1 && mode == "editor" {
				if h.m.edit == nil {
					t.Fatal("single-line paste must stay in the editor")
				}
				if rows[0].line.Text == "" && string(h.m.edit.buf) != "$note" {
					t.Fatalf("empty note token lost: %q", h.m.edit.buf)
				}
				h.press("esc")
			} else if h.m.msg != fmt.Sprintf("pasted %d rows", len(rows)) {
				t.Fatalf("paste status: %q", h.m.msg)
			}
			pasted := assertCopiedRows(t, h, dest, rows)
			if len(h.m.undo) != steps+1 {
				t.Fatalf("paste made %d undo steps", len(h.m.undo)-steps)
			}
			// Re-editing exercises the same grammar without the transport layer.
			h.m.seek(pasted[0])
			h.press("e", "esc")
			assertCopiedRows(t, h, dest, rows)
			h.press("u")
			if len(h.m.tree.Children(dest)) != 0 || len(h.m.undo) != steps {
				t.Fatal("one undo did not remove the entire pasted outline")
			}
		})
	}
}

func TestCopyKeepsABlankTask(t *testing.T) {
	h := project(t)
	source := h.m.current()
	parent := h.m.tree.Row(source).Parent
	h.press("e", "ctrl+u", "esc")
	row := h.m.tree.Row(source)
	if row.Fields[sigil.FieldText] != "" || row.Fields[sigil.FieldState] != sigil.Open {
		t.Fatalf("clearing an existing task changed its state: %+v", row.Fields)
	}
	copyThroughUpdate(t, h, "$open\n", 1)
	h.pasteText("$open\n")
	pasted := h.m.current()
	row = h.m.tree.Row(pasted)
	if pasted == source || row.Parent != parent || row.Fields[sigil.FieldText] != "" || row.Fields[sigil.FieldState] != sigil.Open {
		t.Fatalf("pasted blank task lost its parent or state: %+v", row)
	}
	// A blank note still has an explicit representation that paste retains.
	h.press("$", `"note"`, "enter")
	copyThroughUpdate(t, h, "$note\n", 1)
	h.pasteText("$note\n")
	row = h.m.tree.Row(h.m.current())
	if h.m.current() == pasted || row.Parent != parent || row.Fields[sigil.FieldText] != "" || row.Fields[sigil.FieldState] != "" {
		t.Fatalf("pasted blank note lost its parent or state: %+v", row)
	}
}

func TestCopyRoundTripsThroughPaste(t *testing.T) {
	t.Run("task-subtree", func(t *testing.T) {
		rows := []copyRow{
			{-1, sigil.Line{Text: "Task", State: sigil.Open, Flag: true, Tags: []string{"work"}}},
			{0, sigil.Line{Text: "Subtask", State: sigil.Open, Due: "2026-11-15"}},
			{0, sigil.Line{Text: "Note"}},
		}
		text := "Task ! #work\n\tSubtask @2026-11-15 $open\n\tNote\n"
		h, _, dest := copyFixture(t, "", rows)
		copyThroughUpdate(t, h, text, 3)
		h.want("▌ ○ Task ! #work", "    ○ Subtask @today", "      Note")
		h.m.open(entry{kind: kindProject, id: dest})
		h.pasteText(text)
		h.want("  ○ Task ! #work", "    ○ Subtask @today", "▌     Note")
		assertCopiedRows(t, h, dest, rows)
	})
	t.Run("selection", func(t *testing.T) {
		h := project(t)
		h.press("k", "v", "v")
		copyThroughUpdate(t, h, "First\nSecond\n", 2)
	})
	t.Run("list", func(t *testing.T) {
		h := project(t)
		h.press("t", "p", `"Today"`, "enter")
		before := h.rows()
		copyThroughUpdate(t, h, "Second\n", 1)
		if h.rows() != before {
			t.Fatal("copy changed the list")
		}
	})
	t.Run("filter", func(t *testing.T) {
		h := project(t)
		h.press("/", `"Second"`, "enter", "G")
		copyThroughUpdate(t, h, "Second\n", 1)
		h.press("v")
		copyThroughUpdate(t, h, "Second\n", 1)
	})
	t.Run("hidden-and-selected-descendants", func(t *testing.T) {
		rows := []copyRow{{-1, sigil.Line{Text: "Task", State: sigil.Open}},
			{0, sigil.Line{Text: "Closed", State: sigil.Done}},
			{1, sigil.Line{Text: "Hidden"}},
			{0, sigil.Line{Text: "Deleted"}},
			{3, sigil.Line{Text: "Deleted child", State: sigil.Open}}}
		h, ids, _ := copyFixture(t, "", rows)
		if !h.m.do(outline.Op{Kind: outline.Set, Row: ids[3], Field: outline.FieldDeleted, Value: "1"}) {
			t.Fatal(h.m.msg)
		}
		h.m.picked[ids[0]], h.m.picked[ids[1]] = true, true
		h.press(":", `"closed"`, "enter", "h")
		copyThroughUpdate(t, h, "Task\n\tClosed $done\n\t\tHidden\n", 3)
	})
	t.Run("literal-prefixes", func(t *testing.T) {
		for i, prefix := range []string{"- ", "* ", "+ ", "• ", "[ ] ", "[x] ", "[X] ", `\`, `\\`} {
			t.Run(fmt.Sprintf("prefix-%d", i), func(t *testing.T) {
				rows := []copyRow{{-1, sigil.Line{Text: prefix + "child"}}}
				t.Run("single", func(t *testing.T) {
					roundTripCopy(t, sigil.Open, rows, `\`+prefix+"child\n")
				})
				// Multiline insertion also preserves sibling order and hierarchy.
				rows = append(rows, copyRow{0, sigil.Line{Text: "Grandchild", State: sigil.Open}},
					copyRow{-1, sigil.Line{Text: "Following"}})
				t.Run("subtree", func(t *testing.T) {
					roundTripCopy(t, sigil.Open, rows, `\`+prefix+"child\n\tGrandchild\nFollowing\n")
				})
			})
		}
	})
	t.Run("unescaped-checkbox", func(t *testing.T) {
		for _, mode := range []string{"outline", "editor"} {
			t.Run(mode, func(t *testing.T) {
				h, _, dest := copyFixture(t, sigil.Open, nil)
				h.m.open(entry{kind: kindProject, id: dest})
				if mode == "editor" {
					h.press("o")
				}
				h.pasteText("[ ] child")
				if mode == "editor" {
					h.press("esc")
				}
				assertCopiedRows(t, h, dest, []copyRow{{-1, sigil.Line{Text: "child", State: sigil.Open}}})
			})
		}
	})
	t.Run("empty-note", func(t *testing.T) {
		rows := []copyRow{{-1, sigil.Line{Text: "Task", State: sigil.Open}},
			{0, sigil.Line{}}, {1, sigil.Line{Text: "Child", State: sigil.Open}},
			{0, sigil.Line{Text: "Following"}}}
		t.Run("subtree", func(t *testing.T) {
			roundTripCopy(t, "", rows, "Task\n\t$note\n\t\tChild\n\tFollowing\n")
		})
		t.Run("alone", func(t *testing.T) {
			roundTripCopy(t, sigil.Open, []copyRow{{-1, sigil.Line{}}}, "$note\n")
		})
	})
	t.Run("backslash-before-sigil", func(t *testing.T) {
		for ti, token := range []string{"$open", "!", "@2026-11-15", "@@2026-11-15", "*2w", "#tag"} {
			for n := 0; n <= 2; n++ {
				for pi, prefix := range []string{"", "prefix "} {
					t.Run(fmt.Sprintf("sigil-%d/backslashes-%d/position-%d", ti, n, pi), func(t *testing.T) {
						line := sigil.Line{Text: prefix + strings.Repeat(`\`, n) + token}
						encoded := prefix + strings.Repeat(`\`, n+1) + token
						if prefix == "" {
							encoded = `\` + encoded
						}
						roundTripCopy(t, sigil.Open, []copyRow{{-1, line}}, encoded+"\n")
					})
				}
			}
		}
		line := sigil.Line{Text: `\plain prefix \#tag`, State: sigil.Doing, Flag: true,
			Due: "2026-11-15", Start: "2026-11-15", Every: "2w", Tags: []string{"real"}}
		roundTripCopy(t, sigil.Open, []copyRow{{-1, line}}, `\\plain prefix \\#tag ! @2026-11-15 @@2026-11-15 *2w #real $doing`+"\n")
	})
	t.Run("over-2000-rows", func(t *testing.T) {
		rows := []copyRow{{-1, sigil.Line{Text: "Task", State: sigil.Open}}}
		var text strings.Builder
		text.WriteString("Task\n")
		for i := 1; i <= 2001; i++ {
			line := sigil.Line{Text: fmt.Sprintf("Child %d", i), Flag: true, Tags: []string{"large"}}
			rows = append(rows, copyRow{0, line})
			fmt.Fprintf(&text, "\tChild %d ! #large\n", i)
		}
		roundTripCopy(t, "", rows, text.String())
	})
}
