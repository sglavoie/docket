package tui

import (
	"testing"
)

func TestProjectBadgeColorDeterministic(t *testing.T) {
	names := []string{"alpha", "beta", "my-project", "tt", "", "Project With Spaces"}
	for _, name := range names {
		c1 := projectBadgeColor(name)
		c2 := projectBadgeColor(name)
		if c1 != c2 {
			t.Errorf("projectBadgeColor(%q) not deterministic: got %q then %q", name, c1, c2)
		}
	}
}

func TestProjectBadgeColorPalettesAreHex(t *testing.T) {
	for i, c := range projectColorPaletteDark {
		if len(c) == 0 || c[0] != '#' {
			t.Errorf("projectColorPaletteDark[%d] = %q, expected hex (#rrggbb)", i, c)
		}
	}
	for i, c := range projectColorPaletteLight {
		if len(c) == 0 || c[0] != '#' {
			t.Errorf("projectColorPaletteLight[%d] = %q, expected hex (#rrggbb)", i, c)
		}
	}
}

func TestProjectBadgeColorInPalette(t *testing.T) {
	names := []string{"alpha", "beta", "gamma", "tt", "work", "home", "side", "hobby"}

	darkSet := make(map[string]bool)
	for _, c := range projectColorPaletteDark {
		darkSet[c] = true
	}
	lightSet := make(map[string]bool)
	for _, c := range projectColorPaletteLight {
		lightSet[c] = true
	}

	activeThemeName = ""
	for _, name := range names {
		c := projectBadgeColor(name)
		if !darkSet[c] {
			t.Errorf("dark theme: projectBadgeColor(%q) = %q, not in dark palette", name, c)
		}
	}

	activeThemeName = "light"
	for _, name := range names {
		c := projectBadgeColor(name)
		if !lightSet[c] {
			t.Errorf("light theme: projectBadgeColor(%q) = %q, not in light palette", name, c)
		}
	}
	activeThemeName = ""
}
