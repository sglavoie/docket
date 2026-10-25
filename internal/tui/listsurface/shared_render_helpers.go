package listsurface

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/docket/internal/icons"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/theme"
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
// It closes on one capture line spelling three sigils, so a first launch shows
// what the composer reads rather than only that it exists.
const EmptyStateNoTasks = "No tasks yet. Press a to capture one, such as: Call the bank !1 @fri #errands"

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
// Semantic tags are deprioritized, never excluded: the descriptive tags a reader
// cannot guess lead, and the six-name vocabulary every task shares follows them.
// Peek is now the one surface that renders a chip at all — the task row carries
// no tags — so the order decides reading sequence rather than which chips survive
// a width. Membership in that deprioritized group is the
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

// PinnedMarker is the one pin vocabulary every surface that lists a pinnable
// thing shares: the bold `*` the `*` key writes and the `*pin` sigil names. It
// is a glyph rather than a color because SelectedLine strips styling from the
// cursor row, so a colored name says nothing about the row the user is on.
func PinnedMarker() string {
	return theme.PinnedMarkerStyle.Render("*") + " "
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
