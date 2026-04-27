package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Color palette.
var (
	ColorPrimary     = lipgloss.Color("39")
	ColorSecondary   = lipgloss.Color("33")
	ColorSuccess     = lipgloss.Color("42")
	ColorWarning     = lipgloss.Color("214")
	ColorError       = lipgloss.Color("196")
	ColorMuted       = lipgloss.Color("248")
	ColorDimmed      = lipgloss.Color("240")
	ColorDimmedBg    = lipgloss.Color("237")
	ColorDimmedFg    = lipgloss.Color("250")
	ColorHighlight   = lipgloss.Color("#f2eb16")
	ColorBackground  = lipgloss.Color("236")
	ColorBorder      = lipgloss.Color("244")
	ColorDiffContext = lipgloss.Color("249")
	ColorBadgeText   = lipgloss.Color("249")
	ColorAccent      = lipgloss.Color("226")
	ColorSelectedBg  = lipgloss.Color("#4c0163")

	ColorViewActive     = lipgloss.Color("34")
	ColorViewNext       = lipgloss.Color("61")
	ColorViewStandby    = lipgloss.Color("141")
	ColorViewSomeday    = lipgloss.Color("135")
	ColorSelectedFg     = lipgloss.Color("#fbeeff")
	ColorSelectedTextFg = lipgloss.Color("#fbeeff") // dark-theme default; set by ApplyTheme
	ColorFlashBg        = lipgloss.Color("#7b2fa0")
	ColorLightGray      = lipgloss.Color("252")
	ColorSubtleBorder   = lipgloss.Color("240")
	ColorPurple         = lipgloss.Color("135")
	ColorOrange         = lipgloss.Color("208")
	ColorRuleKeyword    = lipgloss.Color("14")
	ColorRuleOperator   = lipgloss.Color("13")
	ColorZebraBg        = lipgloss.Color("235")
	ColorProgressEmpty  = lipgloss.Color("248") // dark-theme default; set by rebuildPalette

	// View accent colors — one per pipeline stage for spatial orientation.
	AccentDashboard = lipgloss.Color("39")  // blue (dashboard)
	AccentCapture   = lipgloss.Color("209") // coral/salmon (Capture stage)
	AccentOrganize  = lipgloss.Color("75")  // sky blue (Organize stage)
	AccentPlan      = lipgloss.Color("42")  // green (Plan stage)
	AccentWork      = lipgloss.Color("208") // orange (Work stage)
	AccentExecute   = lipgloss.Color("44")  // cyan (Execute stage)
	AccentReview    = lipgloss.Color("220") // yellow (Review stage)
	AccentSupport   = lipgloss.Color("245") // muted gray (Support stage)
	AccentSearch    = lipgloss.Color("198") // magenta (search/command)

)

// taskCountScaleDark is the 10-step gradient used in dark theme (bright ANSI).
var taskCountScaleDark = [10]color.Color{
	lipgloss.Color("196"),
	lipgloss.Color("202"),
	lipgloss.Color("208"),
	lipgloss.Color("214"),
	lipgloss.Color("220"),
	lipgloss.Color("226"),
	lipgloss.Color("154"),
	lipgloss.Color("118"),
	lipgloss.Color("82"),
	lipgloss.Color("42"),
}

// taskCountScaleLight is the 10-step gradient used in light theme (muted hex).
var taskCountScaleLight = [10]color.Color{
	lipgloss.Color("#b91c1c"),
	lipgloss.Color("#c2410c"),
	lipgloss.Color("#d97706"),
	lipgloss.Color("#ca8a04"),
	lipgloss.Color("#a16207"),
	lipgloss.Color("#65a30d"),
	lipgloss.Color("#4d7c0f"),
	lipgloss.Color("#15803d"),
	lipgloss.Color("#166534"),
	lipgloss.Color("#14532d"),
}

// taskCountScale returns the theme-appropriate color scale.
func taskCountScale() [10]color.Color {
	if activeThemeName == "light" {
		return taskCountScaleLight
	}
	return taskCountScaleDark
}

// TaskCountColor maps a value within [min, max] to a theme-aware color gradient.
func TaskCountColor(value, min, max int) color.Color {
	scale := taskCountScale()
	if min == max {
		return scale[4]
	}
	idx := (value - min) * 9 / (max - min)
	if idx < 0 {
		idx = 0
	} else if idx > 9 {
		idx = 9
	}
	return scale[idx]
}

// SectionColorEntry represents a color in the section color palette.
type SectionColorEntry struct {
	Name  string
	Color color.Color
}

// SectionColorPalette defines the available colors for sections (dark-theme ANSI values).
var SectionColorPalette = []SectionColorEntry{
	{"gray", lipgloss.Color("245")},
	{"red", lipgloss.Color("196")},
	{"orange", ColorOrange},
	{"yellow", lipgloss.Color("220")},
	{"green", lipgloss.Color("42")},
	{"cyan", lipgloss.Color("44")},
	{"blue", lipgloss.Color("33")},
	{"purple", ColorPurple},
	{"magenta", lipgloss.Color("198")},
	{"white", lipgloss.Color("255")},
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

// GetSectionColorStyle returns a style for a section badge given a color name.
func GetSectionColorStyle(colorName string) lipgloss.Style {
	bg := ColorBackground
	if themeBgColor != "" {
		bg = lipgloss.Color(themeBgColor)
	}
	fg := GetSectionColorValue(colorName)
	if fg != nil {
		return lipgloss.NewStyle().Foreground(fg).Background(bg)
	}
	return SectionBadgeStyle
}

// GetSectionColorIndex returns the index of a color name in the palette.
func GetSectionColorIndex(colorName string) int {
	for index, paletteColor := range SectionColorPalette {
		if paletteColor.Name == colorName {
			return index
		}
	}
	return 0
}

// GetSectionColorByIndex returns the color name at the given palette index.
func GetSectionColorByIndex(index int) string {
	if index < 0 || index >= len(SectionColorPalette) {
		return "gray"
	}
	return SectionColorPalette[index].Name
}

// GetSectionColorValue returns the color.Color for a section color name.
// Returns the light-theme hex when the active theme is light, otherwise the
// dark-theme ANSI value. Returns nil if the color name is empty or not found.
func GetSectionColorValue(colorName string) color.Color {
	if colorName == "" {
		return nil
	}
	if activeThemeName == "light" {
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
