package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// IndicatorGroupSep is the dim separator rendered between indicator groups.
var IndicatorGroupSep = MutedStyle.Render(" · ")

// JoinIndicatorGroups joins non-empty group strings with a dim "·" separator.
// Empty groups are skipped so no orphan separators appear.
func JoinIndicatorGroups(groups ...string) string {
	var nonEmpty []string
	for _, g := range groups {
		if g != "" {
			nonEmpty = append(nonEmpty, g)
		}
	}
	return strings.Join(nonEmpty, IndicatorGroupSep)
}

// JoinGroupParts joins non-empty parts within one indicator group with a space.
func JoinGroupParts(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, " ")
}

// FormatCyclePosition returns a " [i/n]" suffix if projectID appears in the
// configured cycle list, or "" otherwise.
func FormatCyclePosition(cfg *config.Config, projectID string) string {
	cycleProjects := cfg.GetCycleProjects()
	for i, p := range cycleProjects {
		if p.ID == projectID {
			return fmt.Sprintf(" [%d/%d]", i+1, len(cycleProjects))
		}
	}
	return ""
}

// EmptyStateFiltered is the standard message shown when active filters produce no results.
const EmptyStateFiltered = "No tasks match filters. Press 'F' then 'x' to clear."

// EmptyStateNoTasks is the standard message shown when a view has no tasks at all.
const EmptyStateNoTasks = "No tasks yet. Press 'a' to add one."

// EmptyStateClosed is the standard message shown when the closed/done tab has no tasks.
const EmptyStateClosed = "No closed tasks."

// EmptyStateNoMatch is the standard message shown when a tab filter produces no results.
const EmptyStateNoMatch = "No tasks match this tab."

// renderTruncatedTags renders a compact inline tag string with per-tag truncation
// and overflow count. Shows at most maxTags tags, each truncated to maxTagLen chars.
// Returns an empty string if there are no tags.
func renderTruncatedTags(tags []string, maxTags, maxTagLen int) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	for i, tag := range tags {
		if i >= maxTags {
			break
		}
		display := truncateToVisualWidth(tag, maxTagLen)
		parts = append(parts, "#"+display)
	}
	result := TagInlineStyle.Render(strings.Join(parts, " "))
	if len(tags) > maxTags {
		result += MutedStyle.Render(fmt.Sprintf(" +%d", len(tags)-maxTags))
	}
	return result
}

// renderCommentIndicator renders a compact comment count indicator.
// Shows unresolved/total when some are resolved, otherwise just total.
func renderCommentIndicator(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	unresolved := task.UnresolvedCommentCount()
	if unresolved < len(task.Comments) {
		return MutedStyle.Render(fmt.Sprintf("💬 %d/%d", unresolved, len(task.Comments)))
	}
	return MutedStyle.Render(fmt.Sprintf("💬 %d", len(task.Comments)))
}

// renderAttachmentIndicator renders a compact attachment count indicator.
func renderAttachmentIndicator(task *models.Task) string {
	if !task.HasAttachments() {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("📎 %d", task.AttachmentCount()))
}

// renderChecklistIndicator renders a compact checklist progress indicator.
func renderChecklistIndicator(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	completed := 0
	for _, item := range task.Checklist {
		if item.Done {
			completed++
		}
	}
	return MutedStyle.Render(fmt.Sprintf("✅%d/%d", completed, len(task.Checklist)))
}

// renderProgressFraction renders a compact [done/total] fraction for parent tasks.
// Returns an empty string if the task has no children.
func renderProgressFraction(task *models.Task) string {
	if !task.HasProgressChildren() {
		return ""
	}
	completed := 0
	for _, child := range task.Children {
		if child.IsComplete() {
			completed++
		}
	}
	return MutedStyle.Render(fmt.Sprintf("[%d/%d]", completed, len(task.Children)))
}

// renderDependencyIndicators renders compact dependency arrow indicators.
func renderDependencyIndicators(task *models.Task, project *models.Project) string {
	var parts []string
	if len(task.DependsOn) > 0 {
		parts = append(parts, DependencyStyle.Render(fmt.Sprintf("→%d", len(task.DependsOn))))
	}
	if blockedBy := project.GetBlockedByTasks(task); len(blockedBy) > 0 {
		parts = append(parts, DependencyStyle.Render(fmt.Sprintf("←%d", len(blockedBy))))
	}
	return strings.Join(parts, " ")
}

// TaskLine holds the configuration for rendering a single task line.
// Callers set the fields they need and call Render() to produce the line string.
// The core badge sequence (status → type → priority → title) is always rendered.
type TaskLine struct {
	Task       *models.Task
	IsSelected bool
	MaxTitle   int // maximum title width; clamped to minimum 10

	// Optional elements
	Prefix          string // before cursor (e.g., star indicator "★ ")
	PreBadges       string // after cursor, before status badge (e.g., project name, checkbox, rank)
	PostTitle       string // after title (e.g., tags, score badge, custom section)
	DueStr          string // right-aligned due date string (pre-styled by caller)
	ViewWidth       int    // total view width for due date right-alignment
	StatusBadge     string // override computed status badge (pre-styled by caller)
	StageBadge      string // pre-styled stage badge (from GetStageBadge)
	HideLowPriority bool   // only show priority badge for priority ≤ 2
	Flash           bool   // cursor flash highlight active
	SearchQuery     string // when set, highlight matching substring in title
}

// CrossProjectTaskLineOpts configures optional elements for a cross-project task line.
// Views that aggregate tasks from multiple projects use this to render a standard
// [project-name] badge plus optional tags/score in a consistent format.
type CrossProjectTaskLineOpts struct {
	ShowScore       bool        // show composite score badge (requires cfg != nil)
	ShowTags        bool        // show inline tag string
	ShowRecurrence  bool        // show ↻ indicator with streak count for recurring tasks
	MaxProjectLen   int         // project name truncation; 0 defaults to 12
	Prefix          string      // prepended before the project badge (e.g. checkbox, date string)
	StatusBadge     string      // pre-styled status badge override
	DueStr          string      // right-aligned context string, already styled by caller
	HideLowPriority bool        // omit priority badge when priority > 2
	Flash           bool        // cursor flash highlight active
	SearchQuery     string      // when set, highlight matching substring in title
	BadgeColor      color.Color // when set, overrides MutedStyle on the [project-name] badge
}

// RenderCrossProjectTaskLine renders a standard task line for views that aggregate
// tasks from multiple projects. It builds the [project] badge, optionally appends
// tags and a score badge, and delegates to TaskLine.Render().
//
// Use opts.Prefix to prepend content before the project badge (e.g. a checkbox or
// a pre-styled date string). Use opts.DueStr for the right-aligned context column.
// For non-standard badge arrangements, construct TaskLine directly instead.
func RenderCrossProjectTaskLine(
	task *models.Task,
	proj *models.Project,
	cfg *config.Config,
	isCursor bool,
	width, maxTitleLen int,
	opts CrossProjectTaskLineOpts,
) string {
	maxProjLen := opts.MaxProjectLen
	if maxProjLen <= 0 {
		maxProjLen = 12
	}

	badgeStyle := MutedStyle
	if opts.BadgeColor != nil {
		badgeStyle = lipgloss.NewStyle().Foreground(opts.BadgeColor)
	}
	projectBadge := badgeStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(proj.Name, maxProjLen)))
	preBadges := opts.Prefix + projectBadge + " "

	// Build indicator groups
	structureGroup := renderProgressFraction(task)

	recurStr := ""
	if opts.ShowRecurrence && task.IsRecurring() {
		recurStr = "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		recurStr = MutedStyle.Render(recurStr)
	}
	lifecycleGroup := JoinGroupParts(recurStr)

	tagsStr := ""
	if opts.ShowTags {
		tagsStr = renderTruncatedTags(task.Tags, 2, 8)
	}
	scoreBadge := ""
	if opts.ShowScore && cfg != nil {
		if profile := cfg.GetProjectScoringProfile(proj.ID); profile != nil {
			scoreBadge = RenderScoreBadge(task, profile)
		}
	}
	contentGroup := JoinGroupParts(tagsStr, scoreBadge)

	postTitle := JoinIndicatorGroups(structureGroup, lifecycleGroup, contentGroup)
	if postTitle != "" {
		postTitle = " " + postTitle
	}

	return TaskLine{
		Task:            task,
		IsSelected:      isCursor,
		MaxTitle:        maxTitleLen,
		PreBadges:       preBadges,
		PostTitle:       postTitle,
		StatusBadge:     opts.StatusBadge,
		StageBadge:      GetStageBadge(task.Stage),
		DueStr:          opts.DueStr,
		ViewWidth:       width,
		HideLowPriority: opts.HideLowPriority,
		Flash:           opts.Flash,
		SearchQuery:     opts.SearchQuery,
	}.Render()
}

// DimLine strips all ANSI styling and re-renders as dim text.
// Call after full line assembly, before selection styling.
// The caller is responsible for placing a "◌" indicator in the cursor prefix
// position so that DimLine does not alter the line width.
func DimLine(line string) string {
	stripped := ansi.Strip(line)
	return DimmedTaskStyle.Render(stripped)
}

// DimSelectedLine strips all ANSI styling and re-renders as dim+selected text.
// Used when a dimmed task is under the cursor to keep it visually distinct from
// normal selected rows. The cursor "→" is preserved in the stripped text.
func DimSelectedLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return DimmedSelectedStyle.Width(width).Render(stripped)
}

// Render assembles the task line string from the configured fields.
func (tl TaskLine) Render() string {
	cursor := "  "
	if tl.IsSelected {
		cursor = "→ "
	} else if tl.Task.Dimmed {
		cursor = "◌ "
	}

	statusIcon := tl.StatusBadge
	if statusIcon == "" {
		statusIcon = GetStatusBadge(string(tl.Task.Status), tl.Task.Status.Icon())
	}
	typeIcon := GetTypeBadge(string(tl.Task.Type), tl.Task.Type.Icon())
	priorityBadge := renderPriorityBadge(tl.Task.Priority)
	if tl.HideLowPriority && tl.Task.Priority > 2 {
		priorityBadge = ""
	}

	maxTitleLen := max(tl.MaxTitle, 10)
	title := truncateToVisualWidth(tl.Task.Title, maxTitleLen)
	if tl.SearchQuery != "" {
		title = HighlightSubstring(title, tl.SearchQuery, NormalStyle)
	}
	if tl.IsSelected && !tl.Task.Dimmed {
		title = SelectedTaskTitleStyle.Render(title)
	}

	// Build priority segment: "badge " when present, "" when hidden
	prioritySegment := ""
	if priorityBadge != "" {
		prioritySegment = priorityBadge + " "
	}

	// Build stage segment: "badge " when present, "" when empty
	stageSegment := ""
	if tl.StageBadge != "" {
		stageSegment = tl.StageBadge + " "
	}

	line := fmt.Sprintf("%s%s%s%s %s %s%s%s%s", tl.Prefix, cursor, tl.PreBadges, statusIcon, typeIcon, stageSegment, prioritySegment, title, tl.PostTitle)
	if tl.DueStr != "" {
		if tl.ViewWidth > 0 {
			leftWidth := max(tl.ViewWidth-lipgloss.Width(tl.DueStr)-1, 1)
			line = truncateToVisualWidth(line, leftWidth)
			line += strings.Repeat(" ", max(leftWidth-lipgloss.Width(line), 0)) + " " + tl.DueStr
		} else {
			line += " " + tl.DueStr
		}
	}

	if tl.Task.Dimmed && !tl.IsSelected {
		line = DimLine(line)
	} else if tl.Task.Dimmed && tl.IsSelected {
		line = DimSelectedLine(line, tl.ViewWidth)
	}

	if tl.IsSelected && (tl.Task == nil || !tl.Task.Dimmed) {
		if tl.ViewWidth > 0 {
			style := SelectionRowStyle
			if tl.Flash {
				style = CursorFlashRowStyle
			}
			line = style.Width(tl.ViewWidth).Render(line)
		} else {
			line = SelectedStyle.Render(line)
		}
	}

	return line
}
