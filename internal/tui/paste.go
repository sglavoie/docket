package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

// pasted is one line of pasted text: how deep it sits below the first line,
// and the row line it stands for.
type pasted struct {
	level     int
	text      string
	checkbox  bool // normalize this line even when pasted into a single-line editor
	protected bool // transport-decoded; never normalize its literal prefixes
}

// readPaste reads pasted text as an outline. Depth follows indentation,
// whatever its unit; list bullets are dropped, and checkboxes name the state.
func readPaste(content string) []pasted {
	var out []pasted
	var cols []int // the indentation of each open level
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r", ""), "\n") {
		text := strings.TrimLeft(raw, " \t")
		if strings.TrimSpace(text) == "" {
			continue
		}
		indent := 0
		for _, c := range raw[:len(raw)-len(text)] {
			if c == '\t' {
				indent += 4
			} else {
				indent++
			}
		}
		for len(cols) > 1 && cols[len(cols)-1] > indent {
			cols = cols[:len(cols)-1]
		}
		if len(cols) == 0 || indent > cols[len(cols)-1] {
			cols = append(cols, indent)
		}

		protected := strings.HasPrefix(text, `\`) && pastePrefix(text[1:])
		state := ""
		if protected {
			text = text[1:]
		} else {
			for _, bullet := range []string{"- ", "* ", "+ ", "• "} {
				if rest, ok := strings.CutPrefix(text, bullet); ok {
					text = rest
					break
				}
			}
			if rest, ok := strings.CutPrefix(text, "[ ] "); ok {
				text, state = rest, sigil.Open
			} else if len(text) > 3 && (strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ")) {
				text, state = text[4:], sigil.Done
			}
		}
		text = strings.Join(strings.Fields(text), " ")
		if state != "" {
			text += " $" + state
		}
		out = append(out, pasted{level: len(cols) - 1, text: text, checkbox: state != "", protected: protected})
	}
	return out
}

// paste puts pasted text where the typing is. One line stays in the editor,
// with checkboxes normalized; more become rows in one undo step.
func (m *Model) paste(content string) {
	lines := readPaste(content)
	oneLine := func() []rune {
		if len(lines) == 1 {
			// A single line is typed as it was copied, spaces and all.
			return []rune(strings.ReplaceAll(strings.Trim(content, "\r\n"), "\t", " "))
		}
		return []rune(strings.Join(strings.Fields(content), " "))
	}
	switch {
	case len(lines) == 0:
	case m.ask != nil:
		m.ask.ed.insert(oneLine())
		if m.ask.kind == askFilter {
			m.at.query = strings.TrimSpace(string(m.ask.ed.buf))
			m.refilter()
			m.cursor, m.top = 0, 0
			m.clearSelection()
		}
	case m.edit != nil && len(lines) == 1:
		if lines[0].checkbox || lines[0].protected {
			m.edit.insert([]rune(lines[0].text))
		} else {
			m.edit.insert(oneLine())
		}
	case m.edit != nil:
		// The first line joins the row being typed; the rest follow it.
		e := m.edit
		e.insert([]rune(lines[0].text))
		if m.finishEdit() {
			m.pasteRows(e.row, true, lines[1:], e.merge, false)
		}
	case m.sideFocus || len(m.picked) > 0 || m.flat():
		if m.msg == "" {
			m.msg = "paste into an outline, with nothing selected"
		}
	default:
		m.pasteRows(m.current(), false, lines, false, false)
	}
}

// put reads the session register through the same parser and builder as paste.
func (m *Model) put(before bool) {
	if m.sideFocus {
		return
	}
	if len(m.picked) > 0 || m.flat() {
		if m.msg == "" {
			m.msg = "put into an outline, with nothing selected"
		}
		return
	}
	lines := readPaste(m.register)
	if len(lines) == 0 {
		m.msg = "nothing to put"
		return
	}
	if n := m.pasteRows(m.current(), false, lines, false, before); n > 0 {
		m.msg = fmt.Sprintf("put %d rows", n)
	}
}

// pasteRows adds lines beside anchor, before it when before is set. When
// anchorIsFirst, deeper lines become its children. With merge, the rows
// join the undo step just recorded. It returns the number of pasted rows.
func (m *Model) pasteRows(anchor string, anchorIsFirst bool, lines []pasted, merge, before bool) int {
	parent0, after := m.at.root, anchor
	if anchor != "" {
		parent0 = m.tree.Row(anchor).Parent
	}
	var stack []string // the newest row at each level
	if anchorIsFirst {
		stack = []string{anchor}
	}
	var ops, inv []outline.Op
	var made []string
	fail := func(err error) {
		now := m.now()
		for _, u := range inv {
			m.tree.Apply(u, now)
		}
		for _, id := range made {
			m.tree.Forget(id)
		}
		m.msg = err.Error()
	}
	today := m.today()
	for _, l := range lines {
		// Never deeper than one below the row above.
		level := min(l.level, len(stack))
		parent, pos := parent0, ""
		switch {
		case level > 0:
			parent = stack[level-1]
			pos = m.tree.PosLast(parent)
		case before && after == anchor && anchor != "":
			pos = m.tree.PosBefore(parent0, anchor)
		case after == "":
			pos = m.tree.PosLast(parent0)
		default:
			pos = m.tree.PosAfter(parent0, after)
		}
		op := outline.Op{Kind: outline.Insert, Row: m.newID(), Parent: parent, Pos: pos}
		undo, err := m.apply([]outline.Op{op})
		if err != nil {
			fail(err)
			return 0
		}
		made = append(made, op.Row)
		fields := m.setFields(op.Row, sigil.Parse(l.text, m.defaultsToNotes(parent), today).Fields())
		more, err := m.apply(fields)
		if err != nil {
			fail(err)
			return 0
		}
		ops = append(append(ops, op), fields...)
		inv = append(append(more, undo...), inv...)
		stack = append(stack[:level], op.Row)
		if level == 0 {
			after = op.Row
		} else {
			m.setFold(parent, false)
		}
	}
	if len(ops) == 0 {
		return 0
	}
	if merge {
		if !m.save(ops, inv) {
			for _, id := range made {
				m.tree.Forget(id)
			}
			return 0
		}
		last := len(m.undo) - 1
		m.undo[last] = append(inv, m.undo[last]...)
		m.keepHistory()
	} else if !m.record(ops, inv) {
		for _, id := range made {
			m.tree.Forget(id)
		}
		return 0
	}
	// Newly made closed rows stay visible for this visit, so the cursor can
	// land on the last row even when closed rows are normally hidden.
	if m.hideClosed {
		for _, id := range made {
			if sigil.FromFields(m.tree.Row(id).Fields).Closed() {
				if m.at.kept == nil {
					m.at.kept = map[string]bool{}
				}
				m.at.kept[id] = true
			}
		}
	}
	m.rebuild()
	m.seek(made[len(made)-1])
	n := len(made)
	if anchorIsFirst {
		n++
	}
	m.msg = fmt.Sprintf("pasted %d rows", n)
	return n
}

// pastePrefix identifies line starts needing protection after sigil.Format.
// Decoding this transport layer must leave sigil escapes for sigil.Parse.
func pastePrefix(text string) bool {
	for _, prefix := range []string{"- ", "* ", "+ ", "• ", "[ ] ", "[x] ", "[X] ", `\`} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}
