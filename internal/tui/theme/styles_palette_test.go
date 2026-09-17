package theme

import (
	"bytes"
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestGetSectionColorValueLightTheme(t *testing.T) {
	ActiveThemeName = "light"
	t.Cleanup(func() { ActiveThemeName = "" })

	want := lipgloss.Color("#15803d")
	got := GetSectionColorValue("green")
	if got != want {
		t.Errorf("light theme: GetSectionColorValue(\"green\") = %v, want %v", got, want)
	}
}

func TestGetSectionColorValueDarkTheme(t *testing.T) {
	ActiveThemeName = ""
	t.Cleanup(func() { ActiveThemeName = "" })

	want := lipgloss.Color("#9ece6a")
	got := GetSectionColorValue("green")
	if got != want {
		t.Errorf("dark theme: GetSectionColorValue(\"green\") = %v, want %v", got, want)
	}
}

// TestCursorBarSurvivesAProfileWithNoColor is why the bar is inverse video. The
// row's band, its badges and its deadline colors all disappear under
// --no-color, so the one mark that says where the cursor is has to be an
// attribute rather than a color: SGR 7 is what a profile that drops every color
// still forwards, and without it the cursor's column arrives as an ordinary
// blank.
func TestCursorBarSurvivesAProfileWithNoColor(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		t.Run(name, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: name})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

			for label, style := range map[string]lipgloss.Style{
				"resting": CursorBarStyle,
				"flash":   CursorFlashBarStyle,
			} {
				mono := monochrome(t, style.Render(" "))
				if !strings.Contains(mono, "\x1b[7m") {
					t.Fatalf("%s cursor bar lost its inverse-video mark without color: %q", label, mono)
				}
				if ansi.Strip(mono) != " " {
					t.Fatalf("%s cursor bar spent a column on something other than its blank: %q", label, mono)
				}
			}
		})
	}
}

// TestCursorRowBandStandsApartFromEveryNeighborColor holds the band to the one
// thing it has to be: distinguishable. It sits under the row's own text, so it
// must not be the paper it is drawn on, and it must not be the selection fill or
// the trail either, since those two say something else entirely.
func TestCursorRowBandStandsApartFromEveryNeighborColor(t *testing.T) {
	for _, name := range []string{"dark", "light"} {
		t.Run(name, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: name})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

			for label, other := range map[string]color.Color{
				"the paper":        ColorBackground,
				"the selection":    ColorSelectedBg,
				"the cursor trail": ColorSelectedDimBg,
				"the cursor bar":   ColorCursorBarBg,
				"the overdue tint": OverdueRowStyle.GetBackground(),
			} {
				if ColorCursorRowBg == other {
					t.Fatalf("the cursor row band is indistinguishable from %s: %v", label, other)
				}
			}
		})
	}
}

// monochrome is what a terminal that paints nothing forwards.
func monochrome(t *testing.T, styled string) string {
	t.Helper()
	var out bytes.Buffer
	writer := colorprofile.Writer{Forward: &out, Profile: colorprofile.ASCII}
	if _, err := writer.Write([]byte(styled)); err != nil {
		t.Fatalf("monochrome write: %v", err)
	}
	return out.String()
}
