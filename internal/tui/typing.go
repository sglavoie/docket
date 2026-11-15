package tui

import (
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

// styler returns how a surface colours a word as it is typed: accent for a
// sigil it reads, warning for one it does not, nothing for prose.
func (m *Model) styler(surface string) func(string) string {
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
	switch surface {
	case surfaceRow:
		return row
	case askCapture:
		return func(w string) string {
			if name, ok := strings.CutPrefix(w, "%"); ok && name != "" {
				if len(match(m.targets(false), name, 2)) > 0 {
					return sgrAccent
				}
				return sgrWarn
			}
			return row(w)
		}
	case askStamp:
		return func(w string) string {
			if w == "-" {
				return ""
			}
			if _, bad := parseStamp(w, today); bad != "" {
				return sgrWarn
			}
			return sgrAccent
		}
	case askFilter:
		return func(w string) string {
			terms := parseQuery(w, today)
			switch {
			case len(terms) == 0:
				return ""
			case terms[0].kind != 't':
				return sgrAccent
			case looksLikeSigil(strings.TrimPrefix(w, "-")):
				return sgrWarn
			}
			return ""
		}
	}
	return nil
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
	typed := strings.ToLower(word[1:])
	var offers []string
	switch word[0] {
	case '#':
		offers = m.tags
	case '$':
		offers = sigil.StateNames
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
func (e *editor) window(avail int, style func(string) string) (string, int) {
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
			s := style(string(e.buf[i:j]))
			for k := i; k < j; k++ {
				styles[k] = s
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
