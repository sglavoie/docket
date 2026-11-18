package tui

import (
	"hash/fnv"
	"image/color"
	"math"
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
	// selection is a teal of its own, apart from the cursor by more than the
	// cursor is from the pane, so a selected row is not taken for the cursor.
	cursor, selection string
	// header is the background the focused pane's header is shaded with: a hue
	// of its own, so it is not taken for the cursor on the first row.
	header string
	// flash is the background a copy lights what it took with, for a moment.
	flash string
	// dimSelected is dim on the selection background, which is lighter in the
	// dark palette and darker in the light one than dim can be read on.
	dimSelected string
	// ages runs from an age just begun to the oldest: the text colour, pale
	// yellow, yellow, orange and red, each readable on the pane's background.
	ages [5][3]float64
	// ground is the background the palette is drawn for until the terminal
	// names its own; Docket.app paints it when no theme is chosen.
	ground [3]float64
	// The four backgrounds above are mixed from the terminal's background,
	// so they sit on any theme's: the cursor a step toward the text colour,
	// the selection and a copy's flash toward the accent, the header toward
	// an indigo.
	cursorMix, selectionMix, flashMix, headerMix mix
}

// mix is how far a background is moved toward a colour.
type mix struct {
	toward [3]float64
	by     float64
}

// over is the background mixed over ground, eased back toward ground until
// each colour in keep reads on it at its ratio, though never below a third of
// the way, so the shade still shows on a theme near the text colour. Where
// the pane itself falls short of a ratio, the shade need only nearly match it.
func (x mix) over(ground [3]float64, keep ...readable) string {
	var c [3]float64
	for by := x.by; ; by -= 0.01 {
		for i := range c {
			c[i] = math.Round(ground[i] + (x.toward[i]-ground[i])*by)
		}
		if by <= x.by/3 || allRead(c, ground, keep) {
			break
		}
	}
	return "\x1b[48;2;" + strconv.Itoa(int(c[0])) + ";" + strconv.Itoa(int(c[1])) + ";" + strconv.Itoa(int(c[2])) + "m"
}

// readable is a colour drawn on a background and the contrast it needs there.
type readable struct {
	color [3]float64
	ratio float64
}

func allRead(bg, ground [3]float64, keep []readable) bool {
	for _, k := range keep {
		if contrastOf(k.color, bg) < min(k.ratio, 0.95*contrastOf(k.color, ground)) {
			return false
		}
	}
	return true
}

// contrastOf is the WCAG contrast ratio between two colours.
func contrastOf(a, b [3]float64) float64 {
	lum := func(c [3]float64) float64 {
		var l [3]float64
		for i, v := range c {
			v /= 255
			if v <= 0.03928 {
				l[i] = v / 12.92
			} else {
				l[i] = math.Pow((v+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
	}
	x, y := lum(a), lum(b)
	return (max(x, y) + 0.05) / (min(x, y) + 0.05)
}

// rgbOfSGR reads the colour back out of a true-colour sequence.
func rgbOfSGR(sgr string) [3]float64 {
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(sgr, "\x1b["), "m"), ";")
	var c [3]float64
	if len(parts) == 5 {
		for i := range c {
			n, _ := strconv.Atoi(parts[i+2])
			c[i] = float64(n)
		}
	}
	return c
}

// on is the palette drawn over a terminal background. Text keeps 7 to 1 on
// the cursor, the selection and the header, dim text 4.5 to 1 on the
// selection and the header, and text 4.5 to 1 on a copy's flash, as far as
// the theme's background allows.
func (p palette) on(ground [3]float64) palette {
	text, dim, dimSelected := rgbOfSGR(p.text), rgbOfSGR(p.dim), rgbOfSGR(p.dimSelected)
	p.ground = ground
	p.cursor = p.cursorMix.over(ground, readable{text, 7})
	p.selection = p.selectionMix.over(ground, readable{text, 7}, readable{dimSelected, 4.5})
	p.flash = p.flashMix.over(ground, readable{text, 4.5})
	p.header = p.headerMix.over(ground, readable{text, 7}, readable{dim, 4.5})
	return p
}

// Bubble Tea's renderer converts these RGB roles to its detected color
// profile (including NO_COLOR); the model never guesses terminal capabilities.
var darkPalette = palette{
	text: "\x1b[38;2;220;225;231m", dim: "\x1b[38;2;146;158;173m",
	accent: "\x1b[38;2;105;199;211m", warn: "\x1b[38;2;230;188;100m",
	alert:        "\x1b[38;2;239;135;135m",
	dimSelected:  "\x1b[38;2;176;188;201m",
	ages:         [5][3]float64{{220, 225, 231}, {214, 203, 150}, {232, 196, 84}, {240, 146, 66}, {242, 86, 78}},
	cursorMix:    mix{[3]float64{220, 225, 231}, 0.07},
	selectionMix: mix{[3]float64{105, 199, 211}, 0.28},
	flashMix:     mix{[3]float64{105, 199, 211}, 0.45},
	headerMix:    mix{[3]float64{140, 120, 240}, 0.25},
}.on([3]float64{20, 26, 32})

var lightPalette = palette{
	text: "\x1b[38;2;35;45;56m", dim: "\x1b[38;2;91;105;119m",
	accent: "\x1b[38;2;0;105;122m", warn: "\x1b[38;2;130;86;0m",
	alert:        "\x1b[38;2;177;49;54m",
	dimSelected:  "\x1b[38;2;75;89;103m",
	ages:         [5][3]float64{{35, 45, 56}, {105, 92, 40}, {130, 94, 0}, {166, 74, 0}, {180, 38, 38}},
	cursorMix:    mix{[3]float64{35, 45, 56}, 0.08},
	selectionMix: mix{[3]float64{0, 105, 122}, 0.25},
	flashMix:     mix{[3]float64{0, 105, 122}, 0.40},
	headerMix:    mix{[3]float64{120, 80, 240}, 0.12},
}.on([3]float64{251, 252, 253})

// paint takes the terminal's background: light or dark picks the palette,
// and its backgrounds are mixed from the colour itself.
func (m *Model) paint(bg color.Color, dark bool) {
	m.lightPalette = !dark
	m.palette = darkPalette
	if m.lightPalette {
		m.palette = lightPalette
	}
	if bg != nil {
		r, g, b, _ := bg.RGBA()
		m.palette = m.palette.on([3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)})
	}
}

func (m *Model) colors() palette {
	if m.palette.text == "" {
		return darkPalette
	}
	return m.palette
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
	"red":     {fg(234, 92, 132), fg(206, 30, 78)},
	"orange":  {fg(234, 162, 96), fg(162, 90, 36)},
	"yellow":  {fg(252, 228, 132), fg(126, 114, 12)},
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
