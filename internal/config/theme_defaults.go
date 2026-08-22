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
			NormalText:          "#111827",
			Muted:               "#4b5563",
			Dimmed:              "#9ca3af",
			DimmedBg:            "#e5e7eb",
			DimmedFg:            "#6b7280",
			Primary:             "#0366d6",
			Secondary:           "#1a7f7f",
			Accent:              "#c2410c",
			Success:             "#15803d",
			Warning:             "#92400e",
			Error:               "#b91c1c",
			Border:              "#182230",
			Priority1Title:      "#b91c1c",
			Priority2Title:      "#92400e",
			Priority3Title:      "#1e40af",
			TaskContext:         "#0f766e",
			TaskContextDeferred: "#5f8782",

			SemanticBugTitle:             "#cc2222",
			SemanticEpicTitle:            "#9d1fb8",
			SemanticStoryTitle:           "#0b6ba8",
			SemanticFeatureTitle:         "#067a3f",
			SemanticIdeaTitle:            "#8f6b00",
			SemanticBugSelectedTitle:     "#8f1818",
			SemanticEpicSelectedTitle:    "#6e1681",
			SemanticStorySelectedTitle:   "#084b76",
			SemanticFeatureSelectedTitle: "#04552c",
			SemanticIdeaSelectedTitle:    "#644b00",
			SemanticBugDeferredTitle:     "#b06c6c",
			SemanticEpicDeferredTitle:    "#a26aaf",
			SemanticStoryDeferredTitle:   "#4d84a7",
			SemanticFeatureDeferredTitle: "#3f8d66",
			SemanticIdeaDeferredTitle:    "#927d3c",
		},
	}
}

// DefaultDarkTheme returns a Theme with curated dark-mode hex values for all
// maintained palette roles. Inspired by Tokyo Night with controlled saturation
// (≤65%) and a contrast lift for primary readable text against #1a1b26. The
// semantic title family is the deliberate exception: it runs at full chroma and
// spreads its five hues apart, because only a tagged task wears one and telling
// them apart at a glance is the whole point of the color. The tag chip ramp
// (DefaultTagChipRamp) is the explicit tier below it: chips hold to the ≤65%
// rule, so a chip stays subordinate to a title by construction rather than by
// convention.
func DefaultDarkTheme() Theme {
	return Theme{
		BackgroundColor: "#1a1b26",
		CursorColor:     "#33467c",
		CursorTextColor: "#c0caf5",
		Palette: PaletteColors{
			NormalText:          "#c0caf5",
			Muted:               "#8f98c7",
			Dimmed:              "#565f89",
			DimmedBg:            "#24283b",
			DimmedFg:            "#9aa5ce",
			Primary:             "#7aa2f7",
			Secondary:           "#7dcfff",
			Accent:              "#e0af68",
			Success:             "#9ece6a",
			Warning:             "#e0af68",
			Error:               "#f7768e",
			Border:              "#565f89",
			Priority1Title:      "#f7768e",
			Priority2Title:      "#e0af68",
			Priority3Title:      "#7aa2f7",
			TaskContext:         "#73daca",
			TaskContextDeferred: "#4f948f",

			SemanticBugTitle:             "#ff4f58",
			SemanticEpicTitle:            "#e17bff",
			SemanticStoryTitle:           "#3fc5ff",
			SemanticFeatureTitle:         "#35d67f",
			SemanticIdeaTitle:            "#ffd23f",
			SemanticBugSelectedTitle:     "#ff9ea3",
			SemanticEpicSelectedTitle:    "#eeb6ff",
			SemanticStorySelectedTitle:   "#95dfff",
			SemanticFeatureSelectedTitle: "#90e8b9",
			SemanticIdeaSelectedTitle:    "#ffe695",
			SemanticBugDeferredTitle:     "#c47074",
			SemanticEpicDeferredTitle:    "#b16fc4",
			SemanticStoryDeferredTitle:   "#4891b0",
			SemanticFeatureDeferredTitle: "#549572",
			SemanticIdeaDeferredTitle:    "#9c8740",
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

// DefaultTagChipRamp returns the curated tag chip colors for the given variant.
// Unknown variants fall back to dark, and each call hands back a fresh slice.
//
// The ramp is a mnemonic, not a legend: a slot promises "this tag is reliably
// that color", never "this color means that kind of tag". It is six slots
// because six is what two themes can hold at once — every slot has to stay
// perceptually clear of the other five, of the semantic and priority titles it
// shares a row with, and of both alert row backgrounds, all at ≤65% saturation.
// Slot order carries across variants: slot i is the same color family light and
// dark, so toggling the theme never reshuffles which tag wears which hue.
// theme_defaults_test.go gates each of those properties.
func DefaultTagChipRamp(variant string) []string {
	if variant == "light" {
		return []string{"#a16058", "#677921", "#568760", "#2586a7", "#6d68bb", "#b44e7e"}
	}
	return []string{"#cc846f", "#90ae52", "#45a572", "#2cafc3", "#877cb7", "#e283c9"}
}
