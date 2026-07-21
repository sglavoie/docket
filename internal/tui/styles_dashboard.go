package tui

import "charm.land/lipgloss/v2"

// Dashboard and selector styles.
var (
	DashboardCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	DashboardCardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)

	SelectorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	SelectorSearchStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginBottom(1)

	SelectorCountStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				MarginBottom(1)

	SelectorNoResultsStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true).
				MarginTop(1).
				MarginBottom(1)

	SelectorBaseStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	EmptyErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	CategoryCommandStyle     = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle        = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle     = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle         = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle     = lipgloss.NewStyle().Foreground(ColorPurple)
	CategorySavedFilterStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryNoteStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	CategoryViewStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
)
