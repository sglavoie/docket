package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Semantic color contract:
// Green (ColorSuccess) = completion, positive state (done tasks, diffs added)
// Red   (ColorError)   = danger, critical, overdue, waiting
// Yellow(ColorWarning) = caution, due soon, attention needed
// Blue  (ColorPrimary) = information, navigation, neutral emphasis
// Gray  (ColorMuted)   = disabled, inactive, secondary

// Color palette.
var (
	ColorPrimary    = lipgloss.Color("#7aa2f7")
	ColorSecondary  = lipgloss.Color("#7dcfff")
	ColorSuccess    = lipgloss.Color("#9ece6a")
	ColorWarning    = lipgloss.Color("#e0af68")
	ColorError      = lipgloss.Color("#f7768e")
	ColorMuted      = lipgloss.Color("#8f98c7")
	ColorDimmed     = lipgloss.Color("#565f89")
	ColorDimmedBg   = lipgloss.Color("#24283b")
	ColorDimmedFg   = lipgloss.Color("#9aa5ce")
	ColorHighlight  = lipgloss.Color("#e0af68")
	ColorBackground = lipgloss.Color("#1a1b26")
	ColorBorder     = lipgloss.Color("#565f89")
	ColorBadgeText  = lipgloss.Color("#8f98c7")
	ColorAccent     = lipgloss.Color("#e0af68")
	ColorSelectedBg = lipgloss.Color("#33467c")

	ColorViewStandby     = lipgloss.Color("#bb9af7")
	ColorViewSomeday     = lipgloss.Color("#9d7cd8")
	ColorSelectedFg      = lipgloss.Color("#c0caf5")
	ColorSelectedTextFg  = lipgloss.Color("#c0caf5") // dark-theme default; set by ApplyTheme
	ColorCursorBarBg     = lipgloss.Color("#ffffff") // derived: lightenHex("#33467c", 1.0)
	ColorCursorBarFg     = lipgloss.Color("#000000") // the dark theme's own paper
	ColorFlashBg         = lipgloss.Color("#ffffff") // derived: lightenHex("#33467c", 1.0)
	ColorSelectedDimBg   = lipgloss.Color("#000000") // pure black resting cursor trail
	ColorCursorRowBg     = lipgloss.Color("#1f2a4b") // derived: darkenHex("#33467c", 0.4)
	ColorFlashDimBg      = lipgloss.Color("#333333") // derived: darkenHex("#ffffff", 0.8)
	ColorLightGray       = lipgloss.Color("#8f98c7")
	ColorSubtleBorder    = lipgloss.Color("#565f89")
	ColorPurple          = lipgloss.Color("#bb9af7")
	ColorOrange          = lipgloss.Color("#ff9e64")
	ColorRuleKeyword     = lipgloss.Color("#7dcfff")
	ColorRuleOperator    = lipgloss.Color("#bb9af7")
	ColorZebraBg         = lipgloss.Color("#16171f")
	ColorTreeGroupBg     = lipgloss.Color("#141520") // subtle tint for odd top-level groups
	ColorFirstPlanItemBg = lipgloss.Color("#1e1f2e") // subtle elevated bg for the first today plan item
	ColorProgressEmpty   = lipgloss.Color("#565f89") // dark-theme default; set by rebuildPalette

	// View accent colors — one per pipeline stage for spatial orientation.
	AccentCapture = lipgloss.Color("#f7768e") // coral (Capture stage)
	AccentTriage  = lipgloss.Color("#7dcfff") // sky blue (Triage stage)
	AccentPlan    = lipgloss.Color("#9ece6a") // green (Plan stage)
	AccentWork    = lipgloss.Color("#ff9e64") // orange (Work stage)
	AccentExecute = lipgloss.Color("#73daca") // teal (legacy Execute accent retained for project work views)
	AccentSupport = lipgloss.Color("#565f89") // muted gray (Support stage)
	AccentSearch  = lipgloss.Color("#bb9af7") // purple (search/command)

)

// SectionColorEntry represents a color in the section color palette.
type SectionColorEntry struct {
	Name  string
	Color color.Color
}

// SectionColorPalette defines the available colors for sections (dark-theme hex values).
var SectionColorPalette = defaultSectionColorPalette()

func defaultSectionColorPalette() []SectionColorEntry {
	return []SectionColorEntry{
		{"gray", lipgloss.Color("#565f89")},
		{"red", lipgloss.Color("#f7768e")},
		{"orange", ColorOrange},
		{"yellow", lipgloss.Color("#e0af68")},
		{"green", lipgloss.Color("#9ece6a")},
		{"cyan", lipgloss.Color("#73daca")},
		{"blue", lipgloss.Color("#7aa2f7")},
		{"purple", ColorPurple},
		{"magenta", lipgloss.Color("#ff6b9d")},
		{"white", lipgloss.Color("#c0caf5")},
	}
}

// sectionColorLight maps color names to muted hex values for light theme readability.
var sectionColorLight = map[string]color.Color{
	"gray":    lipgloss.Color("#6b7280"),
	"red":     lipgloss.Color("#b91c1c"),
	"orange":  lipgloss.Color("#c2410c"),
	"yellow":  lipgloss.Color("#a16207"),
	"green":   lipgloss.Color("#15803d"),
	"cyan":    lipgloss.Color("#0e7490"),
	"blue":    lipgloss.Color("#1e40af"),
	"purple":  lipgloss.Color("#6d28d9"),
	"magenta": lipgloss.Color("#9d174d"),
	"white":   lipgloss.Color("#374151"),
}

// GetSectionColorValue returns the color.Color for a section color name.
// Returns the light-theme hex when the active theme is light, otherwise the
// dark-theme ANSI value. Returns nil if the color name is empty or not found.
func GetSectionColorValue(colorName string) color.Color {
	if colorName == "" {
		return nil
	}
	if ActiveThemeName == "light" && activeGhosttyPalette == nil {
		if c, ok := sectionColorLight[colorName]; ok {
			return c
		}
	}
	for _, paletteColor := range SectionColorPalette {
		if paletteColor.Name == colorName {
			return paletteColor.Color
		}
	}
	return nil
}
