package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

const zebraBackgroundDarkenFactor = 0.08

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

// ThemeSpacer renders an empty block of the given width, filled with the theme
// background color when set. Used to plug "holes" in horizontal joins.
func ThemeSpacer(width int) string {
	s := lipgloss.NewStyle().Width(width)
	if themeBgColor != "" {
		s = s.Background(lipgloss.Color(themeBgColor))
	}
	return s.Render("")
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
	effectiveFlash := lightenHex(effectiveBg, 0.4)

	ColorSelectedBg = lipgloss.Color(effectiveBg)
	ColorSelectedFg = lipgloss.Color(effectiveFg)
	ColorFlashBg = lipgloss.Color(effectiveFlash)
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

	CursorFlashRowStyle = lipgloss.NewStyle().
		Background(ColorFlashBg)

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

// makeColorStyle creates a style with just a foreground color.
func makeColorStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}
