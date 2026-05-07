package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Status icon constants for TUI rendering.
const (
	StatusIconBlocked = "⊘"
)

// Priority icon constants for TUI rendering.
const (
	PriorityIcon1 = "◉"
	PriorityIcon2 = "●"
	PriorityIcon3 = "◆"
	PriorityIcon4 = "◇"
	PriorityIcon5 = "·"
)

// PriorityIcon returns the unicode icon for a priority level (1-5).
func PriorityIcon(priority int) string {
	switch priority {
	case 1:
		return PriorityIcon1
	case 2:
		return PriorityIcon2
	case 3:
		return PriorityIcon3
	case 4:
		return PriorityIcon4
	case 5:
		return PriorityIcon5
	default:
		return PriorityIcon3
	}
}

// GetStatusStyle returns the appropriate style for a task status.
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "todo":
		return TodoStyle
	case "in_progress":
		return InProgressStyle
	case "review":
		return ReviewStyle
	case "standby":
		return StandbyStyle
	case "maybe":
		return MaybeStyle
	case "done":
		return DoneStyle
	case "blocked":
		return BlockedStyle
	case "canceled":
		return CanceledStyle
	default:
		return NormalStyle
	}
}

// GetPriorityStyle returns the appropriate style for a priority level.
// Delegates to GetTitlePriorityStyle for unified palette-driven color sourcing.
func GetPriorityStyle(priority int) lipgloss.Style {
	return GetTitlePriorityStyle(priority)
}

// GetTitlePriorityStyle returns the title style for a priority level (1–5).
// Bold+Italic for P1, Bold for P2, plain for P3/P4, Faint for P5.
// Out-of-range values fall back to P3.
func GetTitlePriorityStyle(priority int) lipgloss.Style {
	switch priority {
	case 1:
		return TitlePriority1Style
	case 2:
		return TitlePriority2Style
	case 3:
		return TitlePriority3Style
	case 4:
		return TitlePriority4Style
	case 5:
		return TitlePriority5Style
	default:
		return TitlePriority3Style
	}
}

// GetTypeStyle returns the appropriate style for a task type.
func GetTypeStyle(taskType string) lipgloss.Style {
	switch taskType {
	case "epic":
		return EpicStyle
	case "story":
		return StoryStyle
	case "task":
		return TaskStyle
	case "bug":
		return BugStyle
	case "idea":
		return IdeaStyle
	case "note":
		return NoteStyle
	case "action":
		return ActionStyle
	default:
		return NormalStyle
	}
}

// GetTypeBadge returns the task type icon rendered as a styled badge with background.
func GetTypeBadge(taskType, icon string) string {
	switch taskType {
	case "epic":
		return EpicBadgeStyle.Render(icon)
	case "story":
		return StoryBadgeStyle.Render(icon)
	case "task":
		return TaskBadgeStyle.Render(icon)
	case "bug":
		return BugBadgeStyle.Render(icon)
	case "idea":
		return IdeaBadgeStyle.Render(icon)
	case "note":
		return NoteBadgeStyle.Render(icon)
	case "action":
		return ActionBadgeStyle.Render(icon)
	default:
		return NormalStyle.Render(icon)
	}
}

// GetStatusBadge returns the task status icon rendered as a styled badge with background.
func GetStatusBadge(status, icon string) string {
	switch status {
	case "todo":
		return TodoBadgeStyle.Render(icon)
	case "in_progress":
		return InProgressBadgeStyle.Render(icon)
	case "review":
		return ReviewBadgeStyle.Render(icon)
	case "standby":
		return StandbyBadgeStyle.Render(icon)
	case "maybe":
		return MaybeBadgeStyle.Render(icon)
	case "done":
		return DoneBadgeStyle.Render(icon)
	case "blocked":
		return BlockedBadgeStyle.Render(icon)
	case "canceled":
		return CanceledBadgeStyle.Render(icon)
	default:
		return NormalStyle.Render(icon)
	}
}

// GetStageBadge returns the stage name rendered as a styled badge.
func GetStageBadge(stage string) string {
	if stage == "" {
		return ""
	}
	return StageBadgeStyle.Render(stage)
}

// RenderProgressBar renders a simple progress bar.
func RenderProgressBar(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	empty := width - filled

	var bar strings.Builder
	for range filled {
		bar.WriteString(ProgressFullStyle.Render("█"))
	}
	for range empty {
		bar.WriteString(ProgressEmptyStyle.Render("░"))
	}

	return bar.String()
}

// GetViewModeStyleForSection returns the ViewMode style for a section name.
func GetViewModeStyleForSection(section string) lipgloss.Style {
	switch section {
	case "":
		return ViewModeActiveStyle
	case "Next":
		return ViewModeNextStyle
	case "Inbox":
		return ViewModeInboxStyle
	case "Waiting":
		return ViewModeWaitingStyle
	case "Someday":
		return ViewModeSomedayStyle
	case "Review":
		return ViewModeReviewStyle
	case "Standby":
		return ViewModeStandbyStyle
	case "All":
		return ViewModeAllStyle
	case "Active":
		return ViewModeActiveStyle
	case "Completed":
		return ViewModeCompletedStyle
	case "Snoozed":
		return ViewModeSnoozedStyle
	default:
		return ViewModeActiveStyle
	}
}

// RenderCursorPrefix returns the 3-character cursor prefix string based on row state.
func RenderCursorPrefix(selected, active, pinned, dimmed bool) string {
	activeMarker := ActiveStyle.Bold(true).Render("*")
	var prefix string
	switch {
	case selected && active && pinned:
		prefix = " " + activeMarker + PinnedMarker
	case selected && active:
		prefix = " " + activeMarker + " "
	case selected && pinned:
		prefix = "  " + PinnedMarker
	case selected:
		prefix = "   "
	case active && pinned:
		prefix = " " + activeMarker + PinnedMarker
	case active:
		prefix = " " + activeMarker + " "
	case pinned:
		prefix = "  " + PinnedMarker
	default:
		prefix = "   "
	}
	if dimmed && len(prefix) > 0 && prefix[0] == ' ' {
		prefix = "◌" + prefix[1:]
	}
	return prefix
}

// RenderSelectableRow renders a row with cursor prefix and optional full-width selection highlight.
func RenderSelectableRow(selected, active, pinned, flash bool, content string, width int, dimmed ...bool) string {
	isDimmed := len(dimmed) > 0 && dimmed[0]
	row := RenderCursorPrefix(selected, active, pinned, isDimmed) + content
	// Truncate to prevent terminal wrapping.
	if width > 0 {
		row = truncateToVisualWidth(row, width)
	}
	if isDimmed && selected {
		return DimSelectedLine(row, width)
	}
	if isDimmed {
		return DimLine(row)
	}
	if selected {
		if flash {
			return SelectedFlashLine(row, width)
		}
		return SelectedLine(row, width)
	}
	return row
}

// Hyperlink creates a clickable OSC 8 hyperlink in supported terminals.
func Hyperlink(url, displayText string) string {
	return "\033]8;;" + url + "\033\\" + displayText + "\033]8;;\033\\"
}

// PadBadge right-pads a pre-rendered badge string to a fixed visual width.
// It uses lipgloss.Width to measure the rendered width (handles ANSI sequences
// and multi-byte unicode). If rendered is already at or above width, it is
// returned unchanged.
func PadBadge(rendered string, width int) string {
	if width <= 0 {
		return rendered
	}
	current := lipgloss.Width(rendered)
	if current >= width {
		return rendered
	}
	return rendered + strings.Repeat(" ", width-current)
}
