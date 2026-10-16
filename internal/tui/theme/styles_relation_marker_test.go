package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRelationMarkerIsQuietAndReadable(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })
	for _, name := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: name})
		style := RelationMarkerStyle
		if style.GetBold() || style.GetBackground() != (lipgloss.NoColor{}) {
			t.Fatal("structural relation marker competes with urgency chips")
		}
		if ratio := dueBadgeContrast(style.GetForeground(), ColorBackground); ratio < 4.5 {
			t.Fatalf("%s relation contrast %.2f", name, ratio)
		}
	}
}
