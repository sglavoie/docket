package tui

import "charm.land/lipgloss/v2"

// Task type styles.
var (
	EpicStyle = lipgloss.NewStyle().
			Foreground(ColorPurple)

	StoryStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	TaskStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	BugStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	IdeaStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	NoteStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	ActionStyle = lipgloss.NewStyle().
			Foreground(ColorOrange)

	// Badge styles below are initial defaults only; rebuildBadgeStyles() overwrites
	// them on every ApplyTheme call, making them theme-aware at runtime.
	EpicBadgeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	StoryBadgeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	TaskBadgeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	BugBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	IdeaBadgeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	NoteBadgeStyle   = makeBadgeStyle("248", "237")
	ActionBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	TodoBadgeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	InProgressBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	ReviewBadgeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	DoneBadgeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	BlockedBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StandbyBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	MaybeBadgeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	CanceledBadgeStyle   = makeBadgeStyle("248", "236")
	StageBadgeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
)
