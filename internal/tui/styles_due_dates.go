package tui

import "charm.land/lipgloss/v2"

// Due date and dependency related styles.
var (
	OverdueStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DependencyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	DueDateStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	DueTodayStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	BlockedByDepsStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")).
				Background(ColorError)
)
