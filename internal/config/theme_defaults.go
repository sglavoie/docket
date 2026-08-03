package config

// LightThemeDefaultsRev is the revision of the curated DefaultLightTheme values.
// Increment this when palette defaults change so MigratePaletteToThemes can
// force-refresh stale configs. Rev 1 = shipped d5d987c; Rev 2 = this refresh.
const LightThemeDefaultsRev = 4

// DarkThemeDefaultsRev is the revision of the curated DefaultDarkTheme values.
// Increment this when dark palette defaults change so MigratePaletteToThemes
// can force-refresh stale configs. Rev 1 = Tokyo Night controlled-saturation
// palette; Rev 2 = dark contrast refresh.
const DarkThemeDefaultsRev = 3

// DefaultLightTheme returns a Theme with curated light-mode color values for
// all maintained palette roles, including priority and semantic-title variants.
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
			Priority1Title: "#b91c1c",
			Priority2Title: "#92400e",
			Priority3Title: "#1e40af",

			SemanticBugTitle:             "#b42318",
			SemanticFeatureTitle:         "#237a3b",
			SemanticIdeaTitle:            "#5f6b7a",
			SemanticBugSelectedTitle:     "#7f1d1d",
			SemanticFeatureSelectedTitle: "#14532d",
			SemanticIdeaSelectedTitle:    "#374151",
			SemanticBugSnoozedTitle:      "#9f5656",
			SemanticFeatureSnoozedTitle:  "#5f8f6c",
			SemanticIdeaSnoozedTitle:     "#7b8491",
		},
	}
}

// DefaultDarkTheme returns a Theme with curated dark-mode hex values for all
// maintained palette roles. Inspired by Tokyo Night with controlled saturation
// (≤65%) and a contrast lift for primary readable text against #1a1b26.
func DefaultDarkTheme() Theme {
	return Theme{
		BackgroundColor: "#1a1b26",
		CursorColor:     "#33467c",
		CursorTextColor: "#c0caf5",
		Palette: PaletteColors{
			NormalText:     "#c0caf5",
			Muted:          "#8f98c7",
			Dimmed:         "#565f89",
			DimmedBg:       "#24283b",
			DimmedFg:       "#9aa5ce",
			Primary:        "#7aa2f7",
			Secondary:      "#7dcfff",
			Accent:         "#e0af68",
			Success:        "#9ece6a",
			Warning:        "#e0af68",
			Error:          "#f7768e",
			Border:         "#565f89",
			Priority1Title: "#f7768e",
			Priority2Title: "#e0af68",
			Priority3Title: "#7aa2f7",

			SemanticBugTitle:             "#ff9e9e",
			SemanticFeatureTitle:         "#b9f27c",
			SemanticIdeaTitle:            "#c0caf5",
			SemanticBugSelectedTitle:     "#ffd1d1",
			SemanticFeatureSelectedTitle: "#d7f7b5",
			SemanticIdeaSelectedTitle:    "#e5e7eb",
			SemanticBugSnoozedTitle:      "#c97b7b",
			SemanticFeatureSnoozedTitle:  "#8bb65f",
			SemanticIdeaSnoozedTitle:     "#8f98c7",
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
