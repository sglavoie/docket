package theme

import (
	"charm.land/lipgloss/v2"
	"testing"
)

func TestLaneHeadingPalette(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })
	for _, name := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: name})
		for _, today := range []bool{false, true} {
			style := LaneHeaderStyle(today)
			want := NormalStyle.GetForeground()
			if today {
				want = ColorAccent
			}
			if style.GetForeground() != want || style.GetBackground() != (lipgloss.NoColor{}) || !style.GetBold() {
				t.Fatalf("%s today=%v: heading must be bold primary text (Today accent) without fill: %v", name, today, style)
			}
		}
	}
}
