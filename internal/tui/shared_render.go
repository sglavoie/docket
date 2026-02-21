package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// EmptyStateFiltered is the standard message shown when active filters produce no results.
const EmptyStateFiltered = "No tasks match current filters."

// EmptyStateNoTasks is the standard message shown when a view has no tasks at all.
const EmptyStateNoTasks = "No tasks."

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
		display := tag
		if len(display) > maxTagLen {
			display = display[:maxTagLen-1] + "…"
		}
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
		return MutedStyle.Render(fmt.Sprintf("💬%d/%d", unresolved, len(task.Comments)))
	}
	return MutedStyle.Render(fmt.Sprintf("💬%d", len(task.Comments)))
}

// renderAttachmentIndicator renders a compact attachment count indicator.
func renderAttachmentIndicator(task *models.Task) string {
	if !task.HasAttachments() {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("📎%d", task.AttachmentCount()))
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
	HideLowPriority bool   // only show priority badge for priority ≤ 2
}

// CrossProjectTaskLineOpts configures optional elements for a cross-project task line.
// Views that aggregate tasks from multiple projects use this to render a standard
// [project-name] badge plus optional tags/score in a consistent format.
type CrossProjectTaskLineOpts struct {
	ShowScore       bool   // show composite score badge (requires cfg != nil)
	ShowTags        bool   // show inline tag string
	MaxProjectLen   int    // project name truncation; 0 defaults to 12
	Prefix          string // prepended before the project badge (e.g. checkbox, date string)
	StatusBadge     string // pre-styled status badge override
	DueStr          string // right-aligned context string, already styled by caller
	HideLowPriority bool   // omit priority badge when priority > 2
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

	projectBadge := MutedStyle.Render(fmt.Sprintf("[%s]", util.Truncate(proj.Name, maxProjLen)))
	preBadges := opts.Prefix + projectBadge + " "

	postTitle := ""
	if opts.ShowTags {
		postTitle += renderInlineTags(task.Tags)
	}
	if opts.ShowScore && cfg != nil {
		if profile := cfg.GetProjectScoringProfile(proj.ID); profile != nil {
			if badge := RenderScoreBadge(task, profile); badge != "" {
				postTitle += " " + badge
			}
		}
	}

	return TaskLine{
		Task:            task,
		IsSelected:      isCursor,
		MaxTitle:        maxTitleLen,
		PreBadges:       preBadges,
		PostTitle:       postTitle,
		StatusBadge:     opts.StatusBadge,
		DueStr:          opts.DueStr,
		ViewWidth:       width,
		HideLowPriority: opts.HideLowPriority,
	}.Render()
}

// Render assembles the task line string from the configured fields.
func (tl TaskLine) Render() string {
	cursor := "  "
	if tl.IsSelected {
		cursor = "> "
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

	maxTitleLen := tl.MaxTitle
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(tl.Task.Title, maxTitleLen)

	// Build priority segment: "badge " when present, "" when hidden
	prioritySegment := ""
	if priorityBadge != "" {
		prioritySegment = priorityBadge + " "
	}

	line := fmt.Sprintf("%s%s%s%s %s %s%s%s", tl.Prefix, cursor, tl.PreBadges, statusIcon, typeIcon, prioritySegment, title, tl.PostTitle)
	if tl.DueStr != "" {
		line = fmt.Sprintf("%-*s %s", tl.ViewWidth-15, line, tl.DueStr)
	}

	if tl.IsSelected {
		if tl.ViewWidth > 0 {
			line = SelectionRowStyle.Width(tl.ViewWidth).Render(line)
		} else {
			line = SelectedStyle.Render(line)
		}
	}

	return line
}
