package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

// ThemeConfig bundles all user-configurable theme values for ApplyTheme.
type ThemeConfig struct {
	Name          string // "light" or "dark"
	BgColor       string
	CursorBg      string
	CursorFg      string
	Palette       config.PaletteColors
	ZebraStriping bool
}

// NewThemeConfig constructs a ThemeConfig from the portable settings domain.
func NewThemeConfig(s *config.Settings) ThemeConfig {
	t := s.ActiveThemeConfig()
	return ThemeConfig{
		Name:          s.ActiveTheme,
		BgColor:       t.BackgroundColor,
		CursorBg:      t.CursorColor,
		CursorFg:      t.CursorTextColor,
		Palette:       t.Palette,
		ZebraStriping: s.ZebraStriping(),
	}
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
	rebuildSidebarStyles()
	rebuildCategoryStyles()
	rebuildMiscStyles()
	rebuildPriorityTitleStyles()
	rebuildGroupColors()
}

// ApplyTheme reconfigures cursor/selection colors and background propagation.
// Call at startup and after settings save.
func ApplyTheme(tc ThemeConfig) {
	activeThemeName = tc.Name

	themeDefaults := config.DefaultTheme(tc.Name)

	effectiveBgColor := tc.BgColor
	if effectiveBgColor == "" {
		effectiveBgColor = themeDefaults.BackgroundColor
	}
	themeBgColor = effectiveBgColor
	ColorBackground = lipgloss.Color(effectiveBgColor)

	rebuildAccentColors()
	rebuildPalette(tc.Palette)
	rebuildStyles()
	buildViewAccentMap()
	ThemedTextInputStyles = themedTextInputStyles()
	ThemedTextareaStyles = themedTextareaStyles()

	effectiveBg := tc.CursorBg
	if effectiveBg == "" {
		effectiveBg = themeDefaults.CursorColor
	}
	effectiveFg := tc.CursorFg
	if effectiveFg == "" {
		effectiveFg = themeDefaults.CursorTextColor
	}
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

	TaskJumperSelectedStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg).
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedTextFg)

	TagManagerItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedTextFg).
		Bold(true)

	FilterItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedTextFg).
		Bold(true)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedTextFg).
		Padding(0, 1)

	DimmedSelectedStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorSelectedFg).
		Italic(true)

	SelectedTaskTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedTextFg)

	zebraEnabled = tc.ZebraStriping

	// Derive zebra background: darken custom bg 8%, or use default ColorZebraBg.
	if effectiveBgColor != "" {
		derivedZebra := darkenHex(effectiveBgColor, 0.08)
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

	PrefixPopupStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorLightGray).
		Padding(0, 1), alwaysBg)

	FocusModeStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true), alwaysBg)

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

	DashboardCardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	SidebarCardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	SidebarCardFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1)

	SidebarStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1)

	TaskJumperListStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(ColorBorder)

	TaskJumperPreviewStyle = lipgloss.NewStyle().
		Padding(0, 1)

	OverlayBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPurple).
		Padding(1, 2)

	TagManagerBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
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
