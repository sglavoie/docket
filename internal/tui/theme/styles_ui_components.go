package theme

import "charm.land/lipgloss/v2"

// UI component styles.
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Background(ColorBackground).
			Padding(0, 1)

	FormLabelWidth = 11

	ProgressFullStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ProgressEmptyStyle = lipgloss.NewStyle().
				Foreground(ColorProgressEmpty)

	ScrollbarThumbStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary)

	ScrollbarTrackStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	ArchivedBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)
)
