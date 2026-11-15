package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

const pasteLimit = 2000

// pasted is one line of pasted text: how deep it sits below the first line,
// and the row line it stands for.
type pasted struct {
	level int
	text  string
}

// readPaste reads pasted text as an outline. Depth follows indentation,
// whatever its unit; list bullets are dropped, and a ticked checkbox closes
// its row.
func readPaste(content string) []pasted {
	var out []pasted
	var cols []int // the indentation of each open level
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r", ""), "\n") {
		text := strings.TrimLeft(raw, " \t")
		if strings.TrimSpace(text) == "" || len(out) == pasteLimit {
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

		for _, bullet := range []string{"- ", "* ", "+ ", "• "} {
			if rest, ok := strings.CutPrefix(text, bullet); ok {
				text = rest
				break
			}
		}
		done := false
		if rest, ok := strings.CutPrefix(text, "[ ] "); ok {
			text = rest
		} else if len(text) > 3 && (strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ")) {
			text, done = text[4:], true
		}
		text = strings.Join(strings.Fields(text), " ")
		if done {
			text += " $done"
		}
		out = append(out, pasted{len(cols) - 1, text})
	}
	return out
}

// paste puts pasted text where the typing is. One line is typed text; more
// become rows, and the whole paste is one undo step.
func (m *Model) paste(content string) {
	lines := readPaste(content)
	oneLine := func() []rune {
		if len(lines) == 1 {
			// A single line is typed as it was copied, spaces and all.
			return []rune(strings.ReplaceAll(strings.Trim(content, "\r\n"), "\t", " "))
		}
		var words []string
		for _, l := range lines {
			words = append(words, l.text)
		}
		return []rune(strings.Join(words, " "))
	}
	switch {
	case len(lines) == 0:
	case m.ask != nil:
		m.ask.ed.insert(oneLine())
		if m.ask.kind == askFilter {
			m.at.query = strings.TrimSpace(string(m.ask.ed.buf))
			m.refilter()
			m.cursor, m.top = 0, 0
			m.picked = map[string]bool{}
		}
	case m.edit != nil && len(lines) == 1:
		m.edit.insert(oneLine())
	case m.edit != nil:
		// The first line joins the row being typed; the rest follow it.
		e := m.edit
		e.insert([]rune(lines[0].text))
		steps := len(m.undo)
		if m.finishEdit() {
			m.pasteRows(e.row, true, lines[1:], len(m.undo) > steps)
		}
	case m.sideFocus || len(m.picked) > 0 || m.flat():
		if m.msg == "" {
			m.msg = "paste into an outline, with nothing selected"
		}
	default:
		m.pasteRows(m.current(), false, lines, false)
	}
}

// pasteRows adds lines as rows after anchor. When the anchor is the paste's
// own first line, deeper lines become its children. With merge, the rows
// join the undo step just recorded.
func (m *Model) pasteRows(anchor string, anchorIsFirst bool, lines []pasted, merge bool) {
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
		case after == "":
			pos = m.tree.PosLast(parent0)
		default:
			pos = m.tree.PosAfter(parent0, after)
		}
		op := outline.Op{Kind: outline.Insert, Row: m.newID(), Parent: parent, Pos: pos}
		undo, err := m.apply([]outline.Op{op})
		if err != nil {
			fail(err)
			return
		}
		made = append(made, op.Row)
		fields := m.setFields(op.Row, sigil.Parse(l.text, parent == "", today).Fields())
		more, err := m.apply(fields)
		if err != nil {
			fail(err)
			return
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
		return
	}
	if merge {
		if !m.save(ops, inv) {
			for _, id := range made {
				m.tree.Forget(id)
			}
			return
		}
		last := len(m.undo) - 1
		m.undo[last] = append(inv, m.undo[last]...)
		m.keepHistory()
	} else if !m.record(ops, inv) {
		for _, id := range made {
			m.tree.Forget(id)
		}
		return
	}
	m.rebuild()
	m.seek(made[len(made)-1])
	m.msg = fmt.Sprintf("pasted %d rows", len(made))
}
