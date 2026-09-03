package theme

import "charm.land/lipgloss/v2"

// Navigation and selection styles.
var (
	PickerSelectedStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg).
				Foreground(ColorSelectedFg).
				Bold(true)

	ListPreviewHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	GroupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	SectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)

	SelectedStyle = lipgloss.NewStyle().
			Background(ColorSelectedBg).
			Foreground(ColorSelectedFg).
			Bold(true)

	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)

	SelectionRowStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg)

	// CursorBarStyle is the Outliner cursor row's first column and CursorTrailStyle
	// the columns past its content; the Flash pair is the same mark while the
	// cursor pulses. The bar is inverse video rather than a background merged into
	// the glyph it covers, because one column has to carry the whole mark.
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

	SelectionStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	SelectionCheckStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	SelectionCountStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)
)
