package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// formatDeferDate renders a deferral against now. A midnight-valued deferral is
// a whole day and keeps the bare date it has always had; any other instant
// carries the clock, because the row has to say when the task comes back. The
// value is resolved into now's zone first, so the midnight and same-day tests
// and the rendered text all read against one clock.
func formatDeferDate(deferredUntil, now time.Time) string {
	value := deferredUntil.In(now.Location())
	offYear := value.Year() != now.Year()
	switch {
	case value.Equal(util.StartOfDay(value)):
		if offYear {
			return value.Format("Jan 2 '06")
		}
		return value.Format("Jan 2")
	case util.StartOfDay(value).Equal(util.StartOfDay(now)):
		return value.Format("15:04")
	case offYear:
		return value.Format("Jan 2 '06 15:04")
	default:
		return value.Format("Jan 2 15:04")
	}
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
	Cursor      cursorPaint    // how this header wears the cursor; zero on an ordinary row
}

// renderSectionHeader renders a standardized collapsible section header:
//
//	▾ [prefix]Name (count) ─────
//
// It returns the leading fold gutter and the rest separately, so a caller
// marking the cursor row can paint the gutter without repainting the name. This
// is the one header whose first column is a glyph rather than a blank, so the
// cursor bar reverses the fold arrow instead of a space.
// AccentColor is applied to the name; indicator and count use MutedStyle.
// When Width > 0 a trailing horizontal rule fills the remaining line width.
// Callers are responsible for applying selection/zebra highlights on the
// returned strings.
func renderSectionHeader(opts SectionHeaderOpts) (gutter, content string) {
	indicator := IconExpanded
	if opts.Collapsed {
		indicator = IconCollapsed
	}
	gutter = opts.Cursor.bar(MutedStyle, indicator) + gutterSpace(opts.Cursor.gutterBg())
	namePart := opts.AccentColor.Bold(true).Render(opts.Prefix + opts.Name)
	content = namePart + " " + MutedStyle.Render(fmt.Sprintf("(%d)", opts.Count))

	if opts.Width > 0 {
		visible := lipgloss.Width(gutter) + lipgloss.Width(content)
		if opts.Width > visible+2 {
			content += " " + MutedStyle.Render(strings.Repeat("─", opts.Width-visible-1))
		}
	}
	return gutter, content
}

// orderedTaskTags is the one display order both tag surfaces render: every
// non-semantic tag first in alphabetical order, then every semantic tag, also
// alphabetical. Storage keeps tags in insertion order (AddTag appends), so two
// tasks tagged the same way could show different chips; this makes the tag set,
// not the typing history, decide what a row shows.
//
// It is a pure permutation — nothing is dropped, nothing is added — so a
// caller's overflow count stays honest by construction and each surface keeps
// its own rules about which tags are renderable at all.
//
// Semantic tags are deprioritized, never excluded. The title color already
// carries the semantic signal, so a row that can afford two chips spends them on
// what the title cannot say; but under --no-color there is no title color
// channel and no renderer can detect that (RunOptions.DisableColor only reaches
// tea.WithEnvironment), so a task tagged only bug must still show #bug.
// Membership in that deprioritized group is decided by the same
// SemanticTaskTagByName lookup TaskTitleStyleFor consults, so a tag is demoted
// exactly when the title can speak for it.
//
// The sort key is models.NormalizeTag, the same canonical spelling tagChipSlot
// hashes, so a tag's position and its color agree about what one tag is. The raw
// spelling breaks a tie, which is what makes the result independent of insertion
// order rather than merely stable under it.
func orderedTaskTags(tags []string) []string {
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

// renderTruncatedTags renders a compact inline tag string with per-tag truncation
// and overflow count. Shows at most maxTags tags, each truncated to maxTagLen chars.
// Each tag wears its own chip color from tagChipStyle; the +N overflow stays
// muted because it counts tags rather than naming one.
// Returns an empty string if there are no tags.
func renderTruncatedTags(tags []string, maxTags, maxTagLen int) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	for i, tag := range orderedTaskTags(tags) {
		if i >= maxTags {
			break
		}
		// The full tag decides the color; truncation is presentation only, so a
		// clipped chip keeps the color its Peek counterpart shows in full.
		display := truncateToVisualWidth(tag, maxTagLen)
		parts = append(parts, tagChipStyle(tag).Render("#"+display))
	}
	result := strings.Join(parts, " ")
	if len(tags) > maxTags {
		result += MutedStyle.Render(fmt.Sprintf(" +%d", len(tags)-maxTags))
	}
	return result
}

// pinnedMarker is the one pin vocabulary every surface that lists a pinnable
// thing shares: the bold `*` the `*` key writes and the `*pin` sigil names. It
// is a glyph rather than a color because SelectedLine strips styling from the
// cursor row, so a colored name says nothing about the row the user is on.
func pinnedMarker() string {
	return PinnedMarkerStyle.Render("*") + " "
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

// formatGitHubIssueRef renders the compact owner/repo#number form. It carries
// the github_issue marker because the compact form is otherwise
// indistinguishable from a plain URL attachment in the same list.
func formatGitHubIssueRef(ref models.GitHubIssueRef) string {
	return fmt.Sprintf("%s %s/%s#%d", icons.Get("github_issue"), ref.Owner, ref.Repo, ref.Number)
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
