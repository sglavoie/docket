package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// applyBackground fills every line of content with the configured TUI
// background color so no terminal default leaks through. Inner lipgloss
// styles emit SGR resets (\x1b[m or \x1b[0m) that clear the background
// mid-line, so we re-inject the background escape after every reset and
// pad each line to the full terminal width.
//
// When a NormalText palette color is configured, the default foreground is
// also injected alongside the background so unstyled text (e.g. plain
// numeric counts in section headers) renders with the theme's body color
// instead of the terminal's default foreground.
//
// lipgloss v2 emits \x1b[m (empty-param reset) rather than \x1b[0m, so
// both forms must be handled.
func (m *AppModel) applyBackground(content string) string {
	bg := themeBgColor
	if bg == "" {
		return content
	}
	bgEsc := hexToBgEsc(bg)
	if bgEsc == "" {
		return content
	}
	fillEsc := bgEsc + hexToFgEsc(colorNormalText)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Re-apply background (and normal foreground, when set) after every
		// SGR reset within the line. lipgloss v2 emits \x1b[m (empty-param);
		// handle \x1b[0m too.
		patched := fillEsc + strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+fillEsc)
		patched = strings.ReplaceAll(patched, "\x1b[0m", "\x1b[0m"+fillEsc)
		// Pad to full terminal width
		pad := m.width - lipgloss.Width(line)
		if pad > 0 {
			patched += strings.Repeat(" ", pad)
		}
		lines[i] = patched + "\x1b[m"
	}
	// Fill remaining vertical space with background-colored empty lines
	emptyLine := bgEsc + strings.Repeat(" ", m.width) + "\x1b[0m"
	for len(lines) < m.height {
		lines = append(lines, emptyLine)
	}
	return strings.Join(lines, "\n")
}

// hexToBgEsc converts a #rrggbb hex color to an ANSI true-color background
// escape sequence.
func hexToBgEsc(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// hexToFgEsc converts a #rrggbb hex color to an ANSI true-color foreground
// escape sequence. Returns an empty string for unset or malformed input.
func hexToFgEsc(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}
