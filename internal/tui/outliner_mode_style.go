package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// outlinerModeStateGroup names the workflow-state group a mode borrows its
// semantic color from. All has none of its own, and neither does an unknown
// token. This is the single mode-to-group table: both the badge style below and
// the named-lens accent read it rather than restating it.
//
// It sits beside the mode catalog rather than beside the theme palette because
// the question it answers is about a mode, not about a color: the theme names
// state groups and knows nothing of OutlinerModeID.
func outlinerModeStateGroup(mode OutlinerModeID) (models.WorkflowStateGroup, bool) {
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
	if group, ok := outlinerModeStateGroup(mode); ok {
		return theme.GetWorkflowStateGroupStyle(group)
	}
	return theme.ViewModeAllStyle
}
