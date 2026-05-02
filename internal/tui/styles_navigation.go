package tui

import "charm.land/lipgloss/v2"

// Navigation and selection styles.
var (
	TaskJumperListStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(ColorBorder)

	TaskJumperPreviewStyle = lipgloss.NewStyle().
				Padding(0, 1)

	TaskJumperSelectedStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg).
				Foreground(ColorSelectedFg).
				Bold(true)

	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().
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

	CursorFlashRowStyle = lipgloss.NewStyle().
				Background(ColorFlashBg)

	SelectionStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	SelectionCheckStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	SelectionCountStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)
)
