package config

// DefaultLightTheme returns a Theme with curated light-mode color values for
// all 17 palette-tab fields (9 palette colors, 3 bg/cursor, 5 priorities).
// These values are optimized for light terminal backgrounds.
func DefaultLightTheme() Theme {
	return Theme{
		BackgroundColor: "#fafafa",
		CursorColor:     "#c7b6f0",
		CursorTextColor: "#1f1b35",
		Palette: PaletteColors{
			NormalText:     "#111827",
			Muted:          "#6b7280",
			Dimmed:         "#9ca3af",
			DimmedBg:       "#e5e7eb",
			DimmedFg:       "#4b5563",
			Primary:        "#0366d6",
			Secondary:      "#1a7f7f",
			Accent:         "#d97706",
			Success:        "#16a34a",
			Warning:        "#b45309",
			Error:          "#dc2626",
			Border:         "#d1d5db",
			Priority1Title: "#0f172a",
			Priority2Title: "#7f1d1d",
			Priority3Title: "#1e3a8a",
			Priority4Title: "#6b7280",
			Priority5Title: "#a3a3a3",
		},
	}
}
