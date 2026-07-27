package tui

import "charm.land/lipgloss/v2"

// Status and help view styles.
var (
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	InfoStatusStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	LoadingStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	ErrorStatusStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	HelpViewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	HelpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	HelpFilterActiveStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)
)
