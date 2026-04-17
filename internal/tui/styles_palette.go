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

	ColorViewActive    = lipgloss.Color("34")
	ColorViewNext      = lipgloss.Color("61")
	ColorViewReview    = lipgloss.Color("30")
	ColorViewStandby   = lipgloss.Color("141")
	ColorViewSomeday   = lipgloss.Color("135")
	ColorSelectedFg    = lipgloss.Color("#fbeeff")
	ColorFlashBg       = lipgloss.Color("#7b2fa0")
	ColorLightGray     = lipgloss.Color("252")
	ColorSubtleBorder  = lipgloss.Color("240")
	ColorPurple        = lipgloss.Color("135")
	ColorOrange        = lipgloss.Color("208")
	ColorRuleKeyword   = lipgloss.Color("14")
	ColorRuleOperator  = lipgloss.Color("13")
	ColorZebraBg       = lipgloss.Color("235")
	ColorProgressEmpty = lipgloss.Color("248") // dark-theme default; set by rebuildPalette

	// View accent colors — one per view category for spatial orientation.
	AccentDashboard   = lipgloss.Color("39")  // blue
	AccentProjectTree = lipgloss.Color("44")  // cyan
	AccentFocus       = lipgloss.Color("42")  // green
	AccentPriority    = lipgloss.Color("208") // orange
	AccentReview      = lipgloss.Color("220") // yellow
	AccentAllTasks    = lipgloss.Color("135") // purple
	AccentMetrics     = lipgloss.Color("30")  // teal
	AccentSearch      = lipgloss.Color("198") // magenta

	// TaskCountColorScale is a 10-step gradient from red (low) to green (high).
	TaskCountColorScale = [10]color.Color{
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
)

// TaskCountColor maps a value within [min, max] to a color from TaskCountColorScale.
func TaskCountColor(value, min, max int) color.Color {
	if min == max {
		return TaskCountColorScale[4]
	}
	idx := (value - min) * 9 / (max - min)
	if idx < 0 {
		idx = 0
	} else if idx > 9 {
		idx = 9
	}
	return TaskCountColorScale[idx]
}

// SectionColorEntry represents a color in the section color palette.
type SectionColorEntry struct {
	Name  string
	Color color.Color
}

// SectionColorPalette defines the available colors for sections.
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

// GetSectionColorStyle returns a style for a section badge given a color name.
func GetSectionColorStyle(colorName string) lipgloss.Style {
	bg := ColorBackground
	if themeBgColor != "" {
		bg = lipgloss.Color(themeBgColor)
	}
	for _, paletteColor := range SectionColorPalette {
		if paletteColor.Name == colorName {
			return lipgloss.NewStyle().
				Foreground(paletteColor.Color).
				Background(bg)
		}
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
// Returns nil if the color name is empty or not found.
func GetSectionColorValue(colorName string) color.Color {
	if colorName == "" {
		return nil
	}
	for _, paletteColor := range SectionColorPalette {
		if paletteColor.Name == colorName {
			return paletteColor.Color
		}
	}
	return nil
}
