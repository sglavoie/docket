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
		{"Primary", d.Palette.Primary, "39"},
		{"Secondary", d.Palette.Secondary, "33"},
		{"Success", d.Palette.Success, "42"},
		{"Warning", d.Palette.Warning, "214"},
		{"Error", d.Palette.Error, "196"},
		{"Muted", d.Palette.Muted, "248"},
		{"Dimmed", d.Palette.Dimmed, "240"},
		{"DimmedBg", d.Palette.DimmedBg, "237"},
		{"DimmedFg", d.Palette.DimmedFg, "250"},
		{"Border", d.Palette.Border, "244"},
		{"Accent", d.Palette.Accent, "#f2eb16"},
		{"Priority1Title", d.Palette.Priority1Title, "#f93c39"},
		{"Priority2Title", d.Palette.Priority2Title, "#f7b831"},
		{"Priority3Title", d.Palette.Priority3Title, "#56c2f4"},
		{"Priority4Title", d.Palette.Priority4Title, "#cccccc"},
		{"Priority5Title", d.Palette.Priority5Title, "#bbbbbb"},
		{"CursorColor", d.CursorColor, "#4c0163"},
		{"CursorTextColor", d.CursorTextColor, "#fbeeff"},
		{"BackgroundColor", d.BackgroundColor, ""},
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
