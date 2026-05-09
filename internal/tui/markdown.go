package tui

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
)

// renderMarkdown renders the given text as styled terminal markdown using glamour.
// Falls back to raw text on error.
func renderMarkdown(text string, width int) string {
	style := styles.DarkStyleConfig
	if activeThemeName == "light" {
		style = styles.LightStyleConfig
	}
	noMargin := uint(0)
	style.Document.Margin = &noMargin

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}

	rendered, err := r.Render(text)
	if err != nil {
		return text
	}

	return strings.TrimSpace(rendered)
}
