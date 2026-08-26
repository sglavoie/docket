package tui

import (
	"fmt"
	"hash/fnv"
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
}

func semanticTaskTitleStyle(
	role models.SemanticTaskTagRole,
	priority int,
	selected bool,
	deferred bool,
) lipgloss.Style {
	style := GetTitlePriorityStyle(priority)
	var foreground color.Color
	switch {
	case selected:
		style = style.Bold(true)
		switch role {
		case models.SemanticTaskTagNone:
			return style
		case models.SemanticTaskTagBug:
			foreground = ColorSemanticBugSelectedTitle
		case models.SemanticTaskTagEpic:
			foreground = ColorSemanticEpicSelectedTitle
		case models.SemanticTaskTagStory:
			foreground = ColorSemanticStorySelectedTitle
		case models.SemanticTaskTagFeature:
			foreground = ColorSemanticFeatureSelectedTitle
		case models.SemanticTaskTagIdea:
			foreground = ColorSemanticIdeaSelectedTitle
		}
	case deferred:
		style = style.Faint(true)
		switch role {
		case models.SemanticTaskTagNone:
			return style
		case models.SemanticTaskTagBug:
			foreground = ColorSemanticBugDeferredTitle
		case models.SemanticTaskTagEpic:
			foreground = ColorSemanticEpicDeferredTitle
		case models.SemanticTaskTagStory:
			foreground = ColorSemanticStoryDeferredTitle
		case models.SemanticTaskTagFeature:
			foreground = ColorSemanticFeatureDeferredTitle
		case models.SemanticTaskTagIdea:
			foreground = ColorSemanticIdeaDeferredTitle
		}
	default:
		switch role {
		case models.SemanticTaskTagNone:
			return style
		case models.SemanticTaskTagBug:
			foreground = ColorSemanticBugTitle
		case models.SemanticTaskTagEpic:
			foreground = ColorSemanticEpicTitle
		case models.SemanticTaskTagStory:
			foreground = ColorSemanticStoryTitle
		case models.SemanticTaskTagFeature:
			foreground = ColorSemanticFeatureTitle
		case models.SemanticTaskTagIdea:
			foreground = ColorSemanticIdeaTitle
		}
	}
	return style.Foreground(foreground)
}

func taskContextBadgeStyle(task *models.Task) lipgloss.Style {
	style := ContextBadgeStyle
	if task != nil && task.IsDeferred() {
		style = style.Foreground(ColorTaskContextDeferred).Faint(true)
	}
	return style
}

// tagChipSlot resolves a tag name to its index in the chip ramp. The normalized
// spelling is what hashes, so Bug and bug are one chip while the stored spelling
// still reaches the screen. The hash is FNV-1a from the standard library because
// it is specified rather than merely implemented: a tag keeps its color across
// Go versions, machines, and workspaces, which is the whole promise of a
// mnemonic. maphash would not — its seed varies per process.
//
// The mapping is fixed per name, so a workspace that gains tags never repaints
// the ones it already had; new tags simply land wherever their name hashes,
// collisions included. Color is a mnemonic, not a legend: two unrelated tags
// sharing slot 3 is expected, not a bug.
func tagChipSlot(tag string) int {
	if len(tagChipColors) == 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(models.NormalizeTag(tag)))
	return int(h.Sum32() % uint32(len(tagChipColors)))
}

// tagChipStyle returns the one style every rendered tag chip wears. Foreground
// only: chips carry no bold, so bold stays the title's selection channel.
func tagChipStyle(tag string) lipgloss.Style {
	if len(tagChipColors) == 0 {
		return TagInlineStyle
	}
	return lipgloss.NewStyle().Foreground(tagChipColors[tagChipSlot(tag)])
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
	if entry, ok := models.SemanticTaskTagForTags(task.Tags); ok {
		return semanticTaskTitleStyle(entry.Role, task.Priority, opts.CursorSelected, task.IsDeferred())
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

// outlinerModeStateGroup names the workflow-state group a mode borrows its
// semantic color from. All has none of its own, and neither does an unknown
// token. This is the single mode-to-group table: both the badge style below and
// the named-lens accent read it rather than restating it.
func outlinerModeStateGroup(mode OutlinerModeID) (models.WorkflowStateGroup, bool) {
	switch mode {
	case ModeInbox:
		return models.StateGroupInbox, true
	case ModeToday:
		return models.StateGroupToday, true
	case ModeNext, ModeDue:
		return models.StateGroupNext, true
	case ModeBacklog:
		return models.StateGroupBacklog, true
	case ModeRecent:
		return models.StateGroupDoing, true
	default:
		return "", false
	}
}

// GetOutlinerModeStyle resolves one of the final seven mode identities to its
// static theme role. Workflow group headers use GetWorkflowStateGroupStyle.
func GetOutlinerModeStyle(mode OutlinerModeID) lipgloss.Style {
	if group, ok := outlinerModeStateGroup(mode); ok {
		return GetWorkflowStateGroupStyle(group)
	}
	return ViewModeAllStyle
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
