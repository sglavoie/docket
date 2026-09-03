package tui

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestGetSectionColorValueLightTheme(t *testing.T) {
	theme.ActiveThemeName = "light"
	t.Cleanup(func() { theme.ActiveThemeName = "" })

	want := lipgloss.Color("#15803d")
	got := theme.GetSectionColorValue("green")
	if got != want {
		t.Errorf("light theme: GetSectionColorValue(\"green\") = %v, want %v", got, want)
	}
}

func TestGetSectionColorValueDarkTheme(t *testing.T) {
	theme.ActiveThemeName = ""
	t.Cleanup(func() { theme.ActiveThemeName = "" })

	want := lipgloss.Color("#9ece6a")
	got := theme.GetSectionColorValue("green")
	if got != want {
		t.Errorf("dark theme: GetSectionColorValue(\"green\") = %v, want %v", got, want)
	}
}
