package chrome

import (
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/theme"
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
	case ModeAgenda, ModeRecent:
		return models.StateGroupDoing, true
	default:
		return "", false
	}
}

// GetOutlinerModeStyle resolves one of the final eight mode identities to its
// static theme role. Workflow group headers use GetWorkflowStateGroupStyle.
func GetOutlinerModeStyle(mode OutlinerModeID) lipgloss.Style {
	if group, ok := OutlinerModeStateGroup(mode); ok {
		return theme.GetWorkflowStateGroupStyle(group)
	}
	return theme.ViewModeAllStyle
}

// ModeArrival is how the active mode was just reached, for the one phase the
// mode switch is drawn arriving: through Tab, through Shift+Tab, or some other
// way (a chord, a lens, Esc).
type ModeArrival uint8

const (
	ModeArrivalNone ModeArrival = iota
	ModeArrivalForward
	ModeArrivalBackward
	ModeArrivalJump
)

// OperationalTitleModeSegmentArriving renders the mode identity segment of the
// operational title row: the badge is pulse's, styled by the mode, and it
// lands reversed while the mode is arriving, at the same width.
func OperationalTitleModeSegmentArriving(mode OutlinerMode, arrival ModeArrival) string {
	style := GetOutlinerModeStyle(mode.ID)
	if arrival != ModeArrivalNone {
		style = style.Reverse(true)
	}
	return pulse.OperationalTitleBadge(style, mode.Label)
}

// OperationalTitleBrandMark is Docket's mark as the title row leads with it:
// the glyph in the active mode's ink, bold, then the one space that parts it
// from the scope. It changes color with the mode, so Tab recolors the mark the
// switch lands on as well as the badge. It is decoration and is spent only from
// columns the title would otherwise leave blank.
func OperationalTitleBrandMark(mode OutlinerModeID) string {
	ink := theme.StructuralSectionStyle(GetOutlinerModeStyle(mode)).GetForeground()
	return lipgloss.NewStyle().Foreground(ink).Bold(true).Render(theme.IconBrand) + " "
}

// OperationalTitleModeStripArriving renders the active mode's badge followed
// by its two Tab neighbors, muted — `Inbox  ‹ Today 2  Next ›` — so a wide
// title says where Shift+Tab and Tab go and, where counts names one, how much
// waits there. A neighbor counts only when counts holds a nonzero entry for it,
// so a mode with nothing waiting reads as bare as one that is never counted. While the mode is arriving the badge lands
// reversed and the arrow the switch traveled
// through — › for Tab, ‹ for Shift+Tab — is lit, so the strip says which way
// the modes turned. Every cell keeps its width.
func OperationalTitleModeStripArriving(active OutlinerModeID, arrival ModeArrival, counts map[OutlinerModeID]int) string {
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
			return OperationalTitleModeSegmentArriving(mode, arrival)
		}
		return ""
	}
	previous := modes[(index-1+len(modes))%len(modes)]
	next := modes[(index+1)%len(modes)]
	back, forward := theme.MutedStyle, theme.MutedStyle
	lit := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true)
	switch arrival {
	case ModeArrivalForward:
		forward = lit
	case ModeArrivalBackward:
		back = lit
	case ModeArrivalNone, ModeArrivalJump:
	}
	return OperationalTitleModeSegmentArriving(modes[index], arrival) + "  " +
		back.Render("‹") + theme.MutedStyle.Render(" "+countedModeLabel(previous, counts)+"  "+countedModeLabel(next, counts)+" ") + forward.Render("›")
}

// countedModeLabel is a Tab neighbor's label with its count, when it has one.
func countedModeLabel(mode OutlinerMode, counts map[OutlinerModeID]int) string {
	if count := counts[mode.ID]; count > 0 {
		return mode.Label + " " + strconv.Itoa(count)
	}
	return mode.Label
}

// ModeArrivalBetween names how from became to: one step along the Tab cycle
// either way, or a jump.
func ModeArrivalBetween(from, to OutlinerModeID) ModeArrival {
	if from == to {
		return ModeArrivalNone
	}
	modes := OutlinerTabModes()
	fromIndex, toIndex := -1, -1
	for i, mode := range modes {
		if mode.ID == from {
			fromIndex = i
		}
		if mode.ID == to {
			toIndex = i
		}
	}
	if fromIndex < 0 || toIndex < 0 {
		return ModeArrivalJump
	}
	switch toIndex {
	case (fromIndex + 1) % len(modes):
		return ModeArrivalForward
	case (fromIndex - 1 + len(modes)) % len(modes):
		return ModeArrivalBackward
	}
	return ModeArrivalJump
}
