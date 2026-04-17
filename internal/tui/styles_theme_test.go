package tui

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

func TestColorProgressEmptyLightTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	want := lipgloss.Color("#9ca3af")
	if ColorProgressEmpty != want {
		t.Errorf("light theme: ColorProgressEmpty = %v, want %v", ColorProgressEmpty, want)
	}
}

func TestColorProgressEmptyDarkTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// Empty palette → dark ANSI muted fallback "248".
	want := lipgloss.Color("248")
	if ColorProgressEmpty != want {
		t.Errorf("dark theme: ColorProgressEmpty = %v, want %v", ColorProgressEmpty, want)
	}
}

func TestMakeViewModeStyleLightThemeBg(t *testing.T) {
	activeThemeName = "light"
	t.Cleanup(func() { activeThemeName = "" })

	darkANSI := "34"
	lightHex := "#15803d"
	s := makeViewModeStyle(darkANSI, lightHex)

	got := s.GetBackground()
	want := lipgloss.Color(lightHex)
	if got != want {
		t.Errorf("light theme: background = %v, want %v", got, want)
	}
}

func TestMakeViewModeStyleDarkThemeBg(t *testing.T) {
	activeThemeName = ""
	t.Cleanup(func() { activeThemeName = "" })

	darkANSI := "34"
	lightHex := "#15803d"
	s := makeViewModeStyle(darkANSI, lightHex)

	got := s.GetBackground()
	want := lipgloss.Color(darkANSI)
	if got != want {
		t.Errorf("dark theme: background = %v, want %v", got, want)
	}
}

func TestRebuildPaletteLightEmptyUsesDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rebuildPalette(config.PaletteColors{})

	if ColorPrimary != lipgloss.Color("#0366d6") {
		t.Errorf("light empty palette: ColorPrimary = %v, want #0366d6", ColorPrimary)
	}
	if ColorMuted != lipgloss.Color("#4b5563") {
		t.Errorf("light empty palette: ColorMuted = %v, want #4b5563", ColorMuted)
	}
	if colorNormalText != "#111827" {
		t.Errorf("light empty palette: colorNormalText = %q, want #111827", colorNormalText)
	}
}

func TestRebuildPaletteDarkEmptyUsesDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rebuildPalette(config.PaletteColors{})

	if ColorPrimary != lipgloss.Color("39") {
		t.Errorf("dark empty palette: ColorPrimary = %v, want 39", ColorPrimary)
	}
	if ColorMuted != lipgloss.Color("248") {
		t.Errorf("dark empty palette: ColorMuted = %v, want 248", ColorMuted)
	}
	if colorNormalText != "" {
		t.Errorf("dark empty palette: colorNormalText = %q, want empty", colorNormalText)
	}
}

func TestApplyThemeLightEmptyCursorResolvesToDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	if ColorSelectedBg != lipgloss.Color("#c7b6f0") {
		t.Errorf("light empty cursor: ColorSelectedBg = %v, want #c7b6f0", ColorSelectedBg)
	}
	if ColorSelectedFg != lipgloss.Color("#1f1b35") {
		t.Errorf("light empty cursor: ColorSelectedFg = %v, want #1f1b35", ColorSelectedFg)
	}
	if themeBgColor != "#f6f6f0" {
		t.Errorf("light empty bg: themeBgColor = %q, want #f6f6f0", themeBgColor)
	}
}

func TestApplyThemeDarkEmptyCursorResolvesToDefaults(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	if ColorSelectedBg != lipgloss.Color("#4c0163") {
		t.Errorf("dark empty cursor: ColorSelectedBg = %v, want #4c0163", ColorSelectedBg)
	}
	if ColorSelectedFg != lipgloss.Color("#fbeeff") {
		t.Errorf("dark empty cursor: ColorSelectedFg = %v, want #fbeeff", ColorSelectedFg)
	}
	if themeBgColor != "" {
		t.Errorf("dark empty bg: themeBgColor = %q, want empty", themeBgColor)
	}
}

func TestColorSelectedTextFgLightTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "light"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	want := lipgloss.Color("#0f172a")
	if ColorSelectedTextFg != want {
		t.Errorf("light theme: ColorSelectedTextFg = %v, want %v", ColorSelectedTextFg, want)
	}
}

func TestColorSelectedTextFgDarkTheme(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	want := lipgloss.Color("#fbeeff")
	if ColorSelectedTextFg != want {
		t.Errorf("dark theme: ColorSelectedTextFg = %v, want %v", ColorSelectedTextFg, want)
	}
}
