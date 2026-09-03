package theme

import "charm.land/lipgloss/v2"

// Task status styles.
var (
	TodoStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	InProgressStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	WaitingStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	StandbyStyle = lipgloss.NewStyle().
			Foreground(ColorViewStandby)

	MaybeStyle = lipgloss.NewStyle().
			Foreground(ColorViewSomeday)

	CanceledStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Strikethrough(true)
)
