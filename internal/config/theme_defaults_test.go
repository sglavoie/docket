package config

import (
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
		{"Muted", d.Palette.Muted, "#565f89"},
		{"Dimmed", d.Palette.Dimmed, "#3b3f5c"},
		{"DimmedBg", d.Palette.DimmedBg, "#24283b"},
		{"DimmedFg", d.Palette.DimmedFg, "#9aa5ce"},
		{"Border", d.Palette.Border, "#3b3f5c"},
		{"Accent", d.Palette.Accent, "#e0af68"},
		{"NormalText", d.Palette.NormalText, "#c0caf5"},
		{"Priority1Title", d.Palette.Priority1Title, "#f7768e"},
		{"Priority2Title", d.Palette.Priority2Title, "#e0af68"},
		{"Priority3Title", d.Palette.Priority3Title, "#7aa2f7"},
		{"Priority4Title", d.Palette.Priority4Title, "#9aa5ce"},
		{"Priority5Title", d.Palette.Priority5Title, "#565f89"},
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

func TestDefaultLightTheme_AllFieldsPopulated(t *testing.T) {
	l := DefaultLightTheme()

	// All light palette fields must be non-empty (curated hex values).
	fields := map[string]string{
		"NormalText":      l.Palette.NormalText,
		"Muted":           l.Palette.Muted,
		"Dimmed":          l.Palette.Dimmed,
		"DimmedBg":        l.Palette.DimmedBg,
		"DimmedFg":        l.Palette.DimmedFg,
		"Primary":         l.Palette.Primary,
		"Secondary":       l.Palette.Secondary,
		"Accent":          l.Palette.Accent,
		"Success":         l.Palette.Success,
		"Warning":         l.Palette.Warning,
		"Error":           l.Palette.Error,
		"Border":          l.Palette.Border,
		"Priority1Title":  l.Palette.Priority1Title,
		"Priority2Title":  l.Palette.Priority2Title,
		"Priority3Title":  l.Palette.Priority3Title,
		"Priority4Title":  l.Palette.Priority4Title,
		"Priority5Title":  l.Palette.Priority5Title,
		"BackgroundColor": l.BackgroundColor,
		"CursorColor":     l.CursorColor,
		"CursorTextColor": l.CursorTextColor,
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
