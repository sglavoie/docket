package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// PriorityIcon returns the icon for an assigned priority level (1-3). Unset and
// invalid values intentionally render no badge.
func PriorityIcon(priority int) string {
	if models.ValidatePriority(priority, false) != nil {
		return ""
	}
	key := fmt.Sprintf("priority.%d", priority)
	return icons.Get(key)
}

// GetPriorityStyle returns the appropriate style for a priority level.
// Delegates to GetTitlePriorityStyle for unified palette-driven color sourcing.
func GetPriorityStyle(priority int) lipgloss.Style {
	return GetTitlePriorityStyle(priority)
}

// GetTitlePriorityStyle returns the title style for priority 1-3. Unset and
// invalid values use the normal task-title style.
func GetTitlePriorityStyle(priority int) lipgloss.Style {
	switch priority {
	case 1:
		return TitlePriority1Style
	case 2:
		return TitlePriority2Style
	case 3:
		return TitlePriority3Style
	default:
		return NormalStyle
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

type semanticTitleRole uint8

const (
	semanticTitleNone semanticTitleRole = iota
	semanticTitleBug
	semanticTitleFeature
	semanticTitleIdea
)

// semanticTitleRoleForTags recognizes only exact case-insensitive tag values.
// Slice order never affects the locked bug > feature > idea precedence.
func semanticTitleRoleForTags(tags []string) semanticTitleRole {
	var feature, idea bool
	for _, tag := range tags {
		switch {
		case strings.EqualFold(tag, "bug"):
			return semanticTitleBug
		case strings.EqualFold(tag, "feature"):
			feature = true
		case strings.EqualFold(tag, "idea"):
			idea = true
		}
	}
	if feature {
		return semanticTitleFeature
	}
	if idea {
		return semanticTitleIdea
	}
	return semanticTitleNone
}

func semanticTaskTitleStyle(
	role semanticTitleRole,
	priority int,
	selected bool,
	snoozed bool,
) lipgloss.Style {
	style := GetTitlePriorityStyle(priority)
	var foreground color.Color
	switch {
	case selected:
		style = style.Bold(true)
		switch role {
		case semanticTitleNone:
			return style
		case semanticTitleBug:
			foreground = ColorSemanticBugSelectedTitle
		case semanticTitleFeature:
			foreground = ColorSemanticFeatureSelectedTitle
		case semanticTitleIdea:
			foreground = ColorSemanticIdeaSelectedTitle
		}
	case snoozed:
		style = style.Faint(true)
		switch role {
		case semanticTitleNone:
			return style
		case semanticTitleBug:
			foreground = ColorSemanticBugSnoozedTitle
		case semanticTitleFeature:
			foreground = ColorSemanticFeatureSnoozedTitle
		case semanticTitleIdea:
			foreground = ColorSemanticIdeaSnoozedTitle
		}
	default:
		switch role {
		case semanticTitleNone:
			return style
		case semanticTitleBug:
			foreground = ColorSemanticBugTitle
		case semanticTitleFeature:
			foreground = ColorSemanticFeatureTitle
		case semanticTitleIdea:
			foreground = ColorSemanticIdeaTitle
		}
	}
	return style.Foreground(foreground)
}

// TaskTitleStyleFor returns the canonical style for a rendered task title.
func TaskTitleStyleFor(task *models.Task, opts TaskTitleStyleOptions) lipgloss.Style {
	if task == nil {
		return NormalStyle
	}

	isMuted := opts.Muted || opts.Disabled
	isCanceled := task.IsInState(models.StateCanceled)
	if isMuted {
		return MutedStyle
	}
	if isCanceled {
		return CanceledStyle
	}
	if role := semanticTitleRoleForTags(task.Tags); role != semanticTitleNone {
		return semanticTaskTitleStyle(role, task.Priority, opts.CursorSelected, task.IsSnoozed())
	}
	if opts.CursorSelected {
		return SelectedTaskTitleStyle
	}
	return GetTitlePriorityStyle(task.Priority)
}

// GetStateBadge returns the workflow-state icon rendered as a styled badge
// with background. Each state reuses the badge style of the legacy status it
// collapses into (see WorkflowState.Icon): the three todo-ish planning states
// share the todo badge, waiting uses its own badge, paused the standby
// badge, and someday the maybe badge.
func GetStateBadge(state models.WorkflowState) string {
	switch state {
	case models.StateInbox, models.StateBacklog:
		return TodoBadgeStyle.Render(state.Icon())
	case models.StateDoing:
		return InProgressBadgeStyle.Render(state.Icon())
	case models.StateWaiting:
		return WaitingBadgeStyle.Render(state.Icon())
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

// GetOutlinerModeStyle resolves one of the final six mode identities to its
// static theme role. Workflow group headers use GetWorkflowStateGroupStyle.
func GetOutlinerModeStyle(mode OutlinerModeID) lipgloss.Style {
	switch mode {
	case ModeAll:
		return ViewModeAllStyle
	case ModeInbox:
		return GetWorkflowStateGroupStyle(models.StateGroupInbox)
	case ModeToday:
		return GetWorkflowStateGroupStyle(models.StateGroupToday)
	case ModeNext, ModeDue:
		return GetWorkflowStateGroupStyle(models.StateGroupNext)
	case ModeBacklog:
		return GetWorkflowStateGroupStyle(models.StateGroupBacklog)
	default:
		return ViewModeAllStyle
	}
}

// RenderCursorPrefix returns the minimal left margin before a task row. Cursor
// selection is expressed by the row highlight, so no wider marker gutter is
// needed.
func RenderCursorPrefix() string {
	return " "
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
