package tui

import "charm.land/lipgloss/v2"

// Miscellaneous styles.
var (
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)

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
	PinnedMarker        = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")

	// TodayClearedBannerStyle styles the Peak-End "Today cleared" banner with
	// a warm accent that matches the dashboard streak badge.
	TodayClearedBannerStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorOrange)
)
