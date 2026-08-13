package tui

import "charm.land/lipgloss/v2"

// View mode and filter styles.
var (
	ViewModeActiveStyle    = makeViewModeStyle("#2e4a30", "#15803d")
	ViewModeWorkStyle      = makeViewModeStyle("#4a321e", "#9a3412")
	ViewModeNextStyle      = makeViewModeStyle("#2d3a6a", "#3730a3")
	ViewModeInboxStyle     = makeViewModeStyle("#1e3a5f", "#1d4ed8")
	ViewModeWaitingStyle   = makeViewModeStyle("#5c3a1e", "#92400e")
	ViewModeSomedayStyle   = makeViewModeStyle("#3d2a5c", "#6d28d9")
	ViewModeCompletedStyle = makeViewModeStyle("#3b3f5c", "#4b5563")
	ViewModeStandbyStyle   = makeViewModeStyle("#4a2e6a", "#7c3aed")
	ViewModeDeferredStyle  = makeViewModeStyle("#4a2e3a", "#7c2d12")
	ViewModeCanceledStyle  = makeViewModeStyle("#4a2430", "#991b1b")
	ViewModeAllStyle       = makeViewModeStyle("#283352", "#0369a1")

	FilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
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
