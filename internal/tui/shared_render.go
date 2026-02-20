package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

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
		line = SelectedStyle.Render(line)
	}

	return line
}
