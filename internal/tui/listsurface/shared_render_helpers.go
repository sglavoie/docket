package listsurface

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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

// OrderedTaskTags is the one display order both tag surfaces render: every
// non-semantic tag first in alphabetical order, then every semantic tag, also
// alphabetical. Storage keeps tags in insertion order (AddTag appends), so two
// tasks tagged the same way could show different chips; this makes the tag set,
// not the typing history, decide what a row shows.
//
// It is a pure permutation — nothing is dropped, nothing is added — so a
// caller's overflow count stays honest by construction and each surface keeps
// its own rules about which tags are renderable at all.
//
// Semantic tags are deprioritized, never excluded: a row that can afford two
// chips spends them on the tags a reader cannot guess from the six-name
// vocabulary every task shares, and Peek still names the complete set. The rule
// predates neutral titles, where a semantic tag was deprioritized because the
// title color had already said it; the ordering outlived that reason, and which
// chips a narrowed row keeps is the row-budget work's to revisit rather than the
// sort's to decide unilaterally. Membership in that deprioritized group is the
// models.SemanticTaskTagByName lookup, the same one the pickers build their
// SEMANTIC section from.
//
// The sort key is models.NormalizeTag, the same canonical spelling TagChipSlot
// hashes, so a tag's position and its color agree about what one tag is. The raw
// spelling breaks a tie, which is what makes the result independent of insertion
// order rather than merely stable under it.
func OrderedTaskTags(tags []string) []string {
	semantic := func(tag string) bool {
		_, ok := models.SemanticTaskTagByName(tag)
		return ok
	}
	ordered := slices.Clone(tags)
	slices.SortFunc(ordered, func(a, b string) int {
		if aSemantic, bSemantic := semantic(a), semantic(b); aSemantic != bSemantic {
			if aSemantic {
				return 1
			}
			return -1
		}
		if cmp := strings.Compare(models.NormalizeTag(a), models.NormalizeTag(b)); cmp != 0 {
			return cmp
		}
		return strings.Compare(a, b)
	})
	return ordered
}

// RenderTruncatedTags renders a compact inline tag string with per-tag truncation
// and overflow count. Shows at most maxTags tags, each truncated to maxTagLen chars.
// Each tag wears its own chip color from TagChipStyle; the +N overflow stays
// muted because it counts tags rather than naming one.
// Returns an empty string if there are no tags.
func RenderTruncatedTags(tags []string, maxTags, maxTagLen int) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	for i, tag := range OrderedTaskTags(tags) {
		if i >= maxTags {
			break
		}
		// The full tag decides the color; truncation is presentation only, so a
		// clipped chip keeps the color its Peek counterpart shows in full.
		display := textfit.TruncateToVisualWidth(tag, maxTagLen)
		parts = append(parts, theme.TagChipStyle(tag).Render("#"+display))
	}
	result := strings.Join(parts, " ")
	if len(tags) > maxTags {
		result += theme.MutedStyle.Render(fmt.Sprintf(" +%d", len(tags)-maxTags))
	}
	return result
}

// PinnedMarker is the one pin vocabulary every surface that lists a pinnable
// thing shares: the bold `*` the `*` key writes and the `*pin` sigil names. It
// is a glyph rather than a color because SelectedLine strips styling from the
// cursor row, so a colored name says nothing about the row the user is on.
func PinnedMarker() string {
	return theme.PinnedMarkerStyle.Render("*") + " "
}

// RenderCommentIndicator renders a compact comment count indicator.
func RenderCommentIndicator(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	return theme.MutedStyle.Render(fmt.Sprintf("%s %d", theme.IconComment(), len(task.Comments)))
}

// RenderAttachmentIndicator renders a compact attachment count indicator.
func RenderAttachmentIndicator(task *models.Task) string {
	if !task.HasAttachments() {
		return ""
	}
	return theme.MutedStyle.Render(fmt.Sprintf("%s %d", theme.IconAttachment(), task.AttachmentCount()))
}

// formatGitHubIssueRef renders the compact owner/repo#number form. It carries
// the github_issue marker because the compact form is otherwise
// indistinguishable from a plain URL attachment in the same list.
func formatGitHubIssueRef(ref models.GitHubIssueRef) string {
	return fmt.Sprintf("%s %s/%s#%d", icons.Get("github_issue"), ref.Owner, ref.Repo, ref.Number)
}

func GitHubIssueAttachmentRef(att models.Attachment) (models.GitHubIssueRef, bool) {
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

func AttachmentDisplayName(att models.Attachment) string {
	if ref, ok := GitHubIssueAttachmentRef(att); ok {
		return formatGitHubIssueRef(ref)
	}
	if att.IsCodeRef {
		return fmt.Sprintf("%s:%d", att.OriginalPath, att.LineNumber)
	}
	return att.Filename
}

// RenderChecklistIndicator renders a compact checklist progress indicator.
func RenderChecklistIndicator(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	completed, total := checklistCounts(task)
	return theme.MutedStyle.Render(fmt.Sprintf("%s%d/%d", theme.IconChecklist(), completed, total))
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
