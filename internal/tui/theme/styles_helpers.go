package theme

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/icons"
	"github.com/sglavoie/docket/internal/models"
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

// SemanticTaskTitleStyle resolves the color the six-member semantic vocabulary
// reserves for one of its roles. It is the vocabulary's own accent, not a task
// title's: no task title wears one. The `SEMANTIC` picker rows and task status
// icons use this accent to identify the kind of work.
//
// The name and the palette keys behind it — ColorSemanticBugTitle and its
// selected and deferred variants — are historical. They stay as they are because
// the palette is a published surface: renaming a color would rewrite a stored
// vocabulary to record that a renderer stopped calling it.
func SemanticTaskTitleStyle(
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
		case models.SemanticTaskTagQuestion:
			foreground = ColorSemanticQuestionSelectedTitle
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
		case models.SemanticTaskTagQuestion:
			foreground = ColorSemanticQuestionDeferredTitle
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
		case models.SemanticTaskTagQuestion:
			foreground = ColorSemanticQuestionTitle
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

func TaskContextBadgeStyle(task *models.Task) lipgloss.Style {
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

// TagChipStyle returns the one style every rendered tag chip wears. Foreground
// only: chips carry no bold, so bold stays the title's selection channel.
func TagChipStyle(tag string) lipgloss.Style {
	if len(tagChipColors) == 0 {
		return TagInlineStyle
	}
	return lipgloss.NewStyle().Foreground(tagChipColors[tagChipSlot(tag)])
}

// TaskTitleStyleFor returns the canonical style for a rendered task title, and
// a task title is neutral. Nothing about how loudly a task is asking reaches it:
// priority is the priority badge's to say, a deadline the due chip's, an owed
// review the review segment's, and a semantic tag its chip's. A title that also
// carried those signals spent the row's widest and most-read region restating
// what the badges beside it already said, and left the one region a reader
// cannot skip competing with itself.
//
// What remains is lifecycle alone, because those three are statements about
// whether the text should be read at all rather than about urgency: a muted or
// disabled row is context, a canceled task is struck through, and a cursor-selected
// row on a surface that inverts nothing else says so on its title. Every other
// task reads in ordinary text.
func TaskTitleStyleFor(task *models.Task, opts TaskTitleStyleOptions) lipgloss.Style {
	if task == nil {
		return NormalStyle
	}

	if opts.Muted || opts.Disabled {
		return MutedStyle
	}
	if task.IsInState(models.StateCanceled) {
		return CanceledStyle
	}
	if opts.CursorSelected {
		return SelectedTaskTitleStyle
	}
	return NormalStyle
}

// GetStateBadge returns the workflow-state icon rendered as a styled badge.
// Each state reuses the badge style of the legacy status it
// collapses into (see WorkflowState.Icon): the three todo-ish planning states
// share the todo badge, waiting uses its own badge, paused the standby
// badge, and someday the maybe badge.
func GetStateBadge(state models.WorkflowState) string {
	return readableStateBadgeStyle(stateBadgeStyle(state)).Render(stateBadgeIcon(state))
}

// GetTaskStateBadge keeps the workflow-state symbol and badge styling, but uses
// the highest-precedence semantic tag's accent when the task has one.
func GetTaskStateBadge(task *models.Task) string {
	return taskStateBadgeStyle(task).Render(stateBadgeIcon(task.State()))
}

func stateBadgeIcon(state models.WorkflowState) string {
	if state == models.StateBacklog {
		// Terminal bold alone often leaves the ordinary square's outline thin.
		return icons.Get("status.backlog.bold")
	}
	return state.Icon()
}

func taskStateBadgeStyle(task *models.Task) lipgloss.Style {
	style := stateBadgeStyle(task.State())
	role := models.SemanticTaskTagNone
	for _, tag := range task.Tags {
		if entry, ok := models.SemanticTaskTagByName(tag); ok && (role == models.SemanticTaskTagNone || entry.Role < role) {
			role = entry.Role
		}
	}
	if role != models.SemanticTaskTagNone {
		style = style.Foreground(SemanticTaskTitleStyle(role, 0, false, false).GetForeground())
	}
	return readableStateBadgeStyle(style)
}

// Status icons use a vivid light-theme palette independent of text accents.
// The surrounding row supplies the background, including its cursor tint.
func readableStateBadgeStyle(style lipgloss.Style) lipgloss.Style {
	if ActiveThemeName != "light" {
		return style
	}
	hex := "#2b2f36"
	switch style.GetForeground() {
	case ColorSemanticBugTitle, ColorError:
		hex = "#f02318"
	case ColorSemanticQuestionTitle:
		hex = "#e06400"
	case ColorSemanticEpicTitle, StandbyBadgeStyle.GetForeground(), MaybeBadgeStyle.GetForeground():
		hex = "#a020d0"
	case ColorSemanticStoryTitle:
		hex = "#087de0"
	case ColorSemanticFeatureTitle, ColorSuccess:
		hex = "#00a632"
	case ColorSemanticIdeaTitle, ColorWarning:
		hex = "#bc8e00"
	}
	return style.
		UnsetBackground().
		Foreground(lipgloss.Color(hex)).
		Bold(true)
}

func stateBadgeStyle(state models.WorkflowState) lipgloss.Style {
	switch state {
	case models.StateInbox, models.StateBacklog:
		return TodoBadgeStyle
	case models.StateDoing:
		return InProgressBadgeStyle
	case models.StateWaiting:
		return WaitingBadgeStyle
	case models.StatePaused:
		return StandbyBadgeStyle
	case models.StateSomeday:
		return MaybeBadgeStyle
	case models.StateDone:
		return DoneBadgeStyle
	case models.StateCanceled:
		return CanceledBadgeStyle
	default:
		return NormalStyle
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

// StateGroupInlineBadgeStyle is a state group's section badge — fill, ink, and
// bold — without the badge's padding. It lets a value spelled inside a sentence,
// such as capture's route row and preview, read as the badge its section wears
// while the text keeps the width it has uncolored.
func StateGroupInlineBadgeStyle(group models.WorkflowStateGroup) lipgloss.Style {
	return GetWorkflowStateGroupStyle(group).Padding(0)
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
