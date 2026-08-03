package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func formatSnoozeDate(snoozedUntil time.Time) string {
	if snoozedUntil.Year() != time.Now().Year() {
		return snoozedUntil.Format("Jan 2 '06")
	}
	return snoozedUntil.Format("Jan 2")
}

// JoinGroupParts joins non-empty parts within one indicator group with a space.
func JoinGroupParts(parts ...string) string {
	var nonEmpty []string
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, " ")
}

// EmptyStateFiltered is the standard message shown when active filters produce no results.
const EmptyStateFiltered = "No tasks match filters. Press fx to clear filters."

// EmptyStateNoTasks is the standard message shown when a view has no tasks at all.
const EmptyStateNoTasks = "No tasks yet. Press a to capture one."

// SectionHeaderOpts configures the unified section header renderer.
type SectionHeaderOpts struct {
	Name        string
	Count       int
	Collapsed   bool
	AccentColor lipgloss.Style // applied to the name portion
	Width       int            // if > 0, trailing ─ fill is added
	Prefix      string         // inserted between indicator and name (e.g. "@ ")
}

// renderSectionHeader renders a standardized collapsible section header:
//
//	▾ [prefix]Name (count) ─────
//
// AccentColor is applied to the name; indicator and count use MutedStyle.
// When Width > 0 a trailing horizontal rule fills the remaining line width.
// Callers are responsible for applying selection/zebra highlights on the
// returned string.
func renderSectionHeader(opts SectionHeaderOpts) string {
	indicator := IconExpanded
	if opts.Collapsed {
		indicator = IconCollapsed
	}
	indPart := MutedStyle.Render(indicator)
	namePart := opts.AccentColor.Bold(true).Render(opts.Prefix + opts.Name)
	countPart := MutedStyle.Render(fmt.Sprintf("(%d)", opts.Count))

	line := indPart + " " + namePart + " " + countPart

	if opts.Width > 0 {
		visible := lipgloss.Width(line)
		if opts.Width > visible+2 {
			line += " " + MutedStyle.Render(strings.Repeat("─", opts.Width-visible-1))
		}
	}
	return line
}

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
func renderCommentIndicator(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconComment(), len(task.Comments)))
}

// renderAttachmentIndicator renders a compact attachment count indicator.
func renderAttachmentIndicator(task *models.Task) string {
	if !task.HasAttachments() {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconAttachment(), task.AttachmentCount()))
}

func formatGitHubIssueRef(ref models.GitHubIssueRef) string {
	return fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Number)
}

func gitHubIssueAttachmentRef(att models.Attachment) (models.GitHubIssueRef, bool) {
	if !att.IsURL {
		return models.GitHubIssueRef{}, false
	}
	if ref, ok := models.ParseGitHubIssueURL(att.Filename); ok {
		return ref, true
	}
	if att.OriginalPath != "" && att.OriginalPath != att.Filename {
		return models.ParseGitHubIssueURL(att.OriginalPath)
	}
	return models.GitHubIssueRef{}, false
}

func attachmentDisplayName(att models.Attachment) string {
	if ref, ok := gitHubIssueAttachmentRef(att); ok {
		return formatGitHubIssueRef(ref)
	}
	if att.IsCodeRef {
		return fmt.Sprintf("%s:%d", att.OriginalPath, att.LineNumber)
	}
	return att.Filename
}

// renderChecklistIndicator renders a compact checklist progress indicator.
func renderChecklistIndicator(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	completed, total := checklistCounts(task)
	return MutedStyle.Render(fmt.Sprintf("%s%d/%d", IconChecklist(), completed, total))
}

// checklistCounts returns (completed, total) for a task's checklist.
func checklistCounts(task *models.Task) (int, int) {
	completed := 0
	for _, item := range task.Checklist {
		if item.Done {
			completed++
		}
	}
	return completed, len(task.Checklist)
}
