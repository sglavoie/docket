package tui

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/sigil"
)

// The surfaces a line is typed on. Each reads its own grammar, so each has
// its own answer to what a word means.
const surfaceRow = "row"

var dateWords = []string{"today", "tomorrow", "mon", "tue", "wed", "thu", "fri", "sat", "sun", "eow", "eom"}

// looksLikeSigil reports a word that starts like a sigil. When the grammar
// cannot read such a word it stays plain text, and the line says so in colour
// while it is typed.
func looksLikeSigil(w string) bool {
	return len(w) > 1 && w != "##" && strings.ContainsRune("#@$~", rune(w[0]))
}

// styler returns styled spans for a line. Filter phrases stay whole and
// alternatives can carry different styles; other surfaces read each word.
func (m *Model) styler(surface string) func(string) []word {
	today := m.today()
	row := func(w string) string {
		switch {
		case w[0] == '~' && sigil.IsSigil(w, today):
			// A colour is drawn in itself, so it is seen before it is kept.
			c, _ := sigil.ParseColor(w[1:])
			return m.colorOf(c)
		case sigil.IsSigil(w, today):
			return m.colors().accent
		case looksLikeSigil(w):
			return m.colors().warn
		}
		return ""
	}
	styleWord := func(w string) []word {
		style := ""
		switch surface {
		case surfaceRow:
			style = row(w)
		case askCapture:
			style = row(w)
			if name, ok := strings.CutPrefix(w, "%"); ok && name != "" {
				style = m.colors().warn
				if len(match(name, 2, m.targets(askMove))) > 0 {
					style = m.colors().accent
				}
			}
		case askStamp:
			if w != "-" {
				style = m.colors().accent
				if _, bad := parseStamp(w, today); bad != "" {
					style = m.colors().warn
				}
			}
		case askFilter:
			// A bar or a bracket standing alone joins or groups terms.
			if w == "|" || w == "(" || w == ")" || w == "-(" {
				style = m.colors().accent
				break
			}
			terms := parseQuery(w, today)
			if len(terms) == 0 {
				break
			}
			parts := filterParts(w, true)
			if len(parts) == len(terms[0].any) {
				var spans []word
				for i, c := range terms[0].any {
					if i > 0 {
						spans = append(spans, word{"", "|"})
					}
					style = ""
					if c.kind != 't' {
						style = m.colors().accent
					}
					spans = append(spans, word{style, parts[i]})
				}
				if len(parts) > 1 || terms[0].any[0].kind != 't' {
					return spans
				}
			}
			// An unreadable sigil side makes the whole word literal text.
			quoted := make([]bool, len(parts))
			for i, part := range parts {
				part = strings.TrimPrefix(part, "-")
				if terms[0].above {
					part = strings.TrimPrefix(part, "^")
				}
				if terms[0].below {
					part = strings.TrimPrefix(part, "_")
				}
				if terms[0].parent {
					part = strings.TrimPrefix(part, "<")
				}
				if terms[0].child {
					part = strings.TrimPrefix(part, ">")
				}
				quoted[i] = strings.HasPrefix(part, `"`)
				if part != "" && strings.ContainsRune(sigilStarts, rune(part[0])) {
					style = m.colors().warn
				}
			}
			// Even when another side is unreadable, phrases stay plain.
			if style != "" && slices.Contains(quoted, true) {
				var spans []word
				for i, part := range parts {
					if i > 0 {
						spans = append(spans, word{"", "|"})
					}
					partStyle := style
					if quoted[i] {
						partStyle = ""
					}
					spans = append(spans, word{partStyle, part})
				}
				return spans
			}
		}
		return []word{{style, w}}
	}
	return func(line string) []word {
		parts := strings.Split(line, " ")
		if surface == askFilter {
			parts = filterParts(line, false)
		}
		var spans []word
		first := true
		for _, part := range parts {
			if part == "" {
				continue
			}
			i := strings.Index(line, part)
			if i > 0 {
				spans = append(spans, word{"", line[:i]})
			}
			if first && part == sigil.NoteMark && (surface == surfaceRow || surface == askCapture) {
				// Only leading the line does the mark make a note, so only
				// there is it drawn as a sigil.
				spans = append(spans, word{m.colors().accent, part})
			} else {
				spans = append(spans, styleWord(part)...)
			}
			first = false
			line = line[i+len(part):]
		}
		if line != "" {
			spans = append(spans, word{"", line})
		}
		return spans
	}
}

// suggest returns the rest of the sigil being typed at the end of the line:
// a tag already in use, a context, a state, or a date word. The right arrow takes it.
func (m *Model) suggest(e *editor, surface string) string {
	typed, values := m.sigilValues(e, surface)
	for _, v := range values {
		if len(v) > len(typed) {
			return v[len(typed):]
		}
	}
	return ""
}

// sigilValues returns what is typed of the sigil at the end of the line,
// after its mark and in small letters, and every value it could become, in
// the order suggest offers them. The typed part is always the line's tail.
func (m *Model) sigilValues(e *editor, surface string) (string, []string) {
	if surface != surfaceRow && surface != askCapture && surface != askStamp && surface != askFilter {
		return "", nil
	}
	if e.caret != len(e.buf) || len(e.buf) == 0 || e.buf[len(e.buf)-1] == ' ' {
		return "", nil
	}
	line := string(e.buf)
	word := line[strings.LastIndexByte(line, ' ')+1:]
	if surface == askFilter {
		parts := filterParts(line, false)
		if len(parts) == 0 || !strings.HasSuffix(line, parts[len(parts)-1]) {
			return "", nil
		}
		word = parts[len(parts)-1]
	}
	if surface == askStamp || surface == askFilter {
		word = strings.TrimPrefix(word, "-")
	}
	if surface == askFilter {
		for _, twice := range []string{"^^", "__", "<<", ">>"} {
			if strings.HasPrefix(word, twice) {
				return "", nil
			}
		}
		// One mark of where the term is read: above, below, parent or child.
		if word != "" && strings.ContainsRune("^_<>", rune(word[0])) {
			word = word[1:]
		}
	}
	if word == "" {
		return "", nil
	}
	if parts := filterParts(word, true); surface == askFilter && len(parts) > 1 {
		sig := word[:len(word)-len(strings.TrimLeft(word, sigilStarts))]
		word = parts[len(parts)-1]
		if word == "" || !strings.ContainsRune(sigilStarts, rune(word[0])) {
			word = sig + word
		}
		if word == "" {
			return "", nil
		}
	}
	typed := strings.ToLower(word[1:])
	var offers []string
	if rest, start := strings.CutPrefix(typed, "@"); start && word[0] == '@' {
		// A start date takes the same words as a deadline.
		typed, word = rest, "="
		offers = dateWords
		if surface == askFilter {
			offers = append([]string{"later"}, dateWords...)
		}
	}
	if rest, ctx := strings.CutPrefix(typed, "#"); ctx && word[0] == '#' {
		// ## is a context, which takes the contexts in use.
		typed, word = rest, "="
		offers = m.contexts
	}
	switch word[0] {
	case '#':
		offers = m.tags
	case '$':
		offers = sigil.StateNames
		if surface == askFilter {
			offers = append(slices.Clone(offers), "next", "today", "first", "project", "missed", "pinned", "blocked", "unchecked")
		}
	case '@':
		offers = dateWords
		if surface == askFilter {
			offers = append([]string{"overdue", "week"}, dateWords...)
		}
	case '~':
		// On the filter, ~ asks when a row changed, which is no colour.
		if surface != askFilter {
			offers = sigil.ColorNames
		}
	}
	var values []string
	for _, o := range offers {
		if strings.HasPrefix(o, typed) {
			values = append(values, o)
		}
	}
	return typed, values
}

// completeFilter walks the values the sigil at the end of the filter line
// could become: Tab writes the first and again the next, Shift+Tab walks
// back, and past either end the line is as it was typed. While the values
// are listed, the arrows walk them too. A sigil only one value finishes is
// finished at once. It reports false when there is nothing to complete,
// where Tab switches between flat and in context instead.
func (m *Model) completeFilter(a *prompt, k string) bool {
	by := 1
	listed := len(a.menu) > 0
	switch {
	case k == "tab", listed && (k == "down" || k == "ctrl+n"):
	case k == "shift+tab", listed && (k == "up" || k == "ctrl+p"):
		by = -1
	default:
		a.choice, a.menu = 0, nil
		return false
	}
	typed := string(a.ed.buf)
	if a.choice > 0 {
		typed = a.draft
	}
	stem, values := m.sigilValues(&editor{buf: []rune(typed), caret: len([]rune(typed))}, askFilter)
	if a.choice == 0 && !slices.ContainsFunc(values, func(v string) bool { return len(v) > len(stem) }) {
		return false
	}
	base := typed[:len(typed)-len(stem)]
	line := base + values[0]
	if n := len(values); n > 1 {
		if a.choice == 0 {
			a.draft = typed
		}
		a.menu, a.menuAt = values, len([]rune(base))
		a.choice = (a.choice + by + n + 1) % (n + 1)
		line = a.draft
		if a.choice > 0 {
			line = base + values[a.choice-1]
		}
	}
	a.ed = editor{buf: []rune(line), caret: len([]rune(line))}
	a.recalled = 0
	return true
}

// tagsByUse orders the tags in use, most used first.
func tagsByUse(count map[string]int) []string {
	tags := make([]string, 0, len(count))
	for tag := range count {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		if count[tags[i]] != count[tags[j]] {
			return count[tags[i]] > count[tags[j]]
		}
		return tags[i] < tags[j]
	})
	return tags
}

// window returns the part of the buffer that fits in avail columns with the
// caret inside it, coloured word by word, and the caret's column within it.
func (e *editor) window(avail int, style func(string) []word) (string, int) {
	if e.caret < e.off {
		e.off = e.caret
	}
	for e.off < e.caret && ansi.StringWidth(string(e.buf[e.off:e.caret])) > avail-1 {
		e.off++
	}

	styles := make([]string, len(e.buf))
	if style != nil {
		k := 0
		for _, span := range style(string(e.buf)) {
			for range []rune(span.text) {
				styles[k] = span.style
				k++
			}
		}
	}

	var b strings.Builder
	used, cur := 0, ""
	for i := e.off; i < len(e.buf); i++ {
		w := ansi.StringWidth(string(e.buf[i]))
		if used+w > avail {
			break
		}
		if styles[i] != cur {
			if cur != "" {
				b.WriteString(sgrReset)
			}
			b.WriteString(styles[i])
			cur = styles[i]
		}
		b.WriteRune(e.buf[i])
		used += w
	}
	if cur != "" {
		b.WriteString(sgrReset)
	}
	return b.String(), ansi.StringWidth(string(e.buf[e.off:e.caret]))
}

// ghost is the suggestion drawn after the typed text, cut to the room left.
func (m *Model) ghost(text, rest string, avail int) string {
	room := avail - ansi.StringWidth(text) - 1
	if rest == "" || room < 1 {
		return ""
	}
	return m.colors().dim + ansi.Truncate(rest, room, "") + sgrReset
}

type textPoint struct{ x, y int }

// A position for every rune boundary keeps wrapping, arrows and the terminal
// caret in agreement. Spaces are cells too, including at the end of a line.
type editLayout struct {
	lines []string
	pos   []textPoint
}

func (e *editor) wrapped(width int, style func(string) []word) editLayout {
	width = max(width, 1)
	l := editLayout{pos: make([]textPoint, len(e.buf)+1)}
	widths := make([]int, len(e.buf))
	for i, r := range e.buf {
		if r >= ' ' && r < 0x7f {
			widths[i] = 1
		} else {
			widths[i] = ansi.StringWidth(string(r))
		}
	}
	var line strings.Builder
	currentStyle := ""
	setStyle := func(style string) {
		if style == currentStyle {
			return
		}
		if currentStyle != "" {
			line.WriteString(sgrReset)
		}
		line.WriteString(style)
		currentStyle = style
	}
	x, y := 0, 0
	next := func() {
		setStyle("")
		l.lines = append(l.lines, line.String())
		line = strings.Builder{}
		x, y = 0, y+1
	}
	for i := 0; i < len(e.buf); {
		// A body breaks its lines where they were broken, and draws a tab to
		// the next stop of four.
		switch e.buf[i] {
		case '\n':
			l.pos[i] = textPoint{x, y}
			next()
			i++
			continue
		case '\t':
			tab := 4 - x%4
			if x > 0 && x+tab > width {
				next()
				tab = 4
			}
			l.pos[i] = textPoint{x, y}
			setStyle("")
			line.WriteString(strings.Repeat(" ", min(tab, width)))
			x += min(tab, width)
			i++
			continue
		}
		end, wordWidth := i+1, widths[i]
		if e.buf[i] != ' ' {
			for end < len(e.buf) && !isBlank(e.buf[end]) {
				wordWidth += widths[end]
				end++
			}
			if wordWidth <= width && x > 0 && x+wordWidth > width {
				next()
			}
		}
		spans := []word{{text: string(e.buf[i:end])}}
		if style != nil && e.buf[i] != ' ' {
			spans = style(spans[0].text)
		}
		for _, span := range spans {
			for _, r := range span.text {
				if x > 0 && x+widths[i] > width {
					next()
				}
				l.pos[i] = textPoint{x, y}
				if e.sel && i >= min(e.anchor, e.caret) && i <= max(e.anchor, e.caret) || i >= e.flash[0] && i < e.flash[1] {
					setStyle(span.style + sgrReverse)
				} else {
					setStyle(span.style)
				}
				line.WriteRune(r)
				x += widths[i]
				i++
			}
		}
	}
	// A terminal caret needs a cell even after an exactly full final line.
	if x >= width {
		next()
	}
	l.pos[len(e.buf)] = textPoint{x, y}
	setStyle("")
	l.lines = append(l.lines, line.String())
	return l
}

func (l editLayout) at(line, column int) int {
	best := 0
	for i, p := range l.pos {
		if p.y == line {
			if p.x > column {
				return best
			}
			best = i
		} else if p.y > line {
			break
		}
	}
	return best
}

func (m *Model) editWidth() int {
	if m.edit != nil && m.edit.body {
		w, _ := m.size()
		if w >= sidebarFrom {
			w -= m.sideWidth() + 3
		}
		return m.bodyWidth(w)
	}
	if m.edit != nil && m.edit.side {
		return max(m.sideWidth()-2, 1) // after the gutter and its space
	}
	w, _ := m.size()
	if w >= sidebarFrom {
		w -= m.sideWidth() + 3
	}
	return max(w-m.numberCols()-4-2*m.vis[m.cursor].depth, 1)
}

func (m *Model) editVertical(by int) {
	e := m.edit
	l := e.wrapped(m.editWidth(), nil)
	p := l.pos[e.caret]
	if !e.vertical {
		e.column = p.x
	}
	e.vertical = true
	line := p.y + by
	if line >= 0 && line < len(l.lines) {
		e.caret = l.at(line, e.column)
		return
	}
	m.editNext(by)
	if m.edit != e && m.edit != nil {
		l = m.edit.wrapped(m.editWidth(), nil)
		line = 0
		if by < 0 {
			line = len(l.lines) - 1
		}
		m.edit.caret = l.at(line, e.column)
		m.edit.column, m.edit.vertical = e.column, true
	}
}

// valueList is what the popup lists while a sigil ends the line: the
// values the word could become, the rune of the line the value is written
// from, and which one is written, on a row, the capture or the stamp line.
// On the filter Tab lists them instead, and ~ is a time there. A hex value, a removal that
// takes no value and a finished value list nothing.
func (m *Model) valueList(e *editor, surface string) (names []string, from int, at int) {
	if surface != surfaceRow && surface != askCapture && surface != askStamp || e.normal || e.body {
		return nil, 0, 0
	}
	line := string(e.buf)
	if e.walk > 0 {
		line = e.walkDraft
	}
	word := line[strings.LastIndexByte(line, ' ')+1:]
	if strings.HasPrefix(word, "-") && !strings.HasPrefix(word, "-#") {
		// -#tag and -##context name what goes; -@, -$ and -~ take nothing.
		return nil, 0, 0
	}
	stem, values := m.sigilValues(&editor{buf: []rune(line), caret: len([]rune(line))}, surface)
	if len(values) == 0 || len(values) == 1 && values[0] == stem && e.walk == 0 {
		return nil, 0, 0
	}
	return values, len([]rune(line)) - len([]rune(stem)), e.walk - 1
}

// walkValues walks the popup: Tab, Down and Ctrl+N write the next
// name, Shift+Tab, Up and Ctrl+P the one before, and past either end the
// word is as it was typed. Any other key ends the walk where it stands. It
// reports whether the key was the walk's.
func (m *Model) walkValues(e *editor, surface, k string) bool {
	by := 0
	switch k {
	case "tab", "down", "ctrl+n":
		by = 1
	case "shift+tab", "up", "ctrl+p":
		by = -1
	}
	names, from, _ := m.valueList(e, surface)
	if by == 0 || len(names) == 0 {
		e.walk, e.walkDraft = 0, ""
		return false
	}
	if e.walk == 0 {
		e.walkDraft = string(e.buf)
	}
	n := len(names)
	e.walk = (e.walk + by + n + 1) % (n + 1)
	line := e.walkDraft
	if e.walk > 0 {
		line = string([]rune(e.walkDraft)[:from]) + names[e.walk-1]
	}
	e.buf, e.caret = []rune(line), len([]rune(line))
	return true
}

// valueItems are the values as the popup draws them: a colour in itself, a
// tag in its own colour, anything else plain.
func (m *Model) valueItems(e *editor, names []string) []string {
	line := string(e.buf)
	if e.walk > 0 {
		line = e.walkDraft
	}
	word := strings.TrimPrefix(line[strings.LastIndexByte(line, ' ')+1:], "-")
	items := make([]string, len(names))
	for i, name := range names {
		switch {
		case strings.HasPrefix(word, "~"):
			items[i] = m.colorOf(name) + name
		case strings.HasPrefix(word, "#") && !strings.HasPrefix(word, "##"):
			items[i] = m.tagColor(name) + name
		default:
			items[i] = name
		}
	}
	// A tag or a context says how many rows carry it, dim and right-aligned
	// beside its name, so the one list reads both and the line need not.
	var counts []string
	switch {
	case strings.HasPrefix(word, "##"):
		for _, name := range names {
			counts = append(counts, strconv.Itoa(m.contextUse[name]))
		}
	case strings.HasPrefix(word, "#"):
		for _, name := range names {
			counts = append(counts, strconv.Itoa(m.tagCount(name)))
		}
	default:
		return items
	}
	nameW, countW := 0, 0
	for i, name := range names {
		nameW, countW = max(nameW, ansi.StringWidth(name)), max(countW, len(counts[i]))
	}
	for i, name := range names {
		items[i] += strings.Repeat(" ", nameW-ansi.StringWidth(name)+2+countW-len(counts[i])) + m.colors().dim + counts[i]
	}
	return items
}

// tagCount is how many live rows carry a tag: a bare name counts every
// value it carries, as #est counts each #est=…, and a name=value that one.
func (m *Model) tagCount(name string) int {
	if strings.Contains(name, "=") {
		return m.tagUse[name]
	}
	n := 0
	for tag, use := range m.tagUse {
		if sigil.TagName(tag) == name {
			n += use
		}
	}
	return n
}
