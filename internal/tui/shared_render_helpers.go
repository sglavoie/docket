package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// IndicatorGroupSep is the dim separator rendered between indicator groups.
var IndicatorGroupSep = MutedStyle.Render(" │ ")

func formatSnoozeDate(snoozedUntil time.Time) string {
	if snoozedUntil.Year() != time.Now().Year() {
		return snoozedUntil.Format("Jan 2 '06")
	}
	return snoozedUntil.Format("Jan 2")
}

// JoinIndicatorGroups joins non-empty group strings with the shared dim separator.
// Empty groups are skipped so no orphan separators appear.
func JoinIndicatorGroups(groups ...string) string {
	var nonEmpty []string
	for _, group := range groups {
		if group != "" {
			nonEmpty = append(nonEmpty, group)
		}
	}
	return strings.Join(nonEmpty, IndicatorGroupSep)
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

// EmptyStateClosed is the standard message shown when the closed/done tab has no tasks.
const EmptyStateClosed = "No closed tasks."

// EmptyStateNoMatch is the standard message shown when a tab filter produces no results.
const EmptyStateNoMatch = "No tasks match this tab."

// EmptyStateSearchPrompt is shown in a search view when no query has been entered.
const EmptyStateSearchPrompt = "Type to search across all tasks…"

// EmptyStateCrossProject is shown when a cross-project search finds no tasks.
const EmptyStateCrossProject = "No tasks found across projects."

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

func renderDescriptionIndicator(task *models.Task) string {
	if len(task.Description) == 0 {
		return ""
	}
	return MutedStyle.Render(IconDescription())
}

// renderCommentIndicator renders a compact comment count indicator.
func renderCommentIndicator(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconComment(), len(task.Comments)))
}

// renderDecisionIndicator renders a compact decision count indicator.
func renderDecisionIndicator(task *models.Task) string {
	if len(task.Decisions) == 0 {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconDecision(), len(task.Decisions)))
}

// renderAttachmentIndicator renders a compact attachment count indicator.
func renderAttachmentIndicator(task *models.Task) string {
	if !task.HasAttachments() {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconAttachment(), task.AttachmentCount()))
}

func renderGitHubIssueIndicator(task *models.Task) string {
	label := formatGitHubIssueRefs(task.GitHubIssueRefs())
	if label == "" {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %s", IconGitHubIssue(), label))
}

func formatGitHubIssueRefs(refs []models.GitHubIssueRef) string {
	if len(refs) == 0 {
		return ""
	}
	label := formatGitHubIssueRef(refs[0])
	if len(refs) > 1 {
		label += fmt.Sprintf(" +%d", len(refs)-1)
	}
	return label
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

// renderChecklistIndicatorCompact renders the smallest checklist progress form
// (no icon, plain "X/Y") suitable for very narrow containers.
func renderChecklistIndicatorCompact(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	completed, total := checklistCounts(task)
	return MutedStyle.Render(fmt.Sprintf("%d/%d", completed, total))
}

// renderChecklistBar renders a checklist progress bar of the form "[▓▓▓░░] 3/5".
// When the available width is too small to render a meaningful bar, falls back
// to renderChecklistIndicator. Returns "" when the checklist is empty.
func renderChecklistBar(task *models.Task, width int) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	const minBarWidth = 12
	if width < minBarWidth {
		return renderChecklistIndicator(task)
	}
	completed, total := checklistCounts(task)
	percent := float64(completed) / float64(total) * 100
	bar := "[" + RenderProgressBar(percent, 5) + "]"
	fraction := fmt.Sprintf("%d/%d", completed, total)
	if completed == total {
		return bar + " " + SuccessStyle.Render(fraction)
	}
	return bar + " " + MutedStyle.Render(fraction)
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

// renderProgressFraction renders an inline progress bar + fraction for parent tasks.
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
	total := len(task.Children)
	percent := float64(completed) / float64(total) * 100
	bar := "[" + RenderProgressBar(percent, 5) + "]"
	fraction := fmt.Sprintf("%d/%d", completed, total)
	if completed == total {
		return bar + " " + SuccessStyle.Render(fraction)
	}
	return bar + " " + MutedStyle.Render(fraction)
}

func renderDependencyIndicatorCounts(needs, blocks int) string {
	var parts []string
	if needs > 0 {
		parts = append(parts, DependencyStyle.Render(fmt.Sprintf("→%d", needs)))
	}
	if blocks > 0 {
		parts = append(parts, DependencyStyle.Render(fmt.Sprintf("←%d", blocks)))
	}
	return strings.Join(parts, " ")
}
