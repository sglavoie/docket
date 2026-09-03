package theme

import "charm.land/lipgloss/v2"

// Shared selector styles.
var (
	SelectorTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	SelectorSearchStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginBottom(1)
	SelectorCountStyle     = lipgloss.NewStyle().Foreground(ColorMuted).MarginBottom(1)
	SelectorNoResultsStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1).MarginBottom(1)
	SelectorBaseStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(ColorBorder)
	EmptyErrorStyle        = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	SeparatorStyle         = lipgloss.NewStyle().Foreground(ColorMuted)
	CategoryCommandStyle   = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle      = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle   = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle       = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle   = lipgloss.NewStyle().Foreground(ColorPurple)
	CategoryNoteStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	CategoryViewStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	SelectedCheckboxStyle  = lipgloss.NewStyle().Foreground(ColorSuccess)
)
