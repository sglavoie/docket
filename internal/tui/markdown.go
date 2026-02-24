package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown renders the given text as styled terminal markdown using glamour.
// Falls back to raw text on error.
func renderMarkdown(text string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
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
