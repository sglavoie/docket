package config

// LightThemeDefaultsRev is the revision of the curated DefaultLightTheme values.
// Increment this when palette defaults change so MigratePaletteToThemes can
// force-refresh stale configs. Rev 1 = shipped d5d987c; Rev 2 = this refresh.
const LightThemeDefaultsRev = 2

// DefaultLightTheme returns a Theme with curated light-mode color values for
// all 17 palette-tab fields (9 palette colors, 3 bg/cursor, 5 priorities).
// These values are optimized for light terminal backgrounds.
func DefaultLightTheme() Theme {
	return Theme{
		BackgroundColor: "#f6f6f0",
		CursorColor:     "#c7b6f0",
		CursorTextColor: "#1f1b35",
		Palette: PaletteColors{
			NormalText:     "#111827",
			Muted:          "#4b5563",
			Dimmed:         "#9ca3af",
			DimmedBg:       "#e5e7eb",
			DimmedFg:       "#6b7280",
			Primary:        "#0366d6",
			Secondary:      "#1a7f7f",
			Accent:         "#c2410c",
			Success:        "#15803d",
			Warning:        "#92400e",
			Error:          "#b91c1c",
			Border:         "#182230",
			Priority1Title: "#0f172a",
			Priority2Title: "#991b1b",
			Priority3Title: "#1e40af",
			Priority4Title: "#475569",
			Priority5Title: "#64748b",
		},
	}
}

// DefaultDarkTheme returns a Theme mirroring the dark-mode ANSI fallbacks used
// at runtime. Empty string fields mean "use terminal default" (no override).
func DefaultDarkTheme() Theme {
	return Theme{
		BackgroundColor: "",
		CursorColor:     "#4c0163",
		CursorTextColor: "#fbeeff",
		Palette: PaletteColors{
			NormalText:     "",
			Muted:          "248",
			Dimmed:         "240",
			DimmedBg:       "237",
			DimmedFg:       "250",
			Primary:        "39",
			Secondary:      "33",
			Accent:         "#f2eb16",
			Success:        "42",
			Warning:        "214",
			Error:          "196",
			Border:         "244",
			Priority1Title: "#f93c39",
			Priority2Title: "#f7b831",
			Priority3Title: "#56c2f4",
			Priority4Title: "#cccccc",
			Priority5Title: "#bbbbbb",
		},
	}
}

// DefaultTheme returns the canonical default Theme for the given variant.
// Unknown variants fall back to dark.
func DefaultTheme(variant string) Theme {
	if variant == "light" {
		return DefaultLightTheme()
	}
	return DefaultDarkTheme()
}
