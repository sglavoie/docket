package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/outline"
)

// A row's body is the text that does not fit on its line. It is one field of
// plain text, read and written at the head of the pane that opens the row,
// where the in-row grammar edits it by lines as well as characters. Nothing
// in it is read as a sigil.

const fieldBody = "body"

// bodyRow is the row whose body heads the pane, or nothing: the pane shows
// the rows under one row, unfiltered.
func (m *Model) bodyRow() string {
	if m.at.list != "" || m.at.query != "" || m.at.root == "" || !m.live(m.at.root) {
		return ""
	}
	return m.at.root
}

// enterBody steps into the body of the row the pane opens, on its last line.
func (m *Model) enterBody() {
	id := m.bodyRow()
	buf := []rune(m.tree.Row(id).Fields[fieldBody])
	e := &editor{row: id, buf: buf, normal: true, body: true}
	e.caret = lineStart(buf, len(buf))
	e.rest()
	m.edit = e
}

// finishBody keeps what was written in the body, as one undo step. Blanks at
// its end are dropped, so a body of blanks is no body.
func (m *Model) finishBody(e *editor) bool {
	text := strings.TrimRight(string(e.buf), " \t\n")
	if !m.live(e.row) {
		return false
	}
	if text == m.tree.Row(e.row).Fields[fieldBody] {
		return true
	}
	if !m.do(outline.Op{Kind: outline.Set, Row: e.row, Field: fieldBody, Value: text}) {
		return false
	}
	m.rebuild()
	return true
}

// bodyType is a key while typing in the body: Enter is a new line, Tab a
// tab, and Up and Down stay in the body.
func (m *Model) bodyType(msg tea.KeyPressMsg) {
	e := m.edit
	if msg.String() != "up" && msg.String() != "down" {
		e.vertical = false
	}
	switch msg.String() {
	case "enter":
		e.insert([]rune{'\n'})
	case "tab":
		e.insert([]rune{'\t'})
	case "shift+tab":
	case "esc":
		m.textBack(e)
	case "up", "down":
		by := 1
		if msg.String() == "up" {
			by = -1
		}
		l := e.wrapped(m.editWidth(), nil)
		p := l.pos[e.caret]
		if !e.vertical {
			e.column = p.x
		}
		e.vertical = true
		if line := p.y + by; line >= 0 && line < len(l.lines) {
			e.caret = l.at(line, e.column)
		}
	default:
		e.key(msg)
	}
}

// isBlank is a character the word motions pass over.
func isBlank(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }

// lineStart is where the line holding at begins.
func lineStart(buf []rune, at int) int {
	at = min(at, len(buf))
	for at > 0 && buf[at-1] != '\n' {
		at--
	}
	return at
}

// lineEnd is the newline that ends the line holding at, or the end.
func lineEnd(buf []rune, at int) int {
	for at < len(buf) && buf[at] != '\n' {
		at++
	}
	return at
}

// firstChar is the first character of the line holding at that is not blank.
func firstChar(buf []rune, at int) int {
	i, end := lineStart(buf, at), lineEnd(buf, at)
	for i < end && (buf[i] == ' ' || buf[i] == '\t') {
		i++
	}
	return i
}

// emptyLine reports whether the line starting at holds nothing.
func emptyLine(buf []rune, start int) bool {
	return start >= len(buf) || buf[start] == '\n'
}

// lineStarts are where each line of a body begins.
func lineStarts(buf []rune) []int {
	starts := []int{0}
	for i, r := range buf {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineOf is the index of the line holding at.
func lineOf(starts []int, at int) int {
	n := 0
	for n+1 < len(starts) && starts[n+1] <= at {
		n++
	}
	return n
}

// paragraphMove is the next or previous empty line past the paragraph the
// caret is in, else the end or the start.
func paragraphMove(buf []rune, at int, forward bool, n int) int {
	starts := lineStarts(buf)
	empty := func(l int) bool { return emptyLine(buf, starts[l]) }
	l := lineOf(starts, at)
	for range n {
		if forward {
			for l+1 < len(starts) && empty(l+1) {
				l++
			}
			for l+1 < len(starts) && !empty(l+1) {
				l++
			}
			if l+1 >= len(starts) {
				return max(len(buf)-1, 0)
			}
			l++
			continue
		}
		for l > 0 && empty(l-1) {
			l--
		}
		for l > 0 && !empty(l-1) {
			l--
		}
		if l == 0 {
			return 0
		}
		l--
	}
	return starts[l]
}

// sentenceStarts are where the sentences of a body begin: its start, the
// first character after a full stop, question or exclamation mark and the
// blanks after it, and the first character of a paragraph.
func sentenceStarts(buf []rune) []int {
	starts := []int{0}
	for i := 1; i < len(buf); i++ {
		if isBlank(buf[i]) {
			continue
		}
		j := i - 1
		for j >= 0 && isBlank(buf[j]) {
			j--
		}
		if j == i-1 {
			continue // inside a word
		}
		gap := buf[j+1 : i]
		blankLine := strings.Count(string(gap), "\n") > 1
		if j < 0 || blankLine || strings.ContainsRune(".!?", buf[j]) {
			starts = append(starts, i)
		}
	}
	return starts
}

// sentenceMove is the start of the next or previous sentence.
func sentenceMove(buf []rune, at int, forward bool, n int) int {
	starts := sentenceStarts(buf)
	for range n {
		if forward {
			next := max(len(buf)-1, 0)
			for _, s := range starts {
				if s > at {
					next = s
					break
				}
			}
			at = next
			continue
		}
		prev := 0
		for _, s := range starts {
			if s >= at {
				break
			}
			prev = s
		}
		at = prev
	}
	return at
}

// lineSpan is the whole lines from the one holding a to the one holding b,
// with the newline that ends them, or the one before the last line.
func lineSpan(buf []rune, a, b int) (from, to int) {
	from, to = lineStart(buf, min(a, b)), lineEnd(buf, max(a, b))
	if to < len(buf) {
		return from, to + 1
	}
	if from > 0 {
		return from - 1, to
	}
	return from, to
}

// linesDown is a place on the line n-1 lines below the one holding at.
func linesDown(buf []rune, at, n int) int {
	for range n - 1 {
		end := lineEnd(buf, at)
		if end >= len(buf) {
			break
		}
		at = end + 1
	}
	return at
}

// bodyLines acts on whole lines: d deletes them, c empties them and types
// there, and y copies them, newline and all, so p puts them as lines.
func (m *Model) bodyLines(e *editor, op string, a, b int) int {
	from, to := lineSpan(e.buf, a, b)
	if op == "y" {
		text := string(e.buf[lineStart(e.buf, min(a, b)):lineEnd(e.buf, max(a, b))]) + "\n"
		m.textRegister = text
		e.flash = [2]int{from, to}
		m.pendingCmd = tea.Batch(tea.SetClipboard(text), m.startFlash())
		e.caret = lineStart(e.buf, min(a, b))
		return textMoved
	}
	e.keep()
	m.textRegister = string(e.buf[lineStart(e.buf, min(a, b)):lineEnd(e.buf, max(a, b))]) + "\n"
	if op == "c" {
		start := lineStart(e.buf, min(a, b))
		e.cut(start, lineEnd(e.buf, max(a, b)))
		return e.typeAt(start)
	}
	e.cut(from, to)
	e.caret = firstChar(e.buf, min(from, len(e.buf)))
	return textChanged
}

// bodyPut puts lines the register holds below or above the caret's line.
func (e *editor) bodyPut(text string, below bool) {
	text = strings.TrimSuffix(text, "\n")
	at := lineStart(e.buf, e.caret)
	if below {
		end := lineEnd(e.buf, e.caret)
		if end >= len(e.buf) {
			e.caret = end
			e.insert([]rune("\n" + text))
			e.caret = end + 1
			return
		}
		at = end + 1
	}
	e.caret = at
	e.insert([]rune(text + "\n"))
	e.caret = at
}

// bodyJoin joins the next line onto the caret's with one blank.
func (e *editor) bodyJoin() bool {
	end := lineEnd(e.buf, e.caret)
	if end >= len(e.buf) {
		return false
	}
	next := end + 1
	for next < len(e.buf) && (e.buf[next] == ' ' || e.buf[next] == '\t') {
		next++
	}
	glue := " "
	if end == lineStart(e.buf, end) || next >= len(e.buf) || e.buf[next] == '\n' {
		glue = ""
	}
	e.keep()
	e.cut(end, next)
	e.caret = end
	e.insert([]rune(glue))
	e.caret = end
	return true
}

// paragraph is the span a paragraph object names: the run of lines holding
// text around the caret, or of empty lines, whole, with the empty lines
// after it for ap.
func paragraph(buf []rune, at int, inner bool) (from, to int) {
	starts := lineStarts(buf)
	empty := func(l int) bool { return emptyLine(buf, starts[l]) }
	l := lineOf(starts, at)
	first, last := l, l
	for first > 0 && empty(first-1) == empty(l) {
		first--
	}
	for last+1 < len(starts) && empty(last+1) == empty(l) {
		last++
	}
	if !inner && !empty(l) {
		for last+1 < len(starts) && empty(last+1) {
			last++
		}
	}
	return lineSpan(buf, starts[first], starts[last])
}

// bodyCache keeps the last body drawn, wrapped, so moving among the rows
// does not wrap it again.
type bodyCache struct {
	text  string
	width int
	lines []string
}

func (m *Model) bodyWrapped(text string, width int) []string {
	if c := m.bodyDrawn; c.text == text && c.width == width && c.lines != nil {
		return c.lines
	}
	lines := (&editor{buf: []rune(text)}).wrapped(width, nil).lines
	m.bodyDrawn = bodyCache{text, width, lines}
	return lines
}

// bodyWidth is the room a body has: the pane's width after the columns
// before a top-level row's text.
func (m *Model) bodyWidth(paneW int) int {
	return max(paneW-m.numberCols()-4, 1)
}

// linesUp is a place on the line n lines above the one holding at.
func linesUp(buf []rune, at, n int) int {
	at = lineStart(buf, at)
	for range n {
		if at == 0 {
			break
		}
		at = lineStart(buf, at-1)
	}
	return at
}

// bodyMotion reads the motions that differ in a body: they keep to the
// caret's line, or move by paragraph, sentence or to either end.
func bodyMotion(e *editor, k string, n int) (target int, inclusive, done bool) {
	at, buf := e.caret, e.buf
	start, end := lineStart(buf, at), lineEnd(buf, at)
	switch k {
	case "h", "left", "backspace":
		return max(at-n, start), false, true
	case "l", "right", "space":
		return min(at+n, end), false, true
	case "0", "home":
		return start, false, true
	case "^":
		return firstChar(buf, at), false, true
	case "$", "end":
		if end == start {
			return start, false, true
		}
		return end - 1, true, true
	case "}", "{":
		return paragraphMove(buf, at, k == "}", n), false, true
	case ")", "(":
		return sentenceMove(buf, at, k == ")", n), false, true
	case "gg":
		return firstChar(buf, 0), false, true
	case "G":
		return firstChar(buf, len(buf)), false, true
	}
	return at, false, false
}

// bodyStep is a key that does in a body what it cannot do in a row, or does
// by lines what a row does to its end.
func (m *Model) bodyStep(e *editor, k string, n int) (int, bool) {
	switch k {
	case "g":
		e.pend += "g"
		return textPending, true
	case "j", "down", "k", "up":
		// By drawn line, staying in the body; past its last line the caret
		// goes back to the rows.
		by := 1
		if k == "k" || k == "up" {
			by = -1
		}
		l := e.wrapped(m.editWidth(), nil)
		for range n {
			p := l.pos[e.caret]
			line := p.y + by
			if line < 0 {
				break
			}
			if line >= len(l.lines) {
				if e.sel || len(m.vis) == 0 {
					break
				}
				m.finishEdit()
				return textLeft, true
			}
			e.caret = l.at(line, p.x)
			e.rest()
		}
		return textMoved, true
	case "o", "O":
		e.keep()
		at := lineStart(e.buf, e.caret)
		if k == "o" {
			at = lineEnd(e.buf, e.caret)
		}
		e.caret = at
		e.insert([]rune{'\n'})
		if k == "O" {
			return e.typeAt(at), true
		}
		return e.typeAt(at + 1), true
	case "J":
		for range max(n-1, 1) {
			if !e.bodyJoin() {
				break
			}
		}
		return textChanged, true
	case "x", "delete":
		return m.textOperate(e, "d", e.caret, min(e.caret+n, lineEnd(e.buf, e.caret))), true
	case "X":
		return m.textOperate(e, "d", max(e.caret-n, lineStart(e.buf, e.caret)), e.caret), true
	case "D":
		return m.textOperate(e, "d", e.caret, lineEnd(e.buf, e.caret)), true
	case "C":
		return m.textOperate(e, "c", e.caret, lineEnd(e.buf, e.caret)), true
	case "I":
		e.keep()
		return e.typeAt(firstChar(e.buf, e.caret)), true
	case "A":
		e.keep()
		return e.typeAt(lineEnd(e.buf, e.caret)), true
	case "p", "P":
		if !strings.HasSuffix(m.textRegister, "\n") {
			return 0, false
		}
		e.keep()
		for range n {
			e.bodyPut(m.textRegister, k == "p")
		}
		e.caret = firstChar(e.buf, e.caret)
		return textChanged, true
	case "*":
		m.msg = "a body filters nothing: Esc, then / filters the rows"
		return textMoved, true
	}
	return 0, false
}
