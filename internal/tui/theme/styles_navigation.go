package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// cursorBarStyle builds one inverse-video cursor column. The two colors are
// handed over in the order the terminal swaps them — the bright side as the
// foreground, the paper as the background — so the drawn column looks exactly as
// a literal pair would. What the attribute buys is the case where no color
// arrives at all: a profile that drops every color keeps SGR 7, so the cursor's
// own column stays the one inverted cell on screen where a literal background
// would have left an ordinary blank. It is the only mark this row wears that
// survives --no-color, which is why it is the mark and not the row tint.
func cursorBarStyle(bright, paper color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(bright).Background(paper).Reverse(true)
}

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

	// CursorBarStyle is the Outliner cursor row's first column, CursorRowStyle the
	// band the marked task row wears under everything it drew, and CursorTrailStyle
	// the columns past the content on a row that wears no band; the Flash pair is
	// the same mark while the cursor pulses. The bar is inverse video rather than a
	// background merged into the glyph it covers, because one column has to carry
	// the whole mark and has to keep carrying it with no color at all.
	CursorBarStyle = cursorBarStyle(ColorCursorBarBg, ColorCursorBarFg)

	CursorRowStyle = lipgloss.NewStyle().
			Background(ColorCursorRowBg)

	CursorTrailStyle = lipgloss.NewStyle().
				Background(ColorSelectedDimBg)

	CursorFlashBarStyle = cursorBarStyle(ColorFlashBg, ColorCursorBarFg)

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
