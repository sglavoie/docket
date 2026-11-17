package tui

import (
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
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
	// flash is the background a copy lights what it took with, for a moment.
	flash string
	// dimSelected is dim on the selection background, which is lighter in the
	// dark palette and darker in the light one than dim can be read on.
	dimSelected string
	// ages runs from an age just begun to the oldest: the text colour, pale
	// yellow, yellow, orange and red, each readable on the pane's background.
	ages [5][3]float64
}

// Bubble Tea's renderer converts these RGB roles to its detected color
// profile (including NO_COLOR); the model never guesses terminal capabilities.
var darkPalette = palette{
	text: "\x1b[38;2;220;225;231m", dim: "\x1b[38;2;146;158;173m",
	accent: "\x1b[38;2;105;199;211m", warn: "\x1b[38;2;230;188;100m",
	alert:  "\x1b[38;2;239;135;135m",
	cursor: "\x1b[48;2;38;51;63m", selection: "\x1b[48;2;43;65;76m",
	flash:       "\x1b[48;2;46;92;104m",
	dimSelected: "\x1b[38;2;160;172;187m",
	ages:        [5][3]float64{{220, 225, 231}, {214, 203, 150}, {232, 196, 84}, {240, 146, 66}, {242, 86, 78}},
}

var lightPalette = palette{
	text: "\x1b[38;2;35;45;56m", dim: "\x1b[38;2;91;105;119m",
	accent: "\x1b[38;2;0;105;122m", warn: "\x1b[38;2;130;86;0m",
	alert:  "\x1b[38;2;177;49;54m",
	cursor: "\x1b[48;2;229;237;243m", selection: "\x1b[48;2;207;227;235m",
	flash:       "\x1b[48;2;170;214;224m",
	dimSelected: "\x1b[38;2;85;99;113m",
	ages:        [5][3]float64{{35, 45, 56}, {105, 92, 40}, {130, 94, 0}, {166, 74, 0}, {180, 38, 38}},
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
	if c := m.colors(); background == c.selection || background == c.flash {
		// Dim text keeps a contrast of 4.5 to 1 on a selected row.
		line = strings.ReplaceAll(line, c.dim, c.dimSelected)
	}
	line += strings.Repeat(" ", max(width-ansi.StringWidth(line), 0))
	return base + strings.ReplaceAll(line, sgrReset, sgrReset+base) + sgrReset
}

func (m *Model) paintText(text string) string {
	base := m.colors().text
	return base + strings.ReplaceAll(text, sgrReset, sgrReset+base) + sgrReset
}

// fg is a true-colour foreground.
func fg(r, g, b int) string {
	return "\x1b[38;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m"
}

// namedColors are the shades a colour name is drawn in, each readable on
// the pane's background in its palette: lighter for dark, deeper for light.
var namedColors = map[string][2]string{
	//            dark                 light
	"red":     {fg(240, 113, 113), fg(186, 40, 40)},
	"orange":  {fg(242, 163, 92), fg(172, 82, 0)},
	"yellow":  {fg(232, 208, 96), fg(130, 102, 0)},
	"lime":    {fg(178, 216, 84), fg(88, 118, 0)},
	"green":   {fg(122, 204, 120), fg(30, 122, 48)},
	"teal":    {fg(78, 200, 178), fg(0, 118, 106)},
	"cyan":    {fg(96, 204, 230), fg(0, 112, 148)},
	"blue":    {fg(112, 162, 244), fg(34, 88, 192)},
	"indigo":  {fg(140, 140, 240), fg(72, 64, 176)},
	"purple":  {fg(178, 138, 238), fg(114, 66, 184)},
	"magenta": {fg(222, 116, 222), fg(150, 36, 150)},
	"pink":    {fg(242, 140, 190), fg(178, 46, 110)},
	"brown":   {fg(196, 150, 108), fg(124, 80, 40)},
	"gray":    {fg(164, 170, 180), fg(98, 104, 114)},
}

// tagRamps are the colours a tag is told apart by, in the dark and the light
// palette. Kept apart from the alert red and the flag's warning yellow, so a
// tag never reads as an alarm.
var tagRamps = [2][]string{
	{fg(204, 132, 111), fg(144, 174, 82), fg(69, 165, 114), fg(44, 175, 195),
		fg(135, 124, 183), fg(226, 131, 201), fg(200, 160, 90), fg(110, 150, 230)},
	{fg(161, 96, 88), fg(103, 121, 33), fg(56, 135, 86), fg(37, 134, 167),
		fg(109, 104, 187), fg(180, 78, 126), fg(140, 100, 20), fg(50, 100, 190)},
}

func (m *Model) paletteIndex() int {
	if m.lightPalette {
		return 1
	}
	return 0
}

// colorOf draws a colour the ~ sigil stores: a name in its palette's shade,
// a hex value as it is. Anything else is no colour.
func (m *Model) colorOf(value string) string {
	if shades, ok := namedColors[value]; ok {
		return shades[m.paletteIndex()]
	}
	if hex, ok := strings.CutPrefix(value, "#"); ok && len(hex) == 6 {
		if n, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return fg(int(n>>16), int(n>>8&0xff), int(n&0xff))
		}
	}
	return ""
}

// projectColor is the colour a project is drawn in, or nothing. Only a
// project is: the field is read on any row, and drawn on top-level ones.
func (m *Model) projectColor(r *outline.Row) string {
	if r == nil || r.Parent != "" || hiddenTop(r.ID) {
		return ""
	}
	return m.colorOf(r.Fields[sigil.FieldColor])
}

// tagColor tells one tag from another. The name, without a value, is hashed
// by FNV-1a, which is specified and unseeded, so a tag keeps its colour on
// every machine and across restarts, and a tag added later never repaints
// the others. Two tags sharing a colour is expected: it is a mnemonic.
func (m *Model) tagColor(tag string) string {
	ramp := tagRamps[m.paletteIndex()]
	h := fnv.New32a()
	_, _ = h.Write([]byte(sigil.TagName(tag)))
	return ramp[h.Sum32()%uint32(len(ramp))]
}
