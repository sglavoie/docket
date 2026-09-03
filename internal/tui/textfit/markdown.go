package textfit

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// RenderMarkdown renders the given text as styled terminal markdown using glamour.
// Falls back to raw text on error. Output is always bounded to width: Glamour word
// wraps and table wraps, and boundToWidth is a final ANSI-aware guard that re-wraps
// any remaining over-wide lines (e.g. wide tables) without stripping styling.
func RenderMarkdown(text string, width int) string {
	style := styles.DarkStyleConfig
	if theme.ActiveThemeName == "light" {
		style = styles.LightStyleConfig
	}
	noMargin := uint(0)
	style.Document.Margin = &noMargin

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		glamour.WithTableWrap(true),
	)
	if err != nil {
		return boundToWidth(text, width)
	}

	rendered, err := r.Render(text)
	if err != nil {
		return boundToWidth(text, width)
	}

	return strings.TrimSpace(boundToWidth(rendered, width))
}

// boundToWidth guarantees every line of s fits within width, measuring visual
// (ANSI-aware) width. Lines already within width pass through untouched so normal
// paragraphs are not re-fragmented. Over-wide lines are word-wrapped first, then
// any residual fragments (e.g. long Markdown separator runs) are hard-wrapped.
func boundToWidth(s string, width int) string {
	if width <= 0 || s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if lipgloss.Width(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}
		for _, wordWrapped := range WrapStyled(line, width) {
			if lipgloss.Width(wordWrapped) <= width {
				wrapped = append(wrapped, wordWrapped)
				continue
			}
			wrapped = append(wrapped, hardWrapStyled(wordWrapped, width)...)
		}
	}
	return strings.Join(wrapped, "\n")
}
