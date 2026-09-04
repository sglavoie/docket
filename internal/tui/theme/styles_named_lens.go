package theme

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// A named lens is a saved projection, and a saved projection is a definition:
// what it selects is what it should look like. These accents are the foreground
// half of the badge families the Outliner already paints — the section fills in
// styles_view_modes.go and the due chips in styles_due_dates.go — lifted out of
// their chips so a lens name can wear one without becoming a badge.
//
// Every hue choice lives here alone. A picker that colors a lens asks
// lens.NamedLensPrimaryStyle and never names a color, exactly as a task row asks
// TagChipStyle and never names a chip.
//
// Each accent keeps its family's hue and moves only its lightness: on a dark
// theme a section fill is far too dark to read as text, and on a light theme the
// curated light fill is already a readable mid-tone. Urgency inside the teal due
// family stays carried by saturation, the same ladder the chips use, so the
// three due accents read as one family with an order rather than three colors.
//
// The pairs live in these constructors alone, because rebuildLensAccentStyles
// repaints the same vars on every ApplyTheme and a second copy of a hex would
// let the startup look and the repainted look drift apart.
func newLensAccentPlanStyle() lipgloss.Style     { return makeLensAccentStyle("#8fd694", "#15803d") }
func newLensAccentWorkStyle() lipgloss.Style     { return makeLensAccentStyle("#ff9e64", "#9a3412") }
func newLensAccentInboxStyle() lipgloss.Style    { return makeLensAccentStyle("#5fb0e8", "#1d4ed8") }
func newLensAccentNextStyle() lipgloss.Style     { return makeLensAccentStyle("#9fa8f5", "#3730a3") }
func newLensAccentWaitingStyle() lipgloss.Style  { return makeLensAccentStyle("#e8b374", "#8a4a0d") }
func newLensAccentPausedStyle() lipgloss.Style   { return makeLensAccentStyle("#b18cf7", "#7c3aed") }
func newLensAccentSomedayStyle() lipgloss.Style  { return makeLensAccentStyle("#9f7ae0", "#6d28d9") }
func newLensAccentCompleteStyle() lipgloss.Style { return makeLensAccentStyle("#9aa0b8", "#4b5563") }
func newLensAccentCanceledStyle() lipgloss.Style { return makeLensAccentStyle("#e06c75", "#991b1b") }
func newLensAccentOverdueStyle() lipgloss.Style  { return makeLensAccentStyle("#5eead4", "#0f766e") }
func newLensAccentDueTodayStyle() lipgloss.Style { return makeLensAccentStyle("#2dd4bf", "#146e64") }
func newLensAccentDueStyle() lipgloss.Style      { return makeLensAccentStyle("#8fbdb8", "#4a6764") }
func newLensAccentDeferredStyle() lipgloss.Style { return makeLensAccentStyle("#e08aa8", "#9d174d") }
func newLensAccentReviewStyle() lipgloss.Style   { return makeLensAccentStyle("#c4b5fd", "#5b21b6") }

// Named lens accents, repainted by rebuildLensAccentStyles on every theme change.
var (
	lensAccentPlanStyle     = newLensAccentPlanStyle()
	lensAccentWorkStyle     = newLensAccentWorkStyle()
	lensAccentInboxStyle    = newLensAccentInboxStyle()
	lensAccentNextStyle     = newLensAccentNextStyle()
	lensAccentWaitingStyle  = newLensAccentWaitingStyle()
	lensAccentPausedStyle   = newLensAccentPausedStyle()
	lensAccentSomedayStyle  = newLensAccentSomedayStyle()
	lensAccentCompleteStyle = newLensAccentCompleteStyle()
	lensAccentCanceledStyle = newLensAccentCanceledStyle()
	LensAccentOverdueStyle  = newLensAccentOverdueStyle()
	LensAccentDueTodayStyle = newLensAccentDueTodayStyle()
	LensAccentDueStyle      = newLensAccentDueStyle()
	LensAccentDeferredStyle = newLensAccentDeferredStyle()
	LensAccentReviewStyle   = newLensAccentReviewStyle()
)

func rebuildLensAccentStyles() {
	lensAccentPlanStyle = newLensAccentPlanStyle()
	lensAccentWorkStyle = newLensAccentWorkStyle()
	lensAccentInboxStyle = newLensAccentInboxStyle()
	lensAccentNextStyle = newLensAccentNextStyle()
	lensAccentWaitingStyle = newLensAccentWaitingStyle()
	lensAccentPausedStyle = newLensAccentPausedStyle()
	lensAccentSomedayStyle = newLensAccentSomedayStyle()
	lensAccentCompleteStyle = newLensAccentCompleteStyle()
	lensAccentCanceledStyle = newLensAccentCanceledStyle()
	LensAccentOverdueStyle = newLensAccentOverdueStyle()
	LensAccentDueTodayStyle = newLensAccentDueTodayStyle()
	LensAccentDueStyle = newLensAccentDueStyle()
	LensAccentDeferredStyle = newLensAccentDeferredStyle()
	LensAccentReviewStyle = newLensAccentReviewStyle()
}

// WorkflowStateGroupAccentStyle is the foreground twin of
// GetWorkflowStateGroupStyle: it answers the same canonical theme role with an
// accent instead of a fill, so a group never gains a second color vocabulary.
// A group with no theme role — an unknown token — resolves to no style rather
// than to an invented one.
func WorkflowStateGroupAccentStyle(group models.WorkflowStateGroup) (lipgloss.Style, bool) {
	switch group.ThemeRole() {
	case models.StateThemePlan, models.StateThemeBacklog:
		return lensAccentPlanStyle, true
	case models.StateThemeWork:
		return lensAccentWorkStyle, true
	case models.StateThemeInbox:
		return lensAccentInboxStyle, true
	case models.StateThemeNext:
		return lensAccentNextStyle, true
	case models.StateThemeWaiting:
		return lensAccentWaitingStyle, true
	case models.StateThemePaused:
		return lensAccentPausedStyle, true
	case models.StateThemeSomeday:
		return lensAccentSomedayStyle, true
	case models.StateThemeComplete:
		return lensAccentCompleteStyle, true
	case models.StateThemeCanceled:
		return lensAccentCanceledStyle, true
	default:
		return lipgloss.Style{}, false
	}
}
