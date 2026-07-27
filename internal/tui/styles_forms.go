package tui

import "charm.land/lipgloss/v2"

// Form and input field styles.
var (
	ValidationErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				MarginTop(1)

	WarningMessageStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	FieldLabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FieldLabelFocusedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	FieldDescriptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	SelectorOptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSelectedFg).
					Padding(0, 1)
)
