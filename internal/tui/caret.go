package tui

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// Inside a row the caret sits on a character and the keys are a grammar over
// its text: motions, the operators d c y that take a motion or a text object,
// and a few acts on characters. It is the same open edit as typing, so
// whatever is done there, and typed after it, is one undo step.

// snapshot is a line as it stood before a change, for u inside the row.
type snapshot struct {
	buf   []rune
	caret int
}

// textAct is the last change made inside a row, for the dot key there: the
// keys that made it and, when they opened typing, what was typed.
type textAct struct {
	keys  []tea.KeyPressMsg
	typed string
}

// charFind is the last f, F, t or T, for ; and ,.
type charFind struct {
	key string
	ch  rune
}

// What a key did to the command being built.
const (
	textPending = iota // more keys are needed
	textMoved          // finished, and the line is as it was
	textChanged        // finished, and the line changed
	textTyping         // finished by opening typing at the caret
	textLeft           // the row was left
)

// wordClass sorts a character for the word motions: blank, word or other.
// A big word, as W B E read it, is anything not blank.
func wordClass(r rune, big bool) int {
	switch {
	case r == ' ':
		return 0
	case big || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
		return 1
	}
	return 2
}

// wordNext is where the next word starts after pos, or the end of the line.
func wordNext(buf []rune, pos int, big bool) int {
	i := max(pos, 0)
	if pos >= 0 && i < len(buf) {
		for c := wordClass(buf[i], big); c != 0 && i < len(buf) && wordClass(buf[i], big) == c; {
			i++
		}
	}
	for i < len(buf) && buf[i] == ' ' {
		i++
	}
	return i
}

// wordPrev is where the word before pos starts.
func wordPrev(buf []rune, pos int, big bool) int {
	i := min(pos, len(buf)) - 1
	for i >= 0 && buf[i] == ' ' {
		i--
	}
	if i < 0 {
		return 0
	}
	for c := wordClass(buf[i], big); i > 0 && wordClass(buf[i-1], big) == c; {
		i--
	}
	return i
}

// wordEnd is the last character of the first word ending after pos.
func wordEnd(buf []rune, pos int, big bool) int {
	i := pos + 1
	for i < len(buf) && buf[i] == ' ' {
		i++
	}
	if i >= len(buf) {
		return max(len(buf)-1, 0)
	}
	for c := wordClass(buf[i], big); i+1 < len(buf) && wordClass(buf[i+1], big) == c; {
		i++
	}
	return i
}

// enterText steps into the cursor row's text with a word motion, as if the
// caret had been waiting before the text, or after it for b.
func (m *Model) enterText(id, k string, count int) {
	m.editRow(id)
	e := m.edit
	e.normal = true
	big := k == "W" || k == "B" || k == "E"
	pos := -1
	if k == "b" || k == "B" {
		pos = len(e.buf)
	}
	for range max(count, 1) {
		switch k {
		case "w", "W":
			pos = wordNext(e.buf, pos, big)
		case "b", "B":
			pos = wordPrev(e.buf, pos, big)
		default:
			pos = wordEnd(e.buf, pos, big)
		}
	}
	e.caret = max(0, min(pos, len(e.buf)-1))
}

// textCount is the pending count inside the row: the one before an operator
// times the one before its motion.
func (e *editor) textCount() int {
	if e.opCount == 0 && e.count == 0 {
		return 0
	}
	a, b := max(e.opCount, 1), max(e.count, 1)
	if a > 1<<20/b {
		return 1 << 20
	}
	return a * b
}

// textMotion reads a motion key from the caret: where it goes, and whether
// an operator takes the character it lands on. arg is the character an f, F,
// t or T looks for.
func (m *Model) textMotion(e *editor, k string, arg rune, count int) (target int, inclusive, ok bool) {
	n, at := max(count, 1), e.caret
	big := k == "W" || k == "B" || k == "E"
	switch k {
	case "h", "left", "backspace":
		return max(at-n, 0), false, true
	case "l", "right", "space":
		return min(at+n, len(e.buf)), false, true
	case "0", "home", "^":
		return 0, false, true
	case "$", "end":
		return max(len(e.buf)-1, 0), true, true
	case "w", "W":
		for range n {
			at = wordNext(e.buf, at, big)
		}
		return at, false, true
	case "b", "B":
		for range n {
			at = wordPrev(e.buf, at, big)
		}
		return at, false, true
	case "e", "E":
		for range n {
			at = wordEnd(e.buf, at, big)
		}
		return at, true, true
	case ";", ",":
		// The last find again, or turned around.
		f := m.charFind
		if f.key == "" {
			return at, false, false
		}
		if k == "," {
			f.key = map[string]string{"f": "F", "F": "f", "t": "T", "T": "t"}[f.key]
		}
		return m.textFind(e, f, n)
	case "f", "F", "t", "T":
		m.charFind = charFind{k, arg}
		return m.textFind(e, m.charFind, n)
	}
	return at, false, false
}

// textFind is the nth character f.ch from the caret in f.key's direction; t
// and T stop one short of it.
func (m *Model) textFind(e *editor, f charFind, n int) (int, bool, bool) {
	step := 1
	if f.key == "F" || f.key == "T" {
		step = -1
	}
	for i := e.caret + step; i >= 0 && i < len(e.buf); i += step {
		if e.buf[i] != f.ch {
			continue
		}
		if n--; n == 0 {
			if f.key == "t" || f.key == "T" {
				i -= step
			}
			return i, step > 0, true
		}
	}
	return e.caret, false, false
}

var textPairs = map[string][2]rune{
	"(": {'(', ')'}, ")": {'(', ')'}, "b": {'(', ')'},
	"[": {'[', ']'}, "]": {'[', ']'},
	"{": {'{', '}'}, "}": {'{', '}'}, "B": {'{', '}'},
	"<": {'<', '>'}, ">": {'<', '>'},
}

// textObject is the span a text object names around the caret: a word, what
// is between quotes, or what is between brackets. inner leaves out the
// blanks after a word and the marks themselves.
func textObject(e *editor, k string, inner bool) (from, to int, ok bool) {
	if len(e.buf) == 0 {
		return 0, 0, false
	}
	at := min(e.caret, len(e.buf)-1)
	switch k {
	case "w", "W":
		big := k == "W"
		c := wordClass(e.buf[at], big)
		from, to = at, at+1
		for from > 0 && wordClass(e.buf[from-1], big) == c {
			from--
		}
		for to < len(e.buf) && wordClass(e.buf[to], big) == c {
			to++
		}
		if inner || c == 0 {
			return from, to, true
		}
		// The blanks after the word go with it, else the ones before.
		if to < len(e.buf) && e.buf[to] == ' ' {
			for to < len(e.buf) && e.buf[to] == ' ' {
				to++
			}
		} else {
			for from > 0 && e.buf[from-1] == ' ' {
				from--
			}
		}
		return from, to, true
	case `"`, "'", "`":
		// The pair the caret is in, else the first one after it.
		q := []rune(k)[0]
		var marks []int
		for i, r := range e.buf {
			if r == q {
				marks = append(marks, i)
			}
		}
		for i := 0; i+1 < len(marks); i += 2 {
			if marks[i+1] >= at {
				if inner {
					return marks[i] + 1, marks[i+1], true
				}
				return marks[i], marks[i+1] + 1, true
			}
		}
	default:
		pair, known := textPairs[k]
		if !known {
			return 0, 0, false
		}
		from, depth := at, 0
		for ; from >= 0; from-- {
			if e.buf[from] == pair[1] && from != at {
				depth++
			} else if e.buf[from] == pair[0] {
				if depth == 0 {
					break
				}
				depth--
			}
		}
		if from < 0 {
			return 0, 0, false
		}
		for to, depth = from+1, 0; to < len(e.buf); to++ {
			if e.buf[to] == pair[0] {
				depth++
			} else if e.buf[to] == pair[1] {
				if depth == 0 {
					if inner {
						return from + 1, to, true
					}
					return from, to + 1, true
				}
				depth--
			}
		}
	}
	return 0, 0, false
}

// keep remembers the line before a change, for u inside the row.
func (e *editor) keep() {
	e.past = append(e.past, snapshot{slices.Clone(e.buf), e.caret})
	e.ahead = nil
}

// cut removes a span of the line and returns it.
func (e *editor) cut(from, to int) string {
	text := string(e.buf[from:to])
	e.buf = append(e.buf[:from:from], e.buf[to:]...)
	e.caret = from
	return text
}

// rest puts the caret back on a character once a change is done.
func (e *editor) rest() {
	e.caret = max(0, min(e.caret, len(e.buf)-1))
}

// typeAt opens typing at a place in the line. The row is left with Esc as
// after any typing, and what is typed joins the change that opened it.
func (e *editor) typeAt(at int) int {
	e.caret = max(0, min(at, len(e.buf)))
	e.normal = false
	e.mark("text")
	return textTyping
}

// textKey handles a key while the caret is inside a row.
func (m *Model) textKey(msg tea.KeyPressMsg) {
	e := m.edit
	e.cmd = append(e.cmd, msg)
	switch m.textStep(e, msg) {
	case textPending:
		return
	case textChanged:
		e.rest()
		m.textRepeat = textAct{keys: e.cmd}
	case textTyping, textLeft:
		// Typing keeps the keys: what is typed completes the change.
		e.pend, e.count, e.opCount = "", 0, 0
		return
	}
	e.cmd, e.pend, e.count, e.opCount = nil, "", 0, 0
}

func (m *Model) textStep(e *editor, msg tea.KeyPressMsg) int {
	k := msg.String()
	if k == "esc" {
		// Esc drops what is pending, else leaves the row, keeping it.
		if e.pend != "" || e.count > 0 || e.opCount > 0 {
			return textMoved
		}
		m.finishEdit()
		return textLeft
	}

	// A key that waits for one character takes this one whole.
	if last := e.pend[max(len(e.pend)-1, 0):]; last != "" && strings.Contains("fFtTr", last) {
		arg := []rune(msg.Text)
		if len(arg) != 1 {
			return textMoved
		}
		if last == "r" {
			n := max(e.textCount(), 1)
			if e.caret+n > len(e.buf) {
				return textMoved
			}
			e.keep()
			for i := range n {
				e.buf[e.caret+i] = arg[0]
			}
			e.caret += n - 1
			return textChanged
		}
		return m.textGo(e, last, arg[0])
	}

	if len(k) == 1 && k[0] >= '0' && k[0] <= '9' && (k != "0" || e.count > 0) && len(e.pend) < 2 {
		e.count = min(e.count*10+int(k[0]-'0'), 1<<20)
		return textPending
	}

	// An operator is waiting for a motion or a text object.
	if e.pend != "" {
		if len(e.pend) == 2 {
			from, to, ok := textObject(e, k, e.pend[1] == 'i')
			if !ok {
				return textMoved
			}
			return m.textOperate(e, e.pend[:1], from, to)
		}
		switch k {
		case "i", "a":
			e.pend += k
			return textPending
		case "f", "F", "t", "T":
			e.pend += k
			return textPending
		}
		return m.textGo(e, k, 0)
	}

	n := max(e.textCount(), 1)
	switch k {
	case "d", "c", "y":
		e.pend, e.opCount, e.count = k, e.count, 0
		return textPending
	case "f", "F", "t", "T", "r":
		e.pend = k
		return textPending
	case "j", "down", "k", "up":
		// By drawn line within the row; the row itself is not left.
		by := 1
		if k == "k" || k == "up" {
			by = -1
		}
		l := e.wrapped(m.editWidth(), nil)
		p := l.pos[e.caret]
		if line := p.y + by; line >= 0 && line < len(l.lines) {
			e.caret = l.at(line, p.x)
			e.rest()
		}
		return textMoved
	case "x", "delete":
		return m.textOperate(e, "d", e.caret, min(e.caret+n, len(e.buf)))
	case "X":
		return m.textOperate(e, "d", max(e.caret-n, 0), e.caret)
	case "D":
		return m.textOperate(e, "d", e.caret, len(e.buf))
	case "C":
		return m.textOperate(e, "c", e.caret, len(e.buf))
	case "s":
		return m.textOperate(e, "c", e.caret, min(e.caret+n, len(e.buf)))
	case "~":
		if len(e.buf) == 0 {
			return textMoved
		}
		e.keep()
		for range min(n, len(e.buf)-e.caret) {
			r := e.buf[e.caret]
			if unicode.IsUpper(r) {
				e.buf[e.caret] = unicode.ToLower(r)
			} else {
				e.buf[e.caret] = unicode.ToUpper(r)
			}
			e.caret++
		}
		return textChanged
	case "i":
		return e.typeAt(e.caret)
	case "a":
		return e.typeAt(e.caret + 1)
	case "I":
		return e.typeAt(0)
	case "A":
		return e.typeAt(len(e.buf))
	case "p", "P":
		if m.textRegister == "" {
			m.msg = "nothing to put: y, d, c and x inside a row fill the text register"
			return textMoved
		}
		e.keep()
		at := e.caret
		if k == "p" && len(e.buf) > 0 {
			at++
		}
		e.caret = at
		e.insert([]rune(strings.Repeat(m.textRegister, n)))
		e.caret--
		return textChanged
	case "u":
		if len(e.past) == 0 {
			m.msg = "nothing to undo inside this row: Esc, then u"
			return textMoved
		}
		last := e.past[len(e.past)-1]
		e.past = e.past[:len(e.past)-1]
		e.ahead = append(e.ahead, snapshot{slices.Clone(e.buf), e.caret})
		e.buf, e.caret = last.buf, last.caret
		return textMoved
	case "ctrl+r":
		if len(e.ahead) == 0 {
			m.msg = "nothing to redo inside this row"
			return textMoved
		}
		next := e.ahead[len(e.ahead)-1]
		e.ahead = e.ahead[:len(e.ahead)-1]
		e.past = append(e.past, snapshot{slices.Clone(e.buf), e.caret})
		e.buf, e.caret = next.buf, next.caret
		return textMoved
	case ".":
		return m.textAgain(e)
	}
	return m.textGo(e, k, 0)
}

// textGo completes a motion: it moves the caret, or hands the pending
// operator the text from the caret to where the motion goes.
func (m *Model) textGo(e *editor, k string, arg rune) int {
	op := e.pend[:min(len(e.pend), 1)]
	if !strings.Contains("dcy", op) {
		op = ""
	}
	// Changing a word leaves the blank after it, as ce does.
	if op == "c" && (k == "w" || k == "W") && e.caret < len(e.buf) && e.buf[e.caret] != ' ' {
		k = map[string]string{"w": "e", "W": "E"}[k]
	}
	target, inclusive, ok := m.textMotion(e, k, arg, e.textCount())
	if !ok {
		return textMoved
	}
	if op == "" {
		e.caret = target
		e.rest()
		return textMoved
	}
	from, to := e.caret, target
	if from > to {
		from, to = to, from
	} else if inclusive {
		to++
	}
	return m.textOperate(e, op, from, min(to, len(e.buf)))
}

// textOperate applies an operator to a span of the line. What it takes goes
// to the text register, and a copy to the system clipboard as well.
func (m *Model) textOperate(e *editor, op string, from, to int) int {
	if from >= to && op != "c" {
		return textMoved
	}
	if op == "y" {
		m.textRegister = string(e.buf[from:to])
		m.pendingCmd = tea.SetClipboard(m.textRegister)
		e.caret = from
		e.rest()
		return textMoved
	}
	e.keep()
	if from < to {
		m.textRegister = e.cut(from, to)
	}
	if op == "c" {
		return e.typeAt(from)
	}
	return textChanged
}

// textAgain makes the last change inside a row again, at the caret. When
// that change typed something, it is typed again and the caret stays in the
// row.
func (m *Model) textAgain(e *editor) int {
	act := m.textRepeat
	if len(act.keys) == 0 {
		m.msg = "nothing to repeat inside a row yet"
		return textMoved
	}
	// One step back for u, however many the keys take.
	e.pend, e.count, e.opCount = "", 0, 0
	base, before := len(e.past), snapshot{slices.Clone(e.buf), e.caret}
	for _, key := range act.keys {
		switch m.textStep(e, key) {
		case textLeft:
			return textLeft
		case textTyping:
			e.insert([]rune(act.typed))
			e.normal, e.how = true, ""
			e.caret--
		case textChanged, textMoved:
			e.pend, e.count, e.opCount = "", 0, 0
		}
	}
	e.rest()
	e.past, e.ahead = append(e.past[:base], before), nil
	return textMoved
}

// textStatus is the status line while the caret is inside a row.
func (e *editor) textStatus() string {
	if e.pend == "" && e.count == 0 && e.opCount == 0 {
		return "in the row: w b e f move · i a c d x change · Esc done"
	}
	left := ""
	if e.opCount > 0 {
		left = strconv.Itoa(e.opCount)
	}
	left += e.pend
	if e.count > 0 {
		left += strconv.Itoa(e.count)
	}
	return left
}
