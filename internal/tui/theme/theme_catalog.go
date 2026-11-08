package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
)

// ActivePaletteName is the machine theme the session is painting, when it is
// one of the palette themes rather than the two built-ins: a curated name, or
// "terminal" once the terminal reported its colors. It is empty under the
// built-ins and under a host's palette, which names itself. ActiveThemeName
// keeps answering the light-or-dark question every style rebuild asks.
var ActivePaletteName string

// ThemeChoice is one theme a machine may choose, in menu order.
type ThemeChoice struct {
	ID    string
	Label string
}

// ThemeChoices lists the built-ins, the curated palettes, and the terminal's
// own colors, in the order the View menu draws them.
func ThemeChoices() []ThemeChoice {
	return []ThemeChoice{
		{ID: config.ThemeDark, Label: "Dark"},
		{ID: config.ThemeLight, Label: "Light"},
		{ID: config.ThemeEmber, Label: "Ember"},
		{ID: config.ThemeFjord, Label: "Fjord"},
		{ID: config.ThemeSepia, Label: "Sepia"},
		{ID: config.ThemeTerminal, Label: "Terminal Colors"},
	}
}

// curatedPalettes are Docket's own sixteen-color palettes. Each is written as a
// terminal palette and mapped through paletteRoles, so a curated theme and the
// terminal's own colors take one road to the screen: legibility, cursor depth,
// and semantic accents are derived, never hand-tuned per theme.
var curatedPalettes = map[string]GhosttyPalette{
	// Ember is a warm dark: charcoal paper, parchment ink, and accents fired
	// toward red and amber.
	config.ThemeEmber: {
		Name: "Ember", Background: "#1c1714", Foreground: "#e8dccb",
		Cursor: "#e5a84b", CursorText: "#1c1714",
		SelectionBackground: "#3a2e26", SelectionForeground: "#f4ead9",
		Palette: []string{
			"#2a221d", "#e06c5a", "#9fb36a", "#e5a84b", "#7f9fc4", "#c48bb8", "#6fb3a6", "#cdbfae",
			"#5a4b40", "#f08a76", "#b7cb82", "#f2c06a", "#9cb8da", "#d8a3cc", "#8ccabd", "#f4ead9",
		},
	},
	// Fjord is a cool dark: slate paper and glacier blues, for long sessions
	// under cold light.
	config.ThemeFjord: {
		Name: "Fjord", Background: "#161b22", Foreground: "#d4dde8",
		Cursor: "#6fa8e8", CursorText: "#161b22",
		SelectionBackground: "#2a3442", SelectionForeground: "#eef3f8",
		Palette: []string{
			"#222a35", "#e07a86", "#8fc79a", "#e2c27a", "#6fa8e8", "#b49be0", "#5fc4cf", "#b8c4d2",
			"#4a5668", "#f0919c", "#a6d9b0", "#efd394", "#8dbdf2", "#c8b3ee", "#7ed6df", "#eef3f8",
		},
	},
	// Sepia is a warm light: unbleached paper and ink-brown text, gentler than
	// the white of Light.
	config.ThemeSepia: {
		Name: "Sepia", Background: "#f4ecdd", Foreground: "#3b3128",
		Cursor: "#3b6a9a", CursorText: "#f4ecdd",
		SelectionBackground: "#e2d4bb", SelectionForeground: "#2a221b",
		Palette: []string{
			"#e6dccb", "#b8473a", "#5e7a2c", "#a5701c", "#3b6a9a", "#8a4f86", "#2f7f78", "#5c4e40",
			"#a89a86", "#c95e4f", "#6f8c3a", "#b98427", "#4b7cae", "#9c6198", "#3c918a", "#2a221b",
		},
	},
}

// terminalPalette is the palette the terminal last reported this session, nil
// until it answers. It is session-only, as a host's palette is: the machine
// stores the choice "terminal", never the colors, which belong to whichever
// terminal the next launch runs in.
var terminalPalette *GhosttyPalette

// ApplyNamedTheme paints the machine theme name: a built-in, a curated
// palette, or the terminal's reported colors. Terminal colors not yet reported
// paint Dark meanwhile, and report false so the caller knows to ask for them.
// An unknown name paints Dark, as ApplyTheme always has.
func ApplyNamedTheme(name string) bool {
	switch name {
	case config.ThemeLight:
		ActivePaletteName = ""
		ApplyTheme(ThemeConfig{Name: "light"})
	case config.ThemeTerminal:
		if terminalPalette == nil {
			ActivePaletteName = ""
			ApplyTheme(ThemeConfig{Name: "dark"})
			return false
		}
		ActivePaletteName = config.ThemeTerminal
		applyPalette(*terminalPalette)
	default:
		if palette, ok := curatedPalettes[name]; ok {
			ActivePaletteName = name
			applyPalette(palette)
			return true
		}
		ActivePaletteName = ""
		ApplyTheme(ThemeConfig{Name: "dark"})
	}
	return true
}

// ApplyTerminalPalette records the colors the terminal reported and paints
// them. A palette that fails validation is refused and nothing changes.
func ApplyTerminalPalette(p GhosttyPalette) error {
	if err := p.Validate(); err != nil {
		return err
	}
	terminalPalette = &p
	ActivePaletteName = config.ThemeTerminal
	applyPalette(p)
	return nil
}

// ResetTerminalPalette forgets the reported colors, for tests that paint the
// terminal theme from a clean session.
func ResetTerminalPalette() {
	terminalPalette = nil
}

// ActiveMachineTheme names the machine theme the session is painting, as the
// View menu's radio rows read it.
func ActiveMachineTheme() string {
	if ActivePaletteName != "" {
		return ActivePaletteName
	}
	if ActiveThemeName == "light" {
		return config.ThemeLight
	}
	return config.ThemeDark
}

// TerminalThemeName is the machine theme that paints the terminal's own colors.
const TerminalThemeName = config.ThemeTerminal

// The terminal's own color reports, read for Terminal Colors.

// XtermDefaultPalette fills the ANSI slots a terminal did not report.
var XtermDefaultPalette = [16]string{
	"#000000", "#cd0000", "#00cd00", "#cdcd00", "#0000ee", "#cd00cd", "#00cdcd", "#e5e5e5",
	"#7f7f7f", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff",
}

// ParsePaletteReport reads one OSC 4 answer, `ESC ] 4 ; N ; rgb:RRRR/GGGG/BBBB`
// ended by BEL or ST, into its slot and a #rrggbb color.
func ParsePaletteReport(raw string) (int, string, bool) {
	body := strings.TrimPrefix(raw, "\x1b]")
	body = strings.TrimSuffix(strings.TrimSuffix(body, "\x07"), "\x1b\\")
	parts := strings.SplitN(body, ";", 3)
	if len(parts) != 3 || parts[0] != "4" {
		return 0, "", false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index > 15 {
		return 0, "", false
	}
	parsed := ansi.XParseColor(parts[2])
	if parsed == nil {
		return 0, "", false
	}
	return index, ColorHex(parsed), true
}

// ColorHex spells a color as #rrggbb, or "" for none.
func ColorHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// TerminalPalette assembles the palette the terminal reported: its paper, ink,
// and sixteen ANSI colors, the slots it left unreported filled from xterm's
// defaults, and a selection blended from paper toward ink.
func TerminalPalette(bg, fg string, reported [16]string) GhosttyPalette {
	palette := make([]string, 16)
	for index, hex := range reported {
		palette[index] = hex
		if hex == "" {
			palette[index] = XtermDefaultPalette[index]
		}
	}
	return GhosttyPalette{
		Name: "Terminal", Background: bg, Foreground: fg,
		Cursor: fg, CursorText: bg,
		SelectionBackground: blendHex(bg, fg, 0.22), SelectionForeground: fg,
		Palette: palette,
	}
}
