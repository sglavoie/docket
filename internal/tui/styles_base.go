package tui

import "charm.land/lipgloss/v2"

// Base typography and common styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			MarginBottom(1)

	// DimStyle applies the ANSI faint attribute plus an explicit muted color to
	// visually de-emphasize unfocused panes. The explicit foreground ensures
	// readable contrast on light themes where Faint alone would wash text out.
	DimStyle = lipgloss.NewStyle().Foreground(ColorDimmed).Faint(true)

	// DimmedTaskStyle applies a light gray background + dark text + italic to visually dim task lines.
	DimmedTaskStyle = lipgloss.NewStyle().
			Background(ColorDimmedBg).
			Foreground(ColorDimmedFg).
			Italic(true)

	// DimmedSelectedStyle is used for dimmed tasks that are also under the cursor.
	DimmedSelectedStyle = lipgloss.NewStyle().
				Background(ColorDimmedBg).
				Foreground(ColorSelectedFg).
				Italic(true)

	// BlockedRowStyle applies a subtle red background tint to blocked rows.
	BlockedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2e1515"))

	// OverdueRowStyle applies a subtle amber background tint to overdue rows.
	OverdueRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2d1f0e"))

	// SelectedTaskTitleStyle is applied to the task title on the selected row.
	SelectedTaskTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSelectedFg)

	NormalStyle = lipgloss.NewStyle()

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)
)
