package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderFullWidth renders the outliner tree using the full terminal width.
//
// The content is wrapped and then defensively fitted to m.width. Lip Gloss'
// width wrapper handles ordinary long headers, but some styled/wide-glyph
// boundary cases can still produce a line one cell over budget; the final pass
// keeps the full-width surface inside the terminal.
func (m *OutlinerModel) renderFullWidth() string {
	rendered := lipgloss.NewStyle().Width(m.width).Render(m.renderOutlinerTreeContent(m.width))
	return fitRenderedBlockToWidth(rendered, m.width)
}

func fitRenderedBlockToWidth(content string, width int) string {
	if width <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			line = truncateIntentional(line, width, "")
		}
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
