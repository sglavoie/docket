package tui

import (
	"strings"

	"charm.land/glamour/v2"
)

// renderMarkdown renders the given text as styled terminal markdown using glamour.
// Falls back to raw text on error.
func renderMarkdown(text string, width int) string {
	stylePath := "dark"
	if activeThemeName == "light" {
		stylePath = "light"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(stylePath),
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
