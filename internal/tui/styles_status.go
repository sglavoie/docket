package tui

import "charm.land/lipgloss/v2"

// Task status styles.
var (
	TodoStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	InProgressStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	ReviewStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	DoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	BlockedStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	StandbyStyle = lipgloss.NewStyle().
			Foreground(ColorViewStandby)

	CanceledStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Strikethrough(true)
)
