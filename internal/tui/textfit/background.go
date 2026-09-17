package textfit

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// resetSequences are the two spellings of a full SGR reset. Only styles this
// package's callers rendered reach PaintBackground, and lipgloss closes every
// one of them with one of these two, so recognizing the pair exactly is the
// whole of what is needed — parsing every SGR a terminal can carry would buy
// nothing and could misread a color parameter as a reset.
var resetSequences = [...]string{ansi.ResetStyle, "\x1b[0m"}

// PaintBackground lays one background under a run of already-styled text
// without rewriting a byte of it. A lipgloss style applied over assembled text
// would stop at that text's first reset, leaving everything past it bare, which
// is why a whole row cannot be tinted by rendering it a second time.
//
// SGR is cumulative, so arming the background once at the front and again after
// every reset paints exactly the cells no segment claimed: a badge that sets its
// own background keeps it for its own span, and the blanks and foreground-only
// segments around it take this one. The armed background is closed at the end,
// so nothing bleeds into the row below.
//
// A nil background is no background: the text is returned untouched, which is
// what lets one seam serve rows that wear a band and rows that do not.
func PaintBackground(text string, bg color.Color) string {
	if bg == nil || text == "" {
		return text
	}
	arm := ansi.NewStyle().BackgroundColor(bg).String()

	var b strings.Builder
	b.Grow(len(text) + 4*len(arm))
	b.WriteString(arm)
	for {
		idx, width := nextReset(text)
		if idx < 0 {
			b.WriteString(text)
			b.WriteString(ansi.ResetStyle)
			return b.String()
		}
		b.WriteString(text[:idx+width])
		text = text[idx+width:]
		if text == "" {
			// The run already ends reset, so the armed background is already closed.
			return b.String()
		}
		b.WriteString(arm)
	}
}

// nextReset reports where the leftmost reset sequence starts and how long it is,
// or -1 when the text carries none.
func nextReset(text string) (int, int) {
	start, width := -1, 0
	for _, seq := range resetSequences {
		idx := strings.Index(text, seq)
		if idx < 0 || (start >= 0 && idx >= start) {
			continue
		}
		start, width = idx, len(seq)
	}
	return start, width
}
