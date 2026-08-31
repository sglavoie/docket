package tui

import "charm.land/lipgloss/v2"

// Miscellaneous styles.
var (
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// The two soft-reference indicator colors are named rather than borrowed from
	// SuccessStyle or DueThisWeekStyle, so retuning a due-date or status color
	// never silently repaints a reference count. The third tier is MutedStyle.
	DependencyIndicatorSettledStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	DependencyIndicatorBlockedStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	OverlaySplitStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	OverlayTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorPurple)
	OverlaySelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	OverlayDetailStyle   = lipgloss.NewStyle().Foreground(ColorLightGray)
	OverlayBoxStyle      = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPurple).
				Padding(1, 2)

	FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	FocusIndicator      = FocusIndicatorStyle.Render("▐")
)
