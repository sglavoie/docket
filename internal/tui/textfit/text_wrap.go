package textfit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// WrapStyled wraps a string that may contain ANSI escape codes, accounting for
// visual width (not byte length). Use this when input has been styled with
// lipgloss; use WrapText for plain text.
func WrapStyled(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	return strings.Split(ansi.Wrap(s, width, ""), "\n")
}

func hardWrapStyled(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	return strings.Split(ansi.Hardwrap(s, width, true), "\n")
}

// WrapText wraps text to the specified width.
func WrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return nil
	}
	return strings.Split(ansi.Wrap(text, width, ""), "\n")
}

// WrapSegments packs whole segments joined by sep onto each line, so a segment
// such as `slug agent-mux` never splits across two lines. A segment wider than
// the line on its own starts a fresh line and falls back to WrapText.
func WrapSegments(segments []string, sep string, width int) []string {
	if width <= 0 {
		return []string{strings.Join(segments, sep)}
	}
	var lines []string
	current := ""
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if current != "" && ansi.StringWidth(current+sep+segment) <= width {
			current += sep + segment
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		if ansi.StringWidth(segment) <= width {
			current = segment
			continue
		}
		wrapped := WrapText(segment, width)
		lines = append(lines, wrapped[:len(wrapped)-1]...)
		current = wrapped[len(wrapped)-1]
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
