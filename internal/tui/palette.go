package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	sgrReset     = "\x1b[0m"
	sgrBold      = "\x1b[1m"
	sgrReverse   = "\x1b[7m"
	sgrUnderline = "\x1b[4m"
)

type palette struct {
	text, dim, accent, warn, alert string
	cursor, selection              string
}

// Bubble Tea's renderer converts these RGB roles to its detected color
// profile (including NO_COLOR); the model never guesses terminal capabilities.
var darkPalette = palette{
	text: "\x1b[38;2;220;225;231m", dim: "\x1b[38;2;146;158;173m",
	accent: "\x1b[38;2;105;199;211m", warn: "\x1b[38;2;230;188;100m",
	alert:  "\x1b[38;2;239;135;135m",
	cursor: "\x1b[48;2;38;51;63m", selection: "\x1b[48;2;43;65;76m",
}

var lightPalette = palette{
	text: "\x1b[38;2;35;45;56m", dim: "\x1b[38;2;91;105;119m",
	accent: "\x1b[38;2;0;105;122m", warn: "\x1b[38;2;130;86;0m",
	alert:  "\x1b[38;2;177;49;54m",
	cursor: "\x1b[48;2;229;237;243m", selection: "\x1b[48;2;207;227;235m",
}

func (m *Model) colors() palette {
	if m.lightPalette {
		return lightPalette
	}
	return darkPalette
}

// tint restores the background after every styled span, including the padding.
// The final reset prevents the next pane or line from inheriting the tint.
func (m *Model) tint(line string, width int, background string) string {
	if background == "" {
		return line
	}
	base := m.colors().text + background
	line += strings.Repeat(" ", max(width-ansi.StringWidth(line), 0))
	return base + strings.ReplaceAll(line, sgrReset, sgrReset+base) + sgrReset
}

func (m *Model) paintText(text string) string {
	base := m.colors().text
	return base + strings.ReplaceAll(text, sgrReset, sgrReset+base) + sgrReset
}
