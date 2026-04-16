package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

// Default cursor/selection colors.
const (
	DefaultSelectedBg = "#4c0163"
	DefaultSelectedFg = "#fbeeff"
	DefaultFlashBg    = "#7b2fa0"
)

// ThemeConfig bundles all user-configurable theme values for ApplyTheme.
type ThemeConfig struct {
	BgColor       string
	CursorBg      string
	CursorFg      string
	Palette       config.PaletteColors
	ZebraStriping bool
}

// NewThemeConfig constructs a ThemeConfig from the application config.
func NewThemeConfig(cfg *config.Config) ThemeConfig {
	return ThemeConfig{
		BgColor:       cfg.TUIBackgroundColor,
		CursorBg:      cfg.TUICursorColor,
		CursorFg:      cfg.TUICursorTextColor,
		Palette:       cfg.Palette,
		ZebraStriping: cfg.ZebraStriping(),
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
}

// ApplyTheme reconfigures cursor/selection colors and background propagation.
// Call at startup and after settings save.
func ApplyTheme(tc ThemeConfig) {
	rebuildPalette(tc.Palette)
	rebuildStyles()
	buildViewAccentMap()

	effectiveBg := tc.CursorBg
	if effectiveBg == "" {
		effectiveBg = DefaultSelectedBg
	}
	effectiveFg := tc.CursorFg
	if effectiveFg == "" {
		effectiveFg = DefaultSelectedFg
	}
	effectiveFlash := lightenHex(effectiveBg, 0.4)

	ColorSelectedBg = lipgloss.Color(effectiveBg)
	ColorSelectedFg = lipgloss.Color(effectiveFg)
	ColorFlashBg = lipgloss.Color(effectiveFlash)

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

	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)

	TagManagerItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedFg).
		Bold(true)

	FilterItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedFg).
		Padding(0, 1)

	DimmedSelectedStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorSelectedFg).
		Italic(true)

	SelectedTaskTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedFg)

	themeBgColor = tc.BgColor
	zebraEnabled = tc.ZebraStriping

	// Derive zebra background: darken custom bg 8%, or use default ColorZebraBg.
	if tc.BgColor != "" {
		derivedZebra := darkenHex(tc.BgColor, 0.08)
		ZebraRowStyle = lipgloss.NewStyle().Background(lipgloss.Color(derivedZebra))
	} else {
		ZebraRowStyle = lipgloss.NewStyle().Background(ColorZebraBg)
	}

	var customBg color.Color
	alwaysBg := ColorBackground
	if tc.BgColor != "" {
		customBg = lipgloss.Color(tc.BgColor)
		alwaysBg = customBg
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

	DashboardCardStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1), customBg)

	SidebarCardStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1), customBg)

	SidebarCardFocusedStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1), customBg)

	SidebarStyle = withOptionalBg(lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1), customBg)

	TaskJumperListStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(ColorBorder), customBg)

	TaskJumperPreviewStyle = withOptionalBg(lipgloss.NewStyle().
		Padding(0, 1), customBg)

	OverlayBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPurple).
		Padding(1, 2), customBg)

	TagManagerBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2), customBg)

	SelectorBaseStyle = withOptionalBg(lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder), customBg)

	FilterViewBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2), customBg)

	SuggestionBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 1), customBg)
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

// makeViewModeStyle creates a view mode tab style with the given background ANSI color.
func makeViewModeStyle(bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1)
}

// makeColorStyle creates a style with just a foreground color.
func makeColorStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}
