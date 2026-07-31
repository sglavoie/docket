package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// normalizeScreen strips ANSI escape sequences from raw rendered content and
// validates the result against the terminal dimensions. It returns the plain
// (ANSI-free) screen plus any layout issues found:
//
//   - invalid_utf8: the plain screen is not valid UTF-8
//   - width_overflow: a line's visual width exceeds the terminal width
//   - height_overflow: the screen has more rows than the terminal height
//
// width and height of zero (or negative) disable the corresponding check.
func normalizeScreen(rawContent string, width, height int) (plain string, issues []UITestIssue) {
	// Validate UTF-8 on the raw content first: ansi.Strip sanitizes invalid
	// bytes, so checking the stripped result would never surface a real problem.
	if !utf8.ValidString(rawContent) {
		issues = append(issues, UITestIssue{
			Kind:    "invalid_utf8",
			Message: "screen contains invalid UTF-8",
		})
	}

	plain = ansi.Strip(rawContent)

	lines := strings.Split(plain, "\n")

	if height > 0 && len(lines) > height {
		issues = append(issues, UITestIssue{
			Kind:    "height_overflow",
			Message: fmt.Sprintf("rendered %d lines exceeds terminal height %d", len(lines), height),
		})
	}

	if width > 0 {
		for i, line := range lines {
			if w := lipgloss.Width(line); w > width {
				issues = append(issues, UITestIssue{
					Kind:    "width_overflow",
					Line:    i + 1,
					Message: fmt.Sprintf("line %d visual width %d exceeds terminal width %d", i+1, w, width),
				})
			}
		}
	}

	return plain, issues
}
