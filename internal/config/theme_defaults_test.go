package config

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestDefaultTheme_DispatchesToVariant(t *testing.T) {
	dark := DefaultTheme("dark")
	light := DefaultTheme("light")

	if dark == light {
		t.Error("DefaultTheme(dark) and DefaultTheme(light) must not be equal")
	}

	if dark != DefaultDarkTheme() {
		t.Error("DefaultTheme(dark) must equal DefaultDarkTheme()")
	}
	if light != DefaultLightTheme() {
		t.Error("DefaultTheme(light) must equal DefaultLightTheme()")
	}
}

func TestDefaultTheme_UnknownVariantReturnsDark(t *testing.T) {
	got := DefaultTheme("unknown")
	want := DefaultDarkTheme()
	if got != want {
		t.Errorf("DefaultTheme(unknown) = %+v, want dark defaults", got)
	}
}

func TestDefaultDarkTheme_KeyFields(t *testing.T) {
	d := DefaultDarkTheme()

	cases := []struct{ field, got, want string }{
		{"Primary", d.Palette.Primary, "#7aa2f7"},
		{"Secondary", d.Palette.Secondary, "#7dcfff"},
		{"Success", d.Palette.Success, "#9ece6a"},
		{"Warning", d.Palette.Warning, "#e0af68"},
		{"Error", d.Palette.Error, "#f7768e"},
		{"Muted", d.Palette.Muted, "#8f98c7"},
		{"Dimmed", d.Palette.Dimmed, "#565f89"},
		{"DimmedBg", d.Palette.DimmedBg, "#24283b"},
		{"DimmedFg", d.Palette.DimmedFg, "#9aa5ce"},
		{"Border", d.Palette.Border, "#565f89"},
		{"Accent", d.Palette.Accent, "#e0af68"},
		{"NormalText", d.Palette.NormalText, "#c0caf5"},
		{"Priority1Title", d.Palette.Priority1Title, "#f7768e"},
		{"Priority2Title", d.Palette.Priority2Title, "#e0af68"},
		{"Priority3Title", d.Palette.Priority3Title, "#7aa2f7"},
		{"SemanticBugTitle", d.Palette.SemanticBugTitle, "#ff9e9e"},
		{"SemanticEpicTitle", d.Palette.SemanticEpicTitle, "#bb9af7"},
		{"SemanticStoryTitle", d.Palette.SemanticStoryTitle, "#7dcfff"},
		{"SemanticFeatureTitle", d.Palette.SemanticFeatureTitle, "#b9f27c"},
		{"SemanticIdeaTitle", d.Palette.SemanticIdeaTitle, "#ff9e64"},
		{"SemanticBugSelectedTitle", d.Palette.SemanticBugSelectedTitle, "#ffd1d1"},
		{"SemanticEpicSelectedTitle", d.Palette.SemanticEpicSelectedTitle, "#dccbff"},
		{"SemanticStorySelectedTitle", d.Palette.SemanticStorySelectedTitle, "#b6e5ff"},
		{"SemanticFeatureSelectedTitle", d.Palette.SemanticFeatureSelectedTitle, "#d7f7b5"},
		{"SemanticIdeaSelectedTitle", d.Palette.SemanticIdeaSelectedTitle, "#ffd0b5"},
		{"SemanticBugDeferredTitle", d.Palette.SemanticBugDeferredTitle, "#c97b7b"},
		{"SemanticEpicDeferredTitle", d.Palette.SemanticEpicDeferredTitle, "#8d73bd"},
		{"SemanticStoryDeferredTitle", d.Palette.SemanticStoryDeferredTitle, "#5f9bbf"},
		{"SemanticFeatureDeferredTitle", d.Palette.SemanticFeatureDeferredTitle, "#8bb65f"},
		{"SemanticIdeaDeferredTitle", d.Palette.SemanticIdeaDeferredTitle, "#b97852"},
		{"CursorColor", d.CursorColor, "#33467c"},
		{"CursorTextColor", d.CursorTextColor, "#c0caf5"},
		{"BackgroundColor", d.BackgroundColor, "#1a1b26"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("DefaultDarkTheme().%s = %q, want %q", c.field, c.got, c.want)
		}
	}
}

func TestDefaultDarkTheme_ContrastContract(t *testing.T) {
	d := DefaultDarkTheme()

	// Dark foreground contract: readable text gets WCAG AA contrast against the
	// shipped dark background without resorting to pure white.
	checks := []struct {
		name       string
		foreground string
		background string
		min        float64
	}{
		{"Muted", d.Palette.Muted, d.BackgroundColor, 4.5},
		{"NormalText", d.Palette.NormalText, d.BackgroundColor, 4.5},
		{"CursorTextColor", d.CursorTextColor, d.CursorColor, 4.5},
	}
	for _, c := range checks {
		if got := wcagContrastRatio(t, c.foreground, c.background); got < c.min {
			t.Errorf("%s contrast = %.2f:1, want at least %.1f:1 for %s on %s",
				c.name, got, c.min, c.foreground, c.background)
		}
	}
}

func TestDefaultDarkTheme_ForegroundTokensAvoidPureWhite(t *testing.T) {
	d := DefaultDarkTheme()

	foregrounds := map[string]string{
		"NormalText":                   d.Palette.NormalText,
		"Muted":                        d.Palette.Muted,
		"Dimmed":                       d.Palette.Dimmed,
		"DimmedFg":                     d.Palette.DimmedFg,
		"Primary":                      d.Palette.Primary,
		"Secondary":                    d.Palette.Secondary,
		"Accent":                       d.Palette.Accent,
		"Success":                      d.Palette.Success,
		"Warning":                      d.Palette.Warning,
		"Error":                        d.Palette.Error,
		"Border":                       d.Palette.Border,
		"Priority1Title":               d.Palette.Priority1Title,
		"Priority2Title":               d.Palette.Priority2Title,
		"Priority3Title":               d.Palette.Priority3Title,
		"SemanticBugTitle":             d.Palette.SemanticBugTitle,
		"SemanticEpicTitle":            d.Palette.SemanticEpicTitle,
		"SemanticStoryTitle":           d.Palette.SemanticStoryTitle,
		"SemanticFeatureTitle":         d.Palette.SemanticFeatureTitle,
		"SemanticIdeaTitle":            d.Palette.SemanticIdeaTitle,
		"SemanticBugSelectedTitle":     d.Palette.SemanticBugSelectedTitle,
		"SemanticEpicSelectedTitle":    d.Palette.SemanticEpicSelectedTitle,
		"SemanticStorySelectedTitle":   d.Palette.SemanticStorySelectedTitle,
		"SemanticFeatureSelectedTitle": d.Palette.SemanticFeatureSelectedTitle,
		"SemanticIdeaSelectedTitle":    d.Palette.SemanticIdeaSelectedTitle,
		"SemanticBugDeferredTitle":     d.Palette.SemanticBugDeferredTitle,
		"SemanticEpicDeferredTitle":    d.Palette.SemanticEpicDeferredTitle,
		"SemanticStoryDeferredTitle":   d.Palette.SemanticStoryDeferredTitle,
		"SemanticFeatureDeferredTitle": d.Palette.SemanticFeatureDeferredTitle,
		"SemanticIdeaDeferredTitle":    d.Palette.SemanticIdeaDeferredTitle,
		"CursorTextColor":              d.CursorTextColor,
	}
	for name, value := range foregrounds {
		if strings.EqualFold(value, "#ffffff") {
			t.Errorf("%s = %q, dark foreground defaults must avoid pure white", name, value)
		}
	}
}

func TestDefaultLightTheme_AllFieldsPopulated(t *testing.T) {
	l := DefaultLightTheme()

	// All light palette fields must be non-empty (curated hex values).
	fields := map[string]string{
		"NormalText":                   l.Palette.NormalText,
		"Muted":                        l.Palette.Muted,
		"Dimmed":                       l.Palette.Dimmed,
		"DimmedBg":                     l.Palette.DimmedBg,
		"DimmedFg":                     l.Palette.DimmedFg,
		"Primary":                      l.Palette.Primary,
		"Secondary":                    l.Palette.Secondary,
		"Accent":                       l.Palette.Accent,
		"Success":                      l.Palette.Success,
		"Warning":                      l.Palette.Warning,
		"Error":                        l.Palette.Error,
		"Border":                       l.Palette.Border,
		"Priority1Title":               l.Palette.Priority1Title,
		"Priority2Title":               l.Palette.Priority2Title,
		"Priority3Title":               l.Palette.Priority3Title,
		"SemanticBugTitle":             l.Palette.SemanticBugTitle,
		"SemanticEpicTitle":            l.Palette.SemanticEpicTitle,
		"SemanticStoryTitle":           l.Palette.SemanticStoryTitle,
		"SemanticFeatureTitle":         l.Palette.SemanticFeatureTitle,
		"SemanticIdeaTitle":            l.Palette.SemanticIdeaTitle,
		"SemanticBugSelectedTitle":     l.Palette.SemanticBugSelectedTitle,
		"SemanticEpicSelectedTitle":    l.Palette.SemanticEpicSelectedTitle,
		"SemanticStorySelectedTitle":   l.Palette.SemanticStorySelectedTitle,
		"SemanticFeatureSelectedTitle": l.Palette.SemanticFeatureSelectedTitle,
		"SemanticIdeaSelectedTitle":    l.Palette.SemanticIdeaSelectedTitle,
		"SemanticBugDeferredTitle":     l.Palette.SemanticBugDeferredTitle,
		"SemanticEpicDeferredTitle":    l.Palette.SemanticEpicDeferredTitle,
		"SemanticStoryDeferredTitle":   l.Palette.SemanticStoryDeferredTitle,
		"SemanticFeatureDeferredTitle": l.Palette.SemanticFeatureDeferredTitle,
		"SemanticIdeaDeferredTitle":    l.Palette.SemanticIdeaDeferredTitle,
		"BackgroundColor":              l.BackgroundColor,
		"CursorColor":                  l.CursorColor,
		"CursorTextColor":              l.CursorTextColor,
	}
	for name, val := range fields {
		if val == "" {
			t.Errorf("DefaultLightTheme().%s is empty, want a non-empty curated value", name)
		}
	}

	// Spot-check key values.
	if l.Palette.Primary != "#0366d6" {
		t.Errorf("Primary = %q, want #0366d6", l.Palette.Primary)
	}
	if l.BackgroundColor != "#f6f6f0" {
		t.Errorf("BackgroundColor = %q, want #f6f6f0", l.BackgroundColor)
	}
	if l.CursorColor != "#c7b6f0" {
		t.Errorf("CursorColor = %q, want #c7b6f0", l.CursorColor)
	}
	if l.CursorTextColor != "#1f1b35" {
		t.Errorf("CursorTextColor = %q, want #1f1b35", l.CursorTextColor)
	}
}

func wcagContrastRatio(t *testing.T, a, b string) float64 {
	t.Helper()

	l1 := relativeLuminance(t, a)
	l2 := relativeLuminance(t, b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(t *testing.T, hex string) float64 {
	t.Helper()

	value := strings.TrimPrefix(hex, "#")
	if len(value) != 6 {
		t.Fatalf("invalid hex color %q: want #rrggbb", hex)
	}

	r := linearizedColorChannel(t, value[0:2])
	g := linearizedColorChannel(t, value[2:4])
	b := linearizedColorChannel(t, value[4:6])
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func linearizedColorChannel(t *testing.T, hex string) float64 {
	t.Helper()

	n, err := strconv.ParseUint(hex, 16, 8)
	if err != nil {
		t.Fatalf("invalid hex channel %q: %v", hex, err)
	}

	c := float64(n) / 255
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
