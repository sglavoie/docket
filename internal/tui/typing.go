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
				if len(match(m.targets(false), name, 2)) > 0 {
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
				if part != "" && strings.ContainsRune("!#$@*~", rune(part[0])) {
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
	if word == "" {
		return ""
	}
	if i := strings.LastIndexByte(word, '|'); surface == askFilter && i >= 0 {
		sig := word[:len(word)-len(strings.TrimLeft(word, "!#$@*~"))]
		word = word[i+1:]
		if word == "" || !strings.ContainsRune("!#$@*~", rune(word[0])) {
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
			offers = append(slices.Clone(offers), "next")
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
