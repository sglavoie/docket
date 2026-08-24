package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

const zebraBackgroundDarkenFactor = 0.08

// The cursor row is marked by one bright column at its left edge and a dim fill
// past its content. Both are derived from the theme's cursor color, in opposite
// directions: the bar away from the terminal's own paper so a single column is
// still unmissable, the fill toward it so the reach never competes with the mark.
//
// Every factor is per-theme, because the two directions are not symmetric.
// Darkening keeps a color's chroma, so a light theme reads a moderate step as a
// distinct dark block; lightening washes chroma out, so the same step on a dark
// theme lands on a mid gray that a row full of colored titles and badges already
// out-shouts. A dark bar therefore goes most of the way to white, which is what
// makes it the brightest thing on the row rather than one more bright thing.
const (
	cursorBarDarkFactor  = 0.85
	cursorBarLightFactor = 0.55

	// A flash is the same mark spent wider rather than recolored, so its color
	// only has to stay past the bar's — see cursorPaint.gutterBg.
	cursorFlashDarkFactor  = 1.0
	cursorFlashLightFactor = 0.8

	// A dark theme can take the fill past its own paper, where it reads as a
	// recess; a light one washes out to invisible long before that, so it stops
	// at a tint of the cursor color.
	cursorTrailDarkFactor  = 0.8
	cursorTrailLightFactor = 0.5
)

// ThemeConfig identifies one of the two maintained themes.
type ThemeConfig struct {
	Name string // "light" or "dark"; every other value resolves to dark
}

// NewThemeConfig constructs a ThemeConfig from machine-local state.
func NewThemeConfig(machine *config.MachineState) ThemeConfig {
	if machine != nil && machine.Theme == "light" {
		return ThemeConfig{Name: "light"}
	}
	return ThemeConfig{Name: "dark"}
}

// rebuildStyles reassigns every style variable using the current palette colors.
// This must be called after rebuildPalette so styles pick up user overrides.
func rebuildStyles() {
	rebuildTextStyles()
	rebuildTaskStyles()
	rebuildBadgeStyles()
	rebuildLayoutStyles()
	rebuildFormStyles()
	rebuildViewStyles()
	rebuildSelectionStyles()
	rebuildPanelStyles()
	rebuildCategoryStyles()
	rebuildMiscStyles()
	rebuildPriorityTitleStyles()
	rebuildTagChipColors()
}

// ApplyTheme rebuilds every active style from one maintained built-in theme.
// Unknown or empty names deterministically select dark.
func ApplyTheme(tc ThemeConfig) {
	if tc.Name != "light" {
		tc.Name = "dark"
	}
	activeThemeName = tc.Name

	themeDefaults := config.DefaultTheme(tc.Name)

	effectiveBgColor := themeDefaults.BackgroundColor
	themeBgColor = effectiveBgColor
	ColorBackground = lipgloss.Color(effectiveBgColor)

	rebuildAccentColors()
	rebuildPalette(themeDefaults.Palette)
	rebuildStyles()
	buildViewAccentMap()
	ThemedTextInputStyles = themedTextInputStyles()
	ThemedTextareaStyles = themedTextareaStyles()

	effectiveBg := themeDefaults.CursorColor
	effectiveFg := themeDefaults.CursorTextColor

	// Which way "louder" and "quieter" point depends on the paper, so the pair of
	// directions flips with the theme rather than being fixed to lighter/darker,
	// and each step's size flips with it.
	pop, dim := lightenHex, darkenHex
	barFactor, flashFactor, dimFactor := cursorBarDarkFactor, cursorFlashDarkFactor, cursorTrailDarkFactor
	if tc.Name == "light" {
		pop, dim = darkenHex, lightenHex
		barFactor, flashFactor, dimFactor = cursorBarLightFactor, cursorFlashLightFactor, cursorTrailLightFactor
	}
	effectiveBar := pop(effectiveBg, barFactor)
	effectiveFlash := pop(effectiveBg, flashFactor)

	ColorSelectedBg = lipgloss.Color(effectiveBg)
	ColorSelectedFg = lipgloss.Color(effectiveFg)
	ColorCursorBarBg = lipgloss.Color(effectiveBar)
	// The bar is inverse video, so its glyph takes the terminal's own paper.
	ColorCursorBarFg = lipgloss.Color(effectiveBgColor)
	ColorFlashBg = lipgloss.Color(effectiveFlash)
	ColorSelectedDimBg = lipgloss.Color(dim(effectiveBg, dimFactor))
	ColorFlashDimBg = lipgloss.Color(dim(effectiveFlash, dimFactor))
	if tc.Name == "light" {
		ColorSelectedTextFg = lipgloss.Color("#0f172a")
	} else {
		ColorSelectedTextFg = lipgloss.Color("#c0caf5")
	}

	SelectedStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg).
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectionRowStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg)

	CursorBarStyle = lipgloss.NewStyle().
		Background(ColorCursorBarBg).
		Foreground(ColorCursorBarFg)

	CursorTrailStyle = lipgloss.NewStyle().
		Background(ColorSelectedDimBg)

	CursorFlashBarStyle = lipgloss.NewStyle().
		Background(ColorFlashBg).
		Foreground(ColorCursorBarFg)

	CursorFlashRowStyle = lipgloss.NewStyle().
		Background(ColorFlashBg)

	CursorFlashTrailStyle = lipgloss.NewStyle().
		Background(ColorFlashDimBg)

	PickerSelectedStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg).
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedTextFg)

	FilterItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedTextFg).
		Bold(true)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedTextFg).
		Padding(0, 1)

	SelectedTaskTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedTextFg)

	zebraEnabled = false

	// Derive zebra background: darken custom bg 8%, or use default ColorZebraBg.
	if effectiveBgColor != "" {
		derivedZebra := darkenHex(effectiveBgColor, zebraBackgroundDarkenFactor)
		ZebraRowStyle = lipgloss.NewStyle().Background(lipgloss.Color(derivedZebra))
	} else {
		ZebraRowStyle = lipgloss.NewStyle().Background(ColorZebraBg)
	}

	// Derive tree group background: darken custom bg 4%, or use default ColorTreeGroupBg.
	if effectiveBgColor != "" {
		derivedGroupBg := darkenHex(effectiveBgColor, 0.04)
		TreeGroupOddBgStyle = lipgloss.NewStyle().Background(lipgloss.Color(derivedGroupBg))
	} else {
		TreeGroupOddBgStyle = lipgloss.NewStyle().Background(ColorTreeGroupBg)
	}

	// Derive first plan item background: lighten custom bg 12%, or use default ColorFirstPlanItemBg.
	if effectiveBgColor != "" {
		derivedFirstPlan := lightenHex(effectiveBgColor, 0.12)
		FirstPlanItemStyle = lipgloss.NewStyle().Background(lipgloss.Color(derivedFirstPlan))
	} else {
		FirstPlanItemStyle = lipgloss.NewStyle().Background(ColorFirstPlanItemBg)
	}

	alwaysBg := ColorBackground
	if effectiveBgColor != "" {
		alwaysBg = lipgloss.Color(effectiveBgColor)
	}

	StatusBarStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorHighlight).
		Padding(0, 1), alwaysBg)

	FilterBadgeInactiveStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorBadgeText).
		Padding(0, 1), alwaysBg)

	TagStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Padding(0, 1), alwaysBg)

	SectionBadgeStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorBadgeText), alwaysBg)

	ChildrenBadgeStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorPrimary), alwaysBg)

	PanelCardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	OverlayBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPurple).
		Padding(1, 2)

	SelectorBaseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder)

	FilterViewBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	SuggestionBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 1)
}

// withOptionalBg applies a background color to a style if bg is non-nil.
func withOptionalBg(s lipgloss.Style, bg color.Color) lipgloss.Style {
	if bg != nil {
		return s.Background(bg)
	}
	return s
}

// makeBadgeStyle creates a badge style with the given foreground and background ANSI colors.
func makeBadgeStyle(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg))
}

// makeViewModeStyle creates a view mode tab style with tone-aware background.
// dark is used on dark themes; lightHex (a curated hex color) on light themes.
func makeViewModeStyle(dark, lightHex string) lipgloss.Style {
	bgStr := dark
	fgStr := "#c0caf5"
	if activeThemeName == "light" {
		bgStr = lightHex
		fgStr = "#ffffff"
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bgStr)).
		Foreground(lipgloss.Color(fgStr)).
		Bold(true).
		Padding(0, 1)
}

// makeDueBadgeStyle creates a due-date chip with the section badge's contrast
// contract: a tone-aware background under a high-contrast foreground. It omits
// bold deliberately — chips carry no bold, so bold stays the title's selection
// channel, exactly as tagChipStyle already holds.
func makeDueBadgeStyle(darkHex, lightHex string) lipgloss.Style {
	bgStr, fgStr := darkHex, "#c0caf5"
	if activeThemeName == "light" {
		bgStr, fgStr = lightHex, "#ffffff"
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bgStr)).
		Foreground(lipgloss.Color(fgStr)).
		Padding(0, 1)
}

// makeColorStyle creates a style with just a foreground color.
func makeColorStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}
