package conflict

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// mergeEditor edits the original runes without a textarea's tab expansion,
// control-character filtering or line limit. Whole-file merges must be lossless.
type mergeEditor struct {
	original string
	text     []rune
	cursor   int
	top      int
	active   bool
}

func newMergeEditor(text string) *mergeEditor {
	return &mergeEditor{original: text, text: []rune(text), active: true}
}

func (e *mergeEditor) insert(text string) {
	inserted := []rune(text)
	tail := append([]rune(nil), e.text[e.cursor:]...)
	e.text = append(e.text[:e.cursor], inserted...)
	e.text = append(e.text, tail...)
	e.cursor += len(inserted)
}

func (e *mergeEditor) lineStart() int {
	start := e.cursor
	for start > 0 && e.text[start-1] != '\n' {
		start--
	}
	return start
}

func (e *mergeEditor) lineEnd() int {
	end := e.cursor
	for end < len(e.text) && e.text[end] != '\n' {
		end++
	}
	return end
}

func (e *mergeEditor) key(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "left":
		e.cursor = max(e.cursor-1, 0)
	case "right":
		e.cursor = min(e.cursor+1, len(e.text))
	case "home", "ctrl+a":
		e.cursor = e.lineStart()
	case "end", "ctrl+e":
		e.cursor = e.lineEnd()
	case "ctrl+home":
		e.cursor = 0
	case "ctrl+end":
		e.cursor = len(e.text)
	case "up", "down":
		e.vertical(msg.String() == "down")
	case "pgup", "pgdown":
		for range 10 {
			e.vertical(msg.String() == "pgdown")
		}
	case "backspace":
		if e.cursor > 0 {
			e.text = append(e.text[:e.cursor-1], e.text[e.cursor:]...)
			e.cursor--
		}
	case "delete":
		if e.cursor < len(e.text) {
			e.text = append(e.text[:e.cursor], e.text[e.cursor+1:]...)
		}
	case "enter", "shift+enter":
		e.insert("\n")
	case "tab":
		e.insert("\t")
	default:
		if msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta|tea.ModSuper) == 0 {
			e.insert(msg.Text)
		}
	}
}

func (e *mergeEditor) vertical(down bool) {
	start, end := e.lineStart(), e.lineEnd()
	column := e.cursor - start
	if down {
		if end == len(e.text) {
			return
		}
		e.cursor = end + 1
		e.cursor = min(e.cursor+column, e.lineEnd())
	} else {
		if start == 0 {
			return
		}
		e.cursor = start - 1
		e.cursor = min(e.lineStart()+column, start-1)
	}
}

func (e *mergeEditor) render(width, height int) string {
	lines := strings.Split(string(e.text), "\n")
	row := strings.Count(string(e.text[:e.cursor]), "\n")
	column := e.cursor - e.lineStart()
	rows := max(height-1, 1)
	e.top = min(e.top, row)
	e.top = max(e.top, row-rows+1)
	body := []string{textfit.TruncateToVisualWidth(fmt.Sprintf("Merged result · line %d/%d · column %d", row+1, len(lines), column+1), width)}
	contentWidth := max(width-7, 1)
	for i := e.top; i < min(e.top+rows, len(lines)); i++ {
		runes := []rune(lines[i])
		start := 0
		if i == row {
			// Keep the caret in view on long lines; tabs are visible single glyphs.
			for start < column && lipgloss.Width(displayRunes(runes[start:column])) >= max(contentWidth-2, 1) {
				start++
			}
		}
		line := displayRunes(runes[start:])
		if i == row {
			prefix := displayRunes(runes[start:column])
			caret, suffix := " ", ""
			if column < len(runes) {
				caret = displayRunes(runes[column : column+1])
				suffix = displayRunes(runes[column+1:])
			}
			line = prefix + lipgloss.NewStyle().Reverse(true).Render(caret) + suffix
		}
		body = append(body, theme.MutedStyle.Render(fmt.Sprintf("%5d ", i+1))+textfit.TruncateToVisualWidth(line, contentWidth))
	}
	return strings.Join(body, "\n")
}

// Only display substitutes control characters; the saved runes are untouched.
func displayRunes(runes []rune) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return '→'
		}
		if r < 32 || r == 127 {
			return utf8.RuneError
		}
		return r
	}, string(runes))
}
