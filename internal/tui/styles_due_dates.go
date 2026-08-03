package tui

import "charm.land/lipgloss/v2"

// Due date styles.
var (
	OverdueStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DueDateStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	DueTodayStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)
)
