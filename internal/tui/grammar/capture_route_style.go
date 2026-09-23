package grammar

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// CapturePlanAccentStyle is the inline badge a planning destination's plan
// value wears: the Today or Next section badge. A lifecycle destination names no
// plan, so it falls back to its status badge.
func CapturePlanAccentStyle(destination app.CaptureDestination) lipgloss.Style {
	switch destination {
	case app.CaptureDestinationToday:
		return theme.StateGroupInlineBadgeStyle(models.StateGroupToday)
	case app.CaptureDestinationNext:
		return theme.StateGroupInlineBadgeStyle(models.StateGroupNext)
	default:
		return CaptureStatusAccentStyle(destination.WorkflowState())
	}
}

// CaptureStatusAccentStyle is the inline badge a lifecycle state wears in
// capture: its state group's section badge.
func CaptureStatusAccentStyle(state models.WorkflowState) lipgloss.Style {
	return theme.StateGroupInlineBadgeStyle(models.WorkflowStateGroupForState(state))
}
