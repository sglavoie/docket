package tui

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGetSectionColorValueLightTheme(t *testing.T) {
	activeThemeName = "light"
	t.Cleanup(func() { activeThemeName = "" })

	want := lipgloss.Color("#15803d")
	got := GetSectionColorValue("green")
	if got != want {
		t.Errorf("light theme: GetSectionColorValue(\"green\") = %v, want %v", got, want)
	}
}

func TestGetSectionColorValueDarkTheme(t *testing.T) {
	activeThemeName = ""
	t.Cleanup(func() { activeThemeName = "" })

	want := lipgloss.Color("#9ece6a")
	got := GetSectionColorValue("green")
	if got != want {
		t.Errorf("dark theme: GetSectionColorValue(\"green\") = %v, want %v", got, want)
	}
}
