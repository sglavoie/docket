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

func TestProjectBadgeColorNoPaletteCollision(t *testing.T) {
	// Colors used by score_badge.go and shared_render.go that must not appear.
	forbidden := map[string]bool{
		"196": true,
		"208": true,
		"226": true,
		"118": true,
		"42":  true,
	}
	for _, c := range projectColorPalette {
		if forbidden[c] {
			t.Errorf("palette color %q collides with score_badge.go / shared_render.go colors", c)
		}
	}
}

func TestProjectBadgeColorInPalette(t *testing.T) {
	paletteSet := make(map[string]bool)
	for _, c := range projectColorPalette {
		paletteSet[c] = true
	}
	names := []string{"alpha", "beta", "gamma", "tt", "work", "home", "side", "hobby"}
	for _, name := range names {
		c := projectBadgeColor(name)
		if !paletteSet[c] {
			t.Errorf("projectBadgeColor(%q) = %q, not in palette", name, c)
		}
	}
}
