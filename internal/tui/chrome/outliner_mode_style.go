package chrome

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// OutlinerModeStateGroup names the workflow-state group a mode borrows its
// semantic color from. All has none of its own, and neither does an unknown
// token. This is the single mode-to-group table: both the badge style below and
// the named-lens accent read it rather than restating it.
//
// It sits beside the mode catalog rather than beside the theme palette because
// the question it answers is about a mode, not about a color: the theme names
// state groups and knows nothing of OutlinerModeID.
func OutlinerModeStateGroup(mode OutlinerModeID) (models.WorkflowStateGroup, bool) {
	switch mode {
	case ModeInbox:
		return models.StateGroupInbox, true
	case ModeToday:
		return models.StateGroupToday, true
	case ModeNext, ModeDue:
		return models.StateGroupNext, true
	case ModeBacklog:
		return models.StateGroupBacklog, true
	case ModeRecent:
		return models.StateGroupDoing, true
	default:
		return "", false
	}
}

// GetOutlinerModeStyle resolves one of the final seven mode identities to its
// static theme role. Workflow group headers use GetWorkflowStateGroupStyle.
func GetOutlinerModeStyle(mode OutlinerModeID) lipgloss.Style {
	if group, ok := OutlinerModeStateGroup(mode); ok {
		return theme.GetWorkflowStateGroupStyle(group)
	}
	return theme.ViewModeAllStyle
}

// OperationalTitleModeSegment renders the mode identity segment of the
// operational title row.
//
// It sits here rather than beside the rest of the title row because it is the
// one segment built out of the mode catalog: OutlinerMode and the style table
// above are chrome vocabulary, and internal/tui/pulse is below chrome. The
// badge itself is pulse's, so this is the adapter that hands chrome's mode to
// it and nothing more.
func OperationalTitleModeSegment(mode OutlinerMode) string {
	return pulse.OperationalTitleBadge(GetOutlinerModeStyle(mode.ID), mode.Label)
}

// OperationalTitleModeStrip renders the active mode's badge followed by its two
// Tab neighbors, muted — `Inbox  ‹ Today  Next ›` — so a wide title says where
// Shift+Tab and Tab go without moving the badge off the scope it qualifies. The
// badge is the segment OperationalTitleModeSegment draws, so a title that falls
// back to the lone badge names the mode identically.
func OperationalTitleModeStrip(active OutlinerModeID) string {
	modes := OutlinerTabModes()
	index := -1
	for i, mode := range modes {
		if mode.ID == active {
			index = i
			break
		}
	}
	if index < 0 || len(modes) < 2 {
		if mode, ok := OutlinerModeByID(active); ok {
			return OperationalTitleModeSegment(mode)
		}
		return ""
	}
	previous := modes[(index-1+len(modes))%len(modes)]
	next := modes[(index+1)%len(modes)]
	return OperationalTitleModeSegment(modes[index]) + "  " +
		theme.MutedStyle.Render("‹ "+previous.Label+"  "+next.Label+" ›")
}
