package tui

import (
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/sigil"
)

// The surfaces a line is typed on. Each reads its own grammar, so each has
// its own answer to what a word means.
const surfaceRow = "row"

var dateWords = []string{"today", "tomorrow", "mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// looksLikeSigil reports a word that starts like a sigil. When the grammar
// cannot read such a word it stays plain text, and the line says so in colour
// while it is typed.
func looksLikeSigil(w string) bool {
	return len(w) > 1 && strings.ContainsRune("#@$", rune(w[0]))
}

// styler returns styled spans for a word. Filter alternatives can carry
// different styles; other surfaces read the word as a whole.
func (m *Model) styler(surface string) func(string) []word {
	today := m.today()
	row := func(w string) string {
		switch {
		case sigil.IsSigil(w, today):
			return sgrAccent
		case looksLikeSigil(w):
			return sgrWarn
		}
		return ""
	}
	return func(w string) []word {
		style := ""
		switch surface {
		case surfaceRow:
			style = row(w)
		case askCapture:
			style = row(w)
			if name, ok := strings.CutPrefix(w, "%"); ok && name != "" {
				style = sgrWarn
				if len(match(name, 2, m.targets(false))) > 0 {
					style = sgrAccent
				}
			}
		case askStamp:
			if w != "-" {
				style = sgrAccent
				if _, bad := parseStamp(w, today); bad != "" {
					style = sgrWarn
				}
			}
		case askFilter:
			// A bar or a bracket standing alone joins or groups terms.
			if w == "|" || w == "(" || w == ")" || w == "-(" {
				style = sgrAccent
				break
			}
			terms := parseQuery(w, today)
			if len(terms) == 0 {
				break
			}
			parts := strings.Split(w, "|")
			if len(parts) == len(terms[0].any) {
				var spans []word
				for i, c := range terms[0].any {
					if i > 0 {
						spans = append(spans, word{"", "|"})
					}
					style = ""
					if c.kind != 't' {
						style = sgrAccent
					}
					spans = append(spans, word{style, parts[i]})
				}
				if len(parts) > 1 || terms[0].any[0].kind != 't' {
					return spans
				}
			}
			// An unreadable sigil side makes the whole word literal text.
			for _, part := range parts {
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
				if part != "" && strings.ContainsRune(sigilStarts, rune(part[0])) {
					style = sgrWarn
					break
				}
			}
		}
		return []word{{style, w}}
	}
}

// suggest returns the rest of the sigil being typed at the end of the line:
// a tag already in use, a state, or a date word. The right arrow takes it.
func (m *Model) suggest(e *editor, surface string) string {
	if surface != surfaceRow && surface != askCapture && surface != askStamp && surface != askFilter {
		return ""
	}
	if e.caret != len(e.buf) || len(e.buf) == 0 || e.buf[len(e.buf)-1] == ' ' {
		return ""
	}
	line := string(e.buf)
	word := line[strings.LastIndexByte(line, ' ')+1:]
	if surface == askStamp || surface == askFilter {
		word = strings.TrimPrefix(word, "-")
	}
	if surface == askFilter {
		for _, twice := range []string{"^^", "__", "<<", ">>"} {
			if strings.HasPrefix(word, twice) {
				return ""
			}
		}
		// One mark of where the term is read: above, below, parent or child.
		if word != "" && strings.ContainsRune("^_<>", rune(word[0])) {
			word = word[1:]
		}
	}
	if word == "" {
		return ""
	}
	if i := strings.LastIndexByte(word, '|'); surface == askFilter && i >= 0 {
		sig := word[:len(word)-len(strings.TrimLeft(word, sigilStarts))]
		word = word[i+1:]
		if word == "" || !strings.ContainsRune(sigilStarts, rune(word[0])) {
			word = sig + word
		}
		if word == "" {
			return ""
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
	switch word[0] {
	case '#':
		offers = m.tags
	case '$':
		offers = sigil.StateNames
		if surface == askFilter {
			offers = append(slices.Clone(offers), "next", "today")
		}
	case '@':
		offers = dateWords
		if surface == askFilter {
			offers = append([]string{"overdue", "week"}, dateWords...)
		}
	}
	for _, o := range offers {
		if len(o) > len(typed) && strings.HasPrefix(o, typed) {
			return o[len(typed):]
		}
	}
	return ""
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
		for i := 0; i < len(e.buf); {
			if e.buf[i] == ' ' {
				i++
				continue
			}
			j := i
			for j < len(e.buf) && e.buf[j] != ' ' {
				j++
			}
			k := i
			for _, span := range style(string(e.buf[i:j])) {
				for range []rune(span.text) {
					styles[k] = span.style
					k++
				}
			}
			i = j
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
func ghost(text, rest string, avail int) string {
	room := avail - ansi.StringWidth(text) - 1
	if rest == "" || room < 1 {
		return ""
	}
	return sgrDim + ansi.Truncate(rest, room, "") + sgrReset
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
		widths[i] = ansi.StringWidth(string(r))
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
		end, wordWidth := i+1, widths[i]
		if e.buf[i] != ' ' {
			for end < len(e.buf) && e.buf[end] != ' ' {
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
				if e.sel && i >= min(e.anchor, e.caret) && i <= max(e.anchor, e.caret) {
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
	w, _ := m.size()
	if w >= sidebarFrom {
		w -= sidebarCols + 3
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
