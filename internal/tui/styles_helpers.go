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
	case models.StateReview:
		return ReviewStyle
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

	isMuted := opts.Muted || opts.Disabled || task.Dimmed
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
	case models.StateReview:
		return ReviewBadgeStyle.Render(state.Icon())
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

// stateMutationFlashBg returns a subtle background color for a
// workflow-state-specific mutation flash.
func stateMutationFlashBg(state models.WorkflowState) color.Color {
	if activeThemeName == "light" {
		switch state {
		case models.StateDone:
			return lipgloss.Color("#d4edda")
		case models.StateWaiting:
			return lipgloss.Color("#f8d7da")
		case models.StateInProgress:
			return lipgloss.Color("#fff3cd")
		case models.StateReview:
			return lipgloss.Color("#e6d6f8")
		default:
			return ColorFlashBg
		}
	}
	switch state {
	case models.StateDone:
		return lipgloss.Color("#1a3825")
	case models.StateWaiting:
		return lipgloss.Color("#3d1515")
	case models.StateInProgress:
		return lipgloss.Color("#3d2e00")
	case models.StateReview:
		return lipgloss.Color("#1f1f3d")
	default:
		return ColorFlashBg
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

// priorityMutationFlashBg returns a subtle background tint for a priority-specific mutation flash.
func priorityMutationFlashBg(priority int) color.Color {
	if activeThemeName == "light" {
		switch priority {
		case 1:
			return lipgloss.Color("#f8d7da")
		case 2:
			return lipgloss.Color("#fff3cd")
		case 3:
			return lipgloss.Color("#dbe8ff")
		default:
			return ColorFlashBg
		}
	}
	switch priority {
	case 1:
		return lipgloss.Color("#3d1a20")
	case 2:
		return lipgloss.Color("#3d2e00")
	case 3:
		return lipgloss.Color("#182035")
	default:
		return ColorFlashBg
	}
}

// typeMutationFlashBg returns a subtle background tint for a type-specific mutation flash.
func typeMutationFlashBg(taskType models.TaskType) color.Color {
	if activeThemeName == "light" {
		switch taskType {
		case models.TaskTypeEpic:
			return lipgloss.Color("#ede0ff")
		case models.TaskTypeBug:
			return lipgloss.Color("#f8d7da")
		case models.TaskTypeIdea:
			return lipgloss.Color("#fff3cd")
		default:
			return ColorFlashBg
		}
	}
	switch taskType {
	case models.TaskTypeEpic:
		return lipgloss.Color("#2b1f45")
	case models.TaskTypeBug:
		return lipgloss.Color("#3d1520")
	case models.TaskTypeIdea:
		return lipgloss.Color("#3d2e00")
	default:
		return ColorFlashBg
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
// overdue and blocked apply a subtle background tint when the row is not selected or dimmed.
func RenderSelectableRow(selected, active, pinned, flash, overdue, blocked bool, content string, width int, dimmed ...bool) string {
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
	if blocked {
		return BlockedRowStyle.Width(width).Render(row)
	}
	if overdue {
		return OverdueRowStyle.Width(width).Render(row)
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

// EmptyBadge returns a whitespace placeholder sized to width. Used by row
// renderers to keep badge columns aligned when a row has no badge for a
// given column. Returns "" when width <= 0.
func EmptyBadge(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}
