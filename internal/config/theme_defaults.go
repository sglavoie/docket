package config

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
			SemanticEpicTitle:            "#6d28d9",
			SemanticStoryTitle:           "#0e7490",
			SemanticFeatureTitle:         "#237a3b",
			SemanticIdeaTitle:            "#5f6b7a",
			SemanticBugSelectedTitle:     "#7f1d1d",
			SemanticEpicSelectedTitle:    "#4c1d95",
			SemanticStorySelectedTitle:   "#155e75",
			SemanticFeatureSelectedTitle: "#14532d",
			SemanticIdeaSelectedTitle:    "#374151",
			SemanticBugSnoozedTitle:      "#9f5656",
			SemanticEpicSnoozedTitle:     "#8b6bb7",
			SemanticStorySnoozedTitle:    "#5f8793",
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
			SemanticEpicTitle:            "#bb9af7",
			SemanticStoryTitle:           "#7dcfff",
			SemanticFeatureTitle:         "#b9f27c",
			SemanticIdeaTitle:            "#c0caf5",
			SemanticBugSelectedTitle:     "#ffd1d1",
			SemanticEpicSelectedTitle:    "#dccbff",
			SemanticStorySelectedTitle:   "#b6e5ff",
			SemanticFeatureSelectedTitle: "#d7f7b5",
			SemanticIdeaSelectedTitle:    "#e5e7eb",
			SemanticBugSnoozedTitle:      "#c97b7b",
			SemanticEpicSnoozedTitle:     "#8d73bd",
			SemanticStorySnoozedTitle:    "#5f9bbf",
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
