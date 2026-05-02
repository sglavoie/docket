package tui

import "charm.land/lipgloss/v2"

// View mode and filter styles.
var (
	ViewModeActiveStyle    = makeViewModeStyle("34", "#15803d")
	ViewModeNextStyle      = makeViewModeStyle("61", "#3730a3")
	ViewModeInboxStyle     = makeViewModeStyle("33", "#1d4ed8")
	ViewModeWaitingStyle   = makeViewModeStyle("214", "#92400e")
	ViewModeSomedayStyle   = makeViewModeStyle("135", "#6d28d9")
	ViewModeCompletedStyle = makeViewModeStyle("136", "#4b5563")
	ViewModeReviewStyle    = makeViewModeStyle("30", "#0f766e")
	ViewModeStandbyStyle   = makeViewModeStyle("141", "#7c3aed")
	ViewModeSnoozedStyle   = makeViewModeStyle("95", "#7c2d12")
	ViewModeAllStyle       = makeViewModeStyle("60", "#0369a1")

	FilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	SavedFilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorSecondary).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	FilterBadgeInactiveStyle = lipgloss.NewStyle().
						Background(ColorBackground).
						Foreground(ColorBadgeText).
						Padding(0, 1)

	FilterViewBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	FilterItemStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FilterItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorSelectedFg).
				Bold(true)

	FilterValueStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	FilterClearStyle = lipgloss.NewStyle().
				Foreground(ColorError)
)
