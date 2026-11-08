package tui

import (
	"fmt"
	"image/color"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestParsePaletteReportReadsOSC4(t *testing.T) {
	index, hex, ok := theme.ParsePaletteReport("\x1b]4;12;rgb:5c5c/5c5c/ffff\x07")
	if !ok || index != 12 || hex != "#5c5cff" {
		t.Fatalf("parsed (%d, %q, %v)", index, hex, ok)
	}
	if _, _, ok := theme.ParsePaletteReport("\x1b]4;16;rgb:0000/0000/0000\x1b\\"); ok {
		t.Fatal("a slot past 15 parsed")
	}
	if _, _, ok := theme.ParsePaletteReport("\x1b]52;c;aGk=\x07"); ok {
		t.Fatal("a clipboard report parsed as a palette color")
	}
}

// Choosing Terminal Colors asks the terminal, and its answers repaint the
// session in its own paper once all eighteen have arrived.
func TestTerminalThemePaintsTheReportedPalette(t *testing.T) {
	t.Cleanup(func() {
		theme.ResetTerminalPalette()
		theme.ApplyNamedTheme(config.ThemeDark)
	})
	theme.ResetTerminalPalette()
	driver := launchReadmeFixture(t)
	app := driver.App()
	query, err := app.coreSetTheme(config.ThemeTerminal)
	if err != nil || query == nil {
		t.Fatalf("choosing terminal colors = (%v, %v), want a query", query, err)
	}
	if theme.ActivePaletteName != "" {
		t.Fatalf("the palette painted before the terminal answered: %q", theme.ActivePaletteName)
	}
	app.handleTerminalThemeMessage(tea.BackgroundColorMsg{Color: color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff}})
	app.handleTerminalThemeMessage(tea.ForegroundColorMsg{Color: color.RGBA{R: 0xee, G: 0xee, B: 0xdd, A: 0xff}})
	for index := range 16 {
		app.handleTerminalThemeMessage(uv.UnknownOscEvent(fmt.Sprintf("\x1b]4;%d;rgb:8080/%02x%02x/4040\x07", index, index*8, index*8)))
	}
	if theme.ActivePaletteName != config.ThemeTerminal || theme.ThemeBgColor != "#102030" {
		t.Fatalf("after the answers: palette %q, paper %q", theme.ActivePaletteName, theme.ThemeBgColor)
	}
	if theme.ActiveMachineTheme() != config.ThemeTerminal {
		t.Fatalf("the View menu would tick %q", theme.ActiveMachineTheme())
	}
}

// A terminal that never answers keeps Dark and says so once.
func TestTerminalThemeTimeoutKeepsDark(t *testing.T) {
	t.Cleanup(func() { theme.ApplyNamedTheme(config.ThemeDark) })
	theme.ResetTerminalPalette()
	driver := launchReadmeFixture(t)
	app := driver.App()
	if _, err := app.coreSetTheme(config.ThemeTerminal); err != nil {
		t.Fatal(err)
	}
	app.handleTerminalThemeMessage(terminalThemeTimeoutMsg{seq: app.terminalTheme.seq})
	if theme.ActivePaletteName != "" || theme.ActiveThemeName != "dark" {
		t.Fatalf("a silent terminal painted %q/%q", theme.ActivePaletteName, theme.ActiveThemeName)
	}
	if got := app.feedback.message; got != "This terminal did not report its colors; showing Dark" {
		t.Fatalf("status = %q", got)
	}
}

// Every curated theme is a valid palette and paints its own paper.
func TestCuratedThemesPaint(t *testing.T) {
	t.Cleanup(func() { theme.ApplyNamedTheme(config.ThemeDark) })
	for _, name := range []string{config.ThemeEmber, config.ThemeFjord, config.ThemeSepia} {
		if !theme.ApplyNamedTheme(name) || theme.ActiveMachineTheme() != name {
			t.Fatalf("%s did not paint: active %q", name, theme.ActiveMachineTheme())
		}
	}
	if theme.ActiveThemeName != "light" {
		t.Fatalf("Sepia's paper read as %q, want light", theme.ActiveThemeName)
	}
}
