package projecthub

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The behavioral project-tag vocabulary is rendered from three places -- the
// tags picker, the picker's own explainer strip, and the project editor's tags
// field -- and every one of them must say the same sentence in the same badge
// style. Keeping both helpers together is what makes that one authority rather
// than three that agree today.

// BehavioralProjectTagExplainer restates every behavioral tag beneath the list.
// The picker cannot show the effect by example the way a state change can, so
// the sentence has to be on screen the whole time the choice is open. Each
// complete entry wraps inline rather than sending a detached tail elsewhere.
func BehavioralProjectTagExplainer(provider operation.ValueProviderID, width int) string {
	if provider != operation.ValueProviderProjectTags {
		return ""
	}
	names := make([]string, 0, len(models.BehavioralProjectTags))
	for name := range models.BehavioralProjectTags {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		info := models.BehavioralProjectTags[name]
		entry := BehavioralProjectTagBadgeStyle(info.BadgeRole).Render(info.Name) +
			" " + theme.MutedStyle.Render(info.Summary)
		lines = append(lines, strings.Join(textfit.WrapStyled(entry, width), "\n"))
	}
	return strings.Join(lines, "\n")
}

// BehavioralProjectTagBadgeStyle turns the metadata badge role into a palette
// style. An unrecognized role renders muted so a tag is never hidden by a
// missing style.
func BehavioralProjectTagBadgeStyle(role string) lipgloss.Style {
	switch role {
	case "warning":
		return lipgloss.NewStyle().Foreground(theme.ColorWarning).Bold(true)
	default:
		return theme.MutedStyle
	}
}
