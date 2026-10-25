package theme

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
)

// A project header names two pressures and they are two different facts: an
// overdue count is a deadline the work missed, a today count is a plan the user
// made. Both carry the emphasis the due chips carry, because a folded group has
// to say what is urgent inside it at the weight the rows themselves would, and
// both stay foreground-only, because a header is text and a filled chip there
// would read as a second badge beside the group size.
//
// Hue is what keeps the two facts apart, and neither hue is minted here: overdue
// borrows the due family's own accent and today borrows the Today group's, so a
// count can never drift from the chips and the lane it summarizes, and nothing
// here lets a deadline and a plan collapse into one color.
func newProjectPressureOverdueStyle() lipgloss.Style {
	return newLensAccentOverdueStyle().Bold(true)
}

func newProjectPressureTodayStyle() lipgloss.Style {
	style, ok := WorkflowStateGroupAccentStyle(models.StateGroupToday)
	if !ok {
		return MutedStyle.Bold(true)
	}
	return style.Bold(true)
}

// Project header pressure counts, repainted by rebuildProjectPressureStyles.
var (
	ProjectPressureOverdueStyle = newProjectPressureOverdueStyle()
	ProjectPressureTodayStyle   = newProjectPressureTodayStyle()
)

// rebuildProjectPressureStyles must run after rebuildLensAccentStyles: the today
// accent is read off the rebuilt lens accent rather than off a second copy of
// its hex, so running it earlier would repaint this pair from the outgoing
// theme.
func rebuildProjectPressureStyles() {
	ProjectPressureOverdueStyle = newProjectPressureOverdueStyle()
	ProjectPressureTodayStyle = newProjectPressureTodayStyle()
}
