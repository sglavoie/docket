package theme

import "charm.land/lipgloss/v2"

// Task workflow status badge styles.
var (
	TodoBadgeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	InProgressBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	DoneBadgeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	WaitingBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StandbyBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	MaybeBadgeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	CanceledBadgeStyle   = makeBadgeStyle("248", "236")
)
