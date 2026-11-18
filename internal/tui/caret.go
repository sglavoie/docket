package tui

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/sigil"
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
	case isBlank(r):
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
	for i < len(buf) && isBlank(buf[i]) {
		i++
	}
	return i
}

// wordPrev is where the word before pos starts.
func wordPrev(buf []rune, pos int, big bool) int {
	i := min(pos, len(buf)) - 1
	for i >= 0 && isBlank(buf[i]) {
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
	for i < len(buf) && isBlank(buf[i]) {
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

// wordEndBack is the last character of the word before the one at pos, as
// Vim's ge reads it.
func wordEndBack(buf []rune, pos int, big bool) int {
	i := min(pos, len(buf)-1)
	if i < 0 {
		return 0
	}
	if c := wordClass(buf[i], big); c != 0 {
		for i >= 0 && wordClass(buf[i], big) == c {
			i--
		}
	}
	for i >= 0 && isBlank(buf[i]) {
		i--
	}
	return max(i, 0)
}

// bracketMatch is the bracket that pairs with the first one at or after at,
// before end, as Vim's % finds it; ok is false when there is none.
func bracketMatch(buf []rune, at, end int) (int, bool) {
	pairs := map[rune]rune{'(': ')', '[': ']', '{': '}', ')': '(', ']': '[', '}': '{'}
	i := at
	for i < end && pairs[buf[i]] == 0 {
		i++
	}
	if i >= end {
		return at, false
	}
	open, shut := buf[i], pairs[buf[i]]
	step := 1
	if strings.ContainsRune(")]}", open) {
		step = -1
	}
	for j, depth := i+step, 0; j >= 0 && j < len(buf); j += step {
		switch buf[j] {
		case open:
			depth++
		case shut:
			if depth == 0 {
				return j, true
			}
			depth--
		}
	}
	return at, false
}

// isCaseOp names the operators that change case inside a row: gu, gU and
// g~, held pending by their second key.
func isCaseOp(op string) bool { return op == "u" || op == "U" || op == "~" }

// recase changes the case of a span: u lowers it, U raises it, ~ switches it.
func (e *editor) recase(op string, from, to int) {
	for i := from; i < to && i < len(e.buf); i++ {
		r := e.buf[i]
		switch {
		case op == "u" || (op == "~" && unicode.IsUpper(r)):
			e.buf[i] = unicode.ToLower(r)
		default:
			e.buf[i] = unicode.ToUpper(r)
		}
	}
}

// bump adds by to the number at or after the caret, as Vim's Ctrl+A and
// Ctrl+X. A minus is its sign only where it starts a word, so the parts of
// 2026-11-09 count as three numbers; leading zeros keep their width.
func (e *editor) bump(by int) bool {
	buf, end := e.buf, len(e.buf)
	if e.body {
		end = lineEnd(buf, e.caret)
	}
	digit := func(i int) bool { return i >= 0 && i < len(buf) && buf[i] >= '0' && buf[i] <= '9' }
	i := e.caret
	if digit(i) {
		for digit(i - 1) {
			i--
		}
	} else {
		for i < end && !digit(i) {
			i++
		}
		if i >= end {
			return false
		}
	}
	j := i
	for digit(j) {
		j++
	}
	digits := string(buf[i:j])
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return false
	}
	start := i
	if i > 0 && buf[i-1] == '-' && (i == 1 || wordClass(buf[i-2], false) != 1) {
		start, n = i-1, -n
	}
	n += int64(by)
	text := strconv.FormatInt(max(n, -n), 10)
	if len(digits) > 1 && digits[0] == '0' && len(text) < len(digits) {
		text = strings.Repeat("0", len(digits)-len(text)) + text
	}
	if n < 0 {
		text = "-" + text
	}
	e.keep()
	e.buf = append(append(slices.Clone(buf[:start]), []rune(text)...), buf[j:]...)
	e.caret = start + len([]rune(text)) - 1
	return true
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

// findMarks are the words, phrases and tags n looks for: those of the filter
// on screen, else of the one last kept or dropped.
func (m *Model) findMarks() []cond {
	q, here := m.at.query, m.at.here
	if q == "" {
		q, here = m.find, m.findHere
	}
	return m.query(q, here, m.today()).marks()
}

// rowLine is a row's line as stepping into it shows it.
func (m *Model) rowLine(id string) []rune {
	row := m.tree.Row(id)
	return []rune(sigil.Format(sigil.FromFields(row.Fields), m.defaultsToNotes(row.Parent), m.today()))
}

// textHits is where in a line the marks are found, in order: the start of
// each place a looked-for word or phrase stands, ignoring case, and of each
// tag the filter names.
func textHits(buf []rune, marks []cond) []int {
	lower := make([]rune, len(buf))
	for i, r := range buf {
		lower[i] = unicode.ToLower(r)
	}
	var hits []int
	for _, c := range marks {
		want := []rune(c.value)
		if c.kind == '#' {
			want = []rune("#" + c.value)
		}
		if c.kind == 'c' {
			want = []rune("##" + c.value)
		}
		for i := 0; len(want) > 0 && i+len(want) <= len(lower); i++ {
			if !slices.Equal(lower[i:i+len(want)], want) {
				continue
			}
			if c.kind == '#' || c.kind == 'c' {
				// A tag is a whole word, with or without a value.
				end := i + len(want)
				if (i > 0 && lower[i-1] != ' ') || (end < len(lower) && lower[end] != ' ' && lower[end] != '=') {
					continue
				}
			}
			hits = append(hits, i)
		}
	}
	slices.Sort(hits)
	return slices.Compact(hits)
}

// hitFrom is the first hit after the caret, or the last one before it.
func hitFrom(hits []int, caret int, forward bool) (int, bool) {
	if forward {
		for _, h := range hits {
			if h > caret {
				return h, true
			}
		}
		return 0, false
	}
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i] < caret {
			return hits[i], true
		}
	}
	return 0, false
}

// textMove keeps the row the caret is in and puts the caret inside another
// shown row. Each row is its own undo step.
func (m *Model) textMove(target string) bool {
	if !m.finishEdit() {
		return false
	}
	m.seek(target)
	if m.current() != target {
		return false
	}
	m.editRow(target)
	m.edit.normal = true
	m.edit.caret = 0
	return true
}

// matchRow is the nearest shown row from the cursor's, in a direction, that
// the filter matches and that holds what it looks for in its line.
func (m *Model) matchRow(marks []cond, forward, self bool) (string, []int) {
	hit := m.finder()
	if hit == nil {
		return "", nil
	}
	step, i := 1, m.cursor
	if !forward {
		step = -1
	}
	if !self {
		i += step
	}
	for ; i >= 0 && i < len(m.vis); i += step {
		if id := m.vis[i].id; hit(id) {
			if hits := textHits(m.rowLine(id), marks); len(hits) > 0 {
				return id, hits
			}
		}
	}
	return "", nil
}

// noMarks says why n has no text to land on.
func (m *Model) noMarks() string {
	if m.at.query == "" && m.find == "" {
		return "nothing to find: keep a filter with / and Enter first"
	}
	return "the filter names no word or tag to land on: w steps into the text"
}

// enterMatch steps into a row's text with the caret on what the filter
// found: in the cursor's row, else in the next shown row that holds it.
func (m *Model) enterMatch(forward bool) {
	marks := m.findMarks()
	if len(marks) == 0 {
		m.msg = m.noMarks()
		return
	}
	id, hits := m.matchRow(marks, forward, true)
	if id == "" {
		m.msg = "no match further on"
		return
	}
	if id != m.current() {
		m.seek(id)
		m.land()
	}
	m.editRow(id)
	m.edit.normal = true
	m.edit.caret = hits[0]
	if !forward {
		m.edit.caret = hits[len(hits)-1]
	}
}

// textSeek moves the caret to the next or previous place the filter's words
// stand, going on into the next shown row that holds one.
func (m *Model) textSeek(e *editor, forward bool, n int) int {
	marks := m.findMarks()
	if len(marks) == 0 {
		m.msg = m.noMarks()
		return textMoved
	}
	for range n {
		cur := m.edit
		if at, ok := hitFrom(textHits(cur.buf, marks), cur.caret, forward); ok {
			cur.caret = at
			continue
		}
		id, hits := "", []int(nil)
		if !cur.sel && !cur.side && !cur.body {
			id, hits = m.matchRow(marks, forward, false)
		}
		if id == "" {
			m.msg = "no match further on"
			break
		}
		if !m.textMove(id) {
			return textLeft
		}
		m.land()
		m.edit.caret = hits[0]
		if !forward {
			m.edit.caret = hits[len(hits)-1]
		}
	}
	if m.edit != e {
		return textLeft
	}
	return textMoved
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
	if e.body {
		if target, inclusive, done := bodyMotion(e, k, n); done {
			return target, inclusive, true
		}
	}
	big := k == "W" || k == "B" || k == "E" || k == "gE"
	switch k {
	case "h", "left", "backspace":
		return max(at-n, 0), false, true
	case "ge", "gE":
		for range n {
			at = wordEndBack(e.buf, at, big)
		}
		return at, true, true
	case "%":
		end := len(e.buf)
		if e.body {
			end = lineEnd(e.buf, at)
		}
		target, ok := bracketMatch(e.buf, at, end)
		return target, true, ok
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
	case "n", "N":
		// For an operator: the nth match in this row, and no further.
		for range n {
			next, ok := hitFrom(textHits(e.buf, m.findMarks()), at, k == "n")
			if !ok {
				return e.caret, false, false
			}
			at = next
		}
		return at, false, true
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
	if k == "p" && e.body {
		from, to = paragraph(e.buf, at, inner)
		return from, to, true
	}
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
		if to < len(e.buf) && isBlank(e.buf[to]) {
			for to < len(e.buf) && isBlank(e.buf[to]) {
				to++
			}
		} else {
			for from > 0 && isBlank(e.buf[from-1]) {
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
	// In a body the caret stays off the newline that ends a line with text.
	if e.body && e.caret < len(e.buf) && e.buf[e.caret] == '\n' && e.caret > lineStart(e.buf, e.caret) {
		e.caret--
	}
}

// typeAt opens typing at a place in the line. What is typed joins the change
// that opened it, and Esc puts the caret back on a character: typing opened
// inside a row ends inside it.
func (e *editor) typeAt(at int) int {
	e.caret = max(0, min(at, len(e.buf)))
	e.normal, e.inside = false, true
	e.mark("text")
	return textTyping
}

// textBack ends typing opened inside the row and puts the caret back on a
// character, where the grammar goes on.
func (m *Model) textBack(e *editor) {
	if !e.body && !e.side {
		m.typedAt = typedSpot{e.row, e.caret}
	}
	if typed, only := e.inserted(); only && !m.replaying {
		m.textRepeat = textAct{keys: e.cmd, typed: typed}
	}
	// Typing that changed nothing leaves nothing for u to take back.
	if n := len(e.past); n > 0 && slices.Equal(e.past[n-1].buf, e.buf) {
		e.past = e.past[:n-1]
	}
	e.normal, e.inside, e.how, e.cmd, e.vertical = true, false, "", nil, false
	if !e.body || e.caret > lineStart(e.buf, e.caret) {
		e.caret--
	}
	e.rest()
}

// textKeys spells keys as if they had been pressed, for the dot key.
func textKeys(s string) []tea.KeyPressMsg {
	var keys []tea.KeyPressMsg
	for _, r := range s {
		keys = append(keys, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return keys
}

// textFilter leaves the row and narrows the pane to the word under the
// caret, as if it had been typed after /: a sigil whole, else the word.
func (m *Model) textFilter(e *editor) int {
	if e.side {
		m.msg = "a project's name filters nothing here: Enter, then / filters its rows"
		return textMoved
	}
	from, to, ok := textObject(e, "W", true)
	if !ok || isBlank(e.buf[min(e.caret, len(e.buf)-1)]) {
		m.msg = "no word under the caret to filter by"
		return textMoved
	}
	word := string(e.buf[from:to])
	if parseCond(word, m.today()).kind == 't' {
		from, to, _ = textObject(e, "w", true)
		word = string(e.buf[from:to])
	}
	row := e.row
	if !m.finishEdit() {
		return textLeft
	}
	if m.at.query == "" {
		m.at.here = row
	}
	m.at.query = word
	m.find, m.findHere = word, m.at.here
	m.remember(askFilter, word)
	m.refilter()
	m.cursor, m.top = 0, 0
	m.clearSelection()
	m.rebuild()
	m.seek(row)
	m.land()
	return textLeft
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
		if e.sel {
			e.sel = false
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

	// A g waits for its second key, alone or after an operator: gg in a
	// body, ge and gE back to a word's end, and gu gU g~ the case operators.
	if strings.HasSuffix(e.pend, "g") {
		e.pend = strings.TrimSuffix(e.pend, "g")
		switch {
		case k == "g" && e.body:
			return m.textGo(e, "gg", 0)
		case k == "e" || k == "E":
			return m.textGo(e, "g"+k, 0)
		case isCaseOp(e.pend) && k == e.pend:
			// gUgU, as gUU, takes the whole line.
			return m.caseLine(e)
		case e.pend == "" && isCaseOp(k) && e.sel:
			return m.caseSelection(e, k)
		case e.pend == "" && isCaseOp(k):
			e.pend, e.opCount, e.count = k, e.count, 0
			return textPending
		}
		return textMoved
	}

	// In a body z folds it: zc to its first line, zo open above the rows,
	// za the other way, as it shows once the caret leaves it.
	if e.body && e.pend == "z" {
		e.pend = ""
		open := m.bodyOpen(e.row)
		switch k {
		case "o":
			open = true
		case "c":
			open = false
		case "a":
			open = !open
		default:
			return textMoved
		}
		m.setFold(bodyOpenKey(e.row), open)
		m.msg = "the body folds to its first line above the rows"
		if open {
			m.msg = "the body stays open above the rows"
		}
		return textMoved
	}

	// An operator is waiting for a motion or a text object.
	if e.pend != "" {
		if e.body && len(e.pend) == 1 && strings.Contains("dcy", e.pend) {
			// Doubled, an operator takes whole lines, since a body has no
			// rows to give it to.
			if k == e.pend {
				return m.bodyLines(e, k, e.caret, linesDown(e.buf, e.caret, max(e.textCount(), 1)))
			}
		}
		if len(e.pend) == 1 && strings.Contains("dcyuU~", e.pend) && k == "g" {
			e.pend += k
			return textPending
		}
		if isCaseOp(e.pend) && k == e.pend {
			return m.caseLine(e)
		}
		if len(e.pend) == 2 {
			from, to, ok := textObject(e, k, e.pend[1] == 'i')
			if !ok {
				return textMoved
			}
			if e.pend[0] == 'v' {
				// A text object named while selecting is what is selected.
				e.anchor, e.caret = from, max(to-1, from)
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

	if e.sel {
		// A selection runs from where v was pressed to the caret, both
		// included. The key that acts on it ends it.
		from, to := min(e.anchor, e.caret), min(max(e.anchor, e.caret)+1, len(e.buf))
		switch k {
		case "v":
			e.sel = false
			return textMoved
		case "i", "a":
			e.pend = "v" + k
			return textPending
		case "y":
			e.sel = false
			return m.textOperate(e, "y", from, to)
		case "u", "U":
			return m.caseSelection(e, k)
		case "d", "x", "c", "s", "~":
			if e.body && k != "~" {
				// A body's selection may run over lines, which a count of
				// characters on one line cannot say again.
				e.sel = false
				return m.textOperate(e, map[string]string{"d": "d", "x": "d", "c": "c", "s": "c"}[k], from, to)
			}
			// Remembered as so many characters from the caret, which is what
			// the dot key makes again.
			if k == "d" {
				k = "x"
			} else if k == "c" {
				k = "s"
			}
			e.sel, e.caret, e.count, e.opCount = false, from, to-from, 0
			e.cmd = textKeys(strconv.Itoa(to-from) + k)
		case "D", "C", "X", "r", "I", "A", "p", "P", "ctrl+r", ".", "*", "delete", "ctrl+a", "ctrl+x":
			return textMoved
		}
	}

	n := max(e.textCount(), 1)
	if e.body {
		if r, ok := m.bodyStep(e, k, n); ok {
			return r
		}
	}
	switch k {
	case "v":
		if len(e.buf) > 0 {
			e.sel, e.anchor = true, e.caret
		}
		return textMoved
	case "*":
		return m.textFilter(e)
	case "d", "c", "y":
		e.pend, e.opCount, e.count = k, e.count, 0
		return textPending
	case "f", "F", "t", "T", "r":
		e.pend = k
		return textPending
	case "g":
		e.pend = k
		return textPending
	case "ctrl+a", "ctrl+x":
		by := n
		if k == "ctrl+x" {
			by = -n
		}
		if !e.bump(by) {
			m.msg = "no number at or after the caret"
			return textMoved
		}
		return textChanged
	case "j", "down", "k", "up":
		// By drawn line. Past the row's first or last line the caret goes on
		// into the row drawn above or below, keeping this one, as Up and
		// Down do while typing. A selection stays in its row.
		by := 1
		if k == "k" || k == "up" {
			by = -1
		}
		for range n {
			cur := m.edit
			l := cur.wrapped(m.editWidth(), nil)
			p := l.pos[cur.caret]
			if line := p.y + by; line >= 0 && line < len(l.lines) {
				cur.caret = l.at(line, p.x)
				cur.rest()
				continue
			}
			next := m.cursor + by
			if cur.sel || cur.side || next < 0 || next >= len(m.vis) || !m.textMove(m.vis[next].id) {
				break
			}
			l = m.edit.wrapped(m.editWidth(), nil)
			line := 0
			if by < 0 {
				line = len(l.lines) - 1
			}
			m.edit.caret = l.at(line, p.x)
			m.edit.rest()
		}
		if m.edit != e {
			return textLeft
		}
		return textMoved
	case "n", "N":
		return m.textSeek(e, k == "n", n)
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
		e.keep()
		return e.typeAt(e.caret)
	case "a":
		e.keep()
		return e.typeAt(e.caret + 1)
	case "I":
		e.keep()
		return e.typeAt(0)
	case "A":
		e.keep()
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
		// Lines taken from a body join into one when put in a row.
		put := m.textRegister
		if strings.ContainsAny(put, "\n\t") {
			put = strings.Join(strings.Fields(put), " ")
		}
		e.insert([]rune(strings.Repeat(put, n)))
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
	if !strings.Contains("dcyuU~", op) {
		op = ""
	}
	// Changing a word leaves the blank after it, as ce does.
	if op == "c" && (k == "w" || k == "W") && e.caret < len(e.buf) && !isBlank(e.buf[e.caret]) {
		k = map[string]string{"w": "e", "W": "E"}[k]
	}
	if e.body && op != "" {
		// Up and down, and to either end, an operator takes whole lines.
		n := max(e.textCount(), 1)
		switch k {
		case "j", "down":
			return m.bodyLines(e, op, e.caret, linesDown(e.buf, e.caret, n+1))
		case "k", "up":
			return m.bodyLines(e, op, e.caret, linesUp(e.buf, e.caret, n))
		case "gg":
			return m.bodyLines(e, op, e.caret, 0)
		case "G":
			return m.bodyLines(e, op, e.caret, len(e.buf))
		}
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
	}
	if inclusive {
		// A motion that takes the character it lands on takes the caret's
		// too when it goes back, as ge and % can.
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
	if isCaseOp(op) {
		e.keep()
		e.recase(op, from, to)
		e.caret = from
		return textChanged
	}
	if op == "y" {
		m.textRegister = string(e.buf[from:to])
		e.flash = [2]int{from, to}
		m.pendingCmd = tea.Batch(tea.SetClipboard(m.textRegister), m.startFlash())
		e.caret = from
		e.rest()
		return textMoved
	}
	e.keep()
	if op == "c" && e.body && to > from && e.buf[to-1] == '\n' {
		to-- // a changed paragraph keeps the line it is typed on
	}
	if from < to {
		m.textRegister = e.cut(from, to)
	}
	if op == "c" {
		return e.typeAt(from)
	}
	return textChanged
}

// caseLine changes the case of the whole line, or in a body of the caret's
// lines, as gUU, guu and g~~ do.
func (m *Model) caseLine(e *editor) int {
	op := e.pend
	if !e.body {
		return m.textOperate(e, op, 0, len(e.buf))
	}
	return m.bodyLines(e, op, e.caret, linesDown(e.buf, e.caret, max(e.textCount(), 1)))
}

// caseSelection changes the case of what is selected, as u and U do on a
// Vim selection. It is remembered as so many characters from the caret.
func (m *Model) caseSelection(e *editor, op string) int {
	from, to := min(e.anchor, e.caret), min(max(e.anchor, e.caret)+1, len(e.buf))
	e.sel, e.caret = false, from
	if !e.body {
		e.cmd = textKeys("g" + op + strconv.Itoa(to-from) + "l")
	}
	return m.textOperate(e, op, from, to)
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
			e.normal, e.inside, e.how = true, false, ""
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
	if e.body && !e.sel && e.pend == "" {
		return "in the body: j k { } move · i a o c d change · zo zc fold · Esc done"
	}
	if e.sel {
		return "selecting: motions extend · d c y x ~ act · v or Esc ends"
	}
	if e.pend == "g" {
		says := "ge gE back to a word's end · gu gU g~ lower, upper, switch case"
		if e.body {
			says = "gg the first line · " + says
		}
		return says
	}
	if isCaseOp(e.pend) {
		op := "g" + e.pend
		return op + "w " + op + "e " + op + "$ through a motion · " + op + "iw " + op + "i\" objects · " + op + e.pend + " the line"
	}
	if op := e.pend[:min(len(e.pend), 1)]; op == "d" || op == "c" || op == "y" {
		// A waiting operator says what may follow it, as one does on the
		// outline.
		if len(e.pend) == 2 && strings.Contains("ia", e.pend[1:]) {
			return e.pend + "w a word · " + e.pend + "\" quotes · " + e.pend + "( [ { < brackets"
		}
		if len(e.pend) == 1 {
			return op + "w " + op + "e " + op + "$ " + op + "t, " + op + "n through a motion · " + op + "iw " + op + "i\" " + op + "i( objects"
		}
	}
	return "in the row: w b e f n move · i a c d x change · v select · Esc done"
}
