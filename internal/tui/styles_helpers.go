package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// PriorityIcon returns the icon for a priority level (1-5).
func PriorityIcon(priority int) string {
	key := fmt.Sprintf("priority.%d", priority)
	if v := icons.Get(key); v != "" {
		return v
	}
	return icons.Get("priority.3")
}

// GetStateStyle returns the appropriate foreground style for a workflow state.
// Each state reuses the style of the legacy status it collapses into, matching
// GetStateBadge.
func GetStateStyle(state models.WorkflowState) lipgloss.Style {
	switch state {
	case models.StateInbox, models.StateReady, models.StateNext:
		return TodoStyle
	case models.StateInProgress:
		return InProgressStyle
	case models.StateWaiting:
		return BlockedStyle
	case models.StatePaused:
		return StandbyStyle
	case models.StateSomeday:
		return MaybeStyle
	case models.StateDone:
		return DoneStyle
	case models.StateCanceled:
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

// TaskTitleStyleOptions carries row/context state that can affect task title styling.
type TaskTitleStyleOptions struct {
	// CursorSelected uses the selected title style unless the task is disabled,
	// muted/dimmed, or canceled.
	CursorSelected bool
	// Muted covers context rows and explicitly muted title renderers.
	Muted bool
	// Disabled suppresses selected-title styling and falls back to muted text.
	Disabled bool
	// BulkSelected is tracked for callers but intentionally does not change title color.
	BulkSelected bool
}

// TaskTitleStyleFor returns the canonical style for a rendered task title.
func TaskTitleStyleFor(task *models.Task, opts TaskTitleStyleOptions) lipgloss.Style {
	if task == nil {
		return NormalStyle
	}

	isMuted := opts.Muted || opts.Disabled
	isCanceled := task.IsInState(models.StateCanceled)
	if opts.CursorSelected && !isMuted && !isCanceled {
		return SelectedTaskTitleStyle
	}
	if isMuted {
		return MutedStyle
	}
	if isCanceled {
		return CanceledStyle
	}
	return GetTitlePriorityStyle(task.Priority)
}

// GetStateBadge returns the workflow-state icon rendered as a styled badge
// with background. Each state reuses the badge style of the legacy status it
// collapses into (see WorkflowState.Icon): the three todo-ish planning states
// share the todo badge, waiting shows the blocked badge, paused the standby
// badge, and someday the maybe badge.
func GetStateBadge(state models.WorkflowState) string {
	switch state {
	case models.StateInbox, models.StateReady, models.StateNext:
		return TodoBadgeStyle.Render(state.Icon())
	case models.StateInProgress:
		return InProgressBadgeStyle.Render(state.Icon())
	case models.StateWaiting:
		return BlockedBadgeStyle.Render(state.Icon())
	case models.StatePaused:
		return StandbyBadgeStyle.Render(state.Icon())
	case models.StateSomeday:
		return MaybeBadgeStyle.Render(state.Icon())
	case models.StateDone:
		return DoneBadgeStyle.Render(state.Icon())
	case models.StateCanceled:
		return CanceledBadgeStyle.Render(state.Icon())
	default:
		return NormalStyle.Render(state.Icon())
	}
}

// completionFlashBg returns the background color for a given frame (0..2) of the
// task-completion ramp. The ramp goes saturated → bright → muted-success to give
// task completion a distinct peak feel relative to ordinary status mutations.
// Frames outside the 0..2 range clamp to the nearest end.
func completionFlashBg(frame int) color.Color {
	if frame < 0 {
		frame = 0
	}
	if frame > 2 {
		frame = 2
	}
	if activeThemeName == "light" {
		switch frame {
		case 0:
			return lipgloss.Color("#9ece6a")
		case 1:
			return lipgloss.Color("#c6f0a8")
		default:
			return lipgloss.Color("#d4edda")
		}
	}
	switch frame {
	case 0:
		return lipgloss.Color("#3f6a2a")
	case 1:
		return lipgloss.Color("#2b4d1a")
	default:
		return lipgloss.Color("#1a3825")
	}
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

// GetWorkflowStateGroupStyle resolves the canonical static theme role for a
// state group. Project section colors never participate.
func GetWorkflowStateGroupStyle(group models.WorkflowStateGroup) lipgloss.Style {
	switch group.ThemeRole() {
	case models.StateThemePlan, models.StateThemeBacklog:
		return ViewModeActiveStyle
	case models.StateThemeWork:
		return ViewModeWorkStyle
	case models.StateThemeInbox:
		return ViewModeInboxStyle
	case models.StateThemeNext:
		return ViewModeNextStyle
	case models.StateThemeWaiting:
		return ViewModeWaitingStyle
	case models.StateThemePaused:
		return ViewModeStandbyStyle
	case models.StateThemeSomeday:
		return ViewModeSomedayStyle
	case models.StateThemeComplete:
		return ViewModeCompletedStyle
	case models.StateThemeCanceled:
		return ViewModeCanceledStyle
	default:
		return ViewModeActiveStyle
	}
}

// GetWorkflowStateGroupAccent resolves the foreground/card accent for a
// canonical state group through the active theme.
func GetWorkflowStateGroupAccent(group models.WorkflowStateGroup) color.Color {
	switch group.ThemeRole() {
	case models.StateThemePlan, models.StateThemeNext:
		return AccentPlan
	case models.StateThemeWork:
		return AccentWork
	case models.StateThemeInbox:
		return AccentTriage
	case models.StateThemeBacklog:
		return AccentSupport
	case models.StateThemeWaiting:
		return ColorWarning
	case models.StateThemePaused, models.StateThemeSomeday:
		return ColorPurple
	case models.StateThemeComplete:
		return ColorSuccess
	case models.StateThemeCanceled:
		return ColorError
	default:
		return ColorMuted
	}
}

// GetOutlinerModeStyle resolves one of the final five mode identities to its
// static theme role. Workflow group headers use GetWorkflowStateGroupStyle.
func GetOutlinerModeStyle(mode OutlinerModeID) lipgloss.Style {
	switch mode {
	case ModeAll:
		return ViewModeAllStyle
	case ModeInbox:
		return GetWorkflowStateGroupStyle(models.StateGroupInbox)
	case ModeNext, ModeDue:
		return GetWorkflowStateGroupStyle(models.StateGroupNext)
	case ModeBacklog:
		return GetWorkflowStateGroupStyle(models.StateGroupBacklog)
	default:
		return ViewModeAllStyle
	}
}

// RenderCursorPrefix returns the 3-character cursor prefix string based on row state.
func RenderCursorPrefix(selected, pinned bool) string {
	var prefix string
	switch {
	case selected && pinned:
		prefix = "  " + PinnedMarker
	case selected:
		prefix = "   "
	case pinned:
		prefix = "  " + PinnedMarker
	default:
		prefix = "   "
	}
	return prefix
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

// EmptyBadge returns a whitespace placeholder sized to width. Used by row
// renderers to keep badge columns aligned when a row has no badge for a
// given column. Returns "" when width <= 0.
func EmptyBadge(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}
