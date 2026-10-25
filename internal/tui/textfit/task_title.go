package textfit

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// RenderTaskTitle colors paired backtick spans without changing the title's
// text or width. Render before clipping or wrapping so a closing delimiter
// outside the viewport still gives the visible code its color. Search matches
// take precedence, and code inherits lifecycle styling such as strikethrough.
func RenderTaskTitle(text string, positions []int, base lipgloss.Style) string {
	if !strings.Contains(text, "`") {
		return HighlightText(text, positions, base)
	}
	runes := []rune(text)
	code := make([]bool, len(runes))
	for i := 0; i < len(runes); {
		if runes[i] == '\\' && i+1 < len(runes) {
			i += 2
			continue
		}
		if runes[i] != '`' {
			i++
			continue
		}
		start := i
		for i < len(runes) && runes[i] == '`' {
			i++
		}
		for j := i; j < len(runes); {
			if runes[j] != '`' {
				j++
				continue
			}
			end := j
			for end < len(runes) && runes[end] == '`' {
				end++
			}
			if end-j == i-start {
				for k := start; k < end; k++ {
					code[k] = true
				}
				i = end
				break
			}
			j = end
		}
	}
	matches := make([]bool, len(runes))
	for _, position := range positions {
		if position >= 0 && position < len(matches) {
			matches[position] = true
		}
	}
	codeStyle := base.Foreground(theme.ColorAccent)
	var result strings.Builder
	for start := 0; start < len(runes); {
		end := start + 1
		for end < len(runes) && code[end] == code[start] && matches[end] == matches[start] {
			end++
		}
		style := base
		if code[start] {
			style = codeStyle
		}
		if matches[start] {
			style = theme.HighlightStyle.Inherit(base)
		}
		result.WriteString(style.Render(string(runes[start:end])))
		start = end
	}
	return result.String()
}
