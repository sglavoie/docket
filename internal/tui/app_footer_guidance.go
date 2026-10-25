package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// activeGuidance resolves the one layer of the composed frame whose
// instructions are live, in the order handleGlobalKeyPress visits its layers: a
// standing confirmation, then a shell-mounted modal, then an unfinished chord,
// then the presentation itself. Whatever it names is the only layer allowed to
// speak, because it is the only one a key press reaches.
//
// The chord spec is returned with the owner so the caller renders exactly what
// the resolution read, rather than asking a second time.
func (m *AppModel) activeGuidance() (route.ActiveGuidanceOwner, chrome.GuidanceSpec) {
	if m.shellConfirmationArmed() {
		return route.GuidanceOwnerConfirmation, chrome.GuidanceSpec{}
	}
	if m.shellModalMounted() {
		return route.GuidanceOwnerModal, chrome.GuidanceSpec{}
	}
	if spec, ok := m.pendingChordGuidance(); ok {
		return route.GuidanceOwnerPendingChord, spec
	}
	if !m.activePresentationPublishesToShell() {
		return route.GuidanceOwnerPresentation, chrome.GuidanceSpec{}
	}
	return route.GuidanceOwnerShell, chrome.GuidanceSpec{}
}

// shellConfirmationArmed reports whether one of the three shell-owned
// confirmations is standing over the view that raised it.
//
// A confirmation the active view owns and answers for through its own
// ContextualGuidance is deliberately not here: there the surface is still the
// layer speaking, and a draft-discard prompt whose exits reach the footer must
// keep reaching it.
func (m *AppModel) shellConfirmationArmed() bool {
	return m.project.ConfirmationArmed() ||
		m.taskMutation.confirmationArmed() ||
		m.operations.Confirm.IsActive()
}

// shellModalMounted covers exactly what handleActiveKeyOverlay routes to:
// whatever that dispatcher hands the next key press is the interaction that owns
// guidance. The action-sheet flow answers through its own OverlayMounted rather
// than a second copy of its four levels, so a level added there is covered here
// without a second edit.
func (m *AppModel) shellModalMounted() bool {
	return m.checkInSetup != nil ||
		m.operations.OverlayMounted() ||
		m.lenses.Mounted() ||
		m.conflictBrowser != nil
}

// activePresentationPublishesToShell asks whether the footer is still this
// presentation's voice. A view answering SelfGuidedPresentation decides per
// frame, since a PresentationID names a view and one view may hold a sub-state
// that owns every key and draws its instructions in its own frame, as an inline
// overlay over the expanded form does; otherwise the static chrome policy
// answers, and it answers by whether a host is still standing under this
// presentation holding that footer.
//
// A fullscreen surface keeps it. One that publishes ContextualGuidance has
// handed the footer its keys and draws none of its own, so the footer is the
// frame's one divider and one set of hints; one that still draws its acts in its
// content leaves the footer that same surface's breadcrumb and its `Esc back · ?
// help`.
//
// An unclassified presentation publishes, deliberately: a footer that may be the
// only instructions on screen is never blanked on a guess.
func (m *AppModel) activePresentationPublishesToShell() bool {
	if view := m.activeView(); view != nil {
		if provider, ok := view.(route.SelfGuidedPresentation); ok && provider.PublishesOwnGuidance() {
			return false
		}
	}
	policy, ok := route.ChromePolicyForPresentation(m.currentView())
	if !ok {
		return true
	}
	return !policy.HostGuidanceStandsBeneath()
}

// suppressGuidanceRows empties an occupant without resizing it. The frame was
// laid out against this height, so returning fewer rows would move the content
// above it and closing the overlay would move it back.
//
// The rows it returns are blank but present, and that distinction is the whole
// of it: the empty string is how this footer spells a block that is not there at
// all, so a one-row occupant blanked to "" would be composed and measured as
// nothing and the footer would shrink under a message it is still carrying.
// Each held row is filled to the footer's own width for the same reason
// renderFooterMessageRow fills its own: a row the footer is spending is a row it
// covers, so the wash and the background find no hole where the occupant stood.
//
// An absent occupant stays absent: there is no row to hold, so blanking one
// would charge the frame for guidance nobody wrote.
func suppressGuidanceRows(content string, width int) string {
	rows := footerBlockRows(content)
	if rows == 0 {
		return ""
	}
	blank := strings.Repeat(" ", max(width, 1))
	return strings.TrimSuffix(strings.Repeat(blank+"\n", rows), "\n")
}

// surfaceFooterContent builds what the shell footer says on behalf of the
// active surface: its contextual guidance, else the breadcrumb row with its
// help hint. It is one occupant, so suppression takes it whole — while a modal
// owns every key, the breadcrumb's Esc preview names a destination that press
// would not reach.
func (m *AppModel) surfaceFooterContent() string {
	// Show the current view's contextual guidance if it provides any (e.g. the
	// capture views). This sits above the generic breadcrumb but below the
	// transient loading/error/status overlays handled by the caller, so a
	// temporary status message still replaces capture controls and the controls
	// return once it clears.
	if view := m.activeView(); view != nil {
		if provider, ok := view.(route.ContextualGuidanceProvider); ok {
			if spec, ok := provider.ContextualGuidance(); ok {
				return chrome.RenderGuidanceText(spec, m.viewNav.width)
			}
		}
	}

	status := m.buildBreadcrumb()

	// Check if view supports help shortcut hint; only show if view implements HelpProvider.
	currentView := m.currentView()
	showHelp := false
	if view := m.activeView(); view != nil {
		if _, ok := view.(help.HelpProvider); ok {
			showHelp = true
		}
	}

	help := ""
	if showHelp {
		if hint := pipelineNextHint(currentView); hint != "" {
			help = theme.HelpStyle.Render(hint)
		} else {
			help = strings.Join(
				chrome.RenderGuidanceLines(chrome.EscapeGuidanceSpec(), m.viewNav.width),
				theme.HelpStyle.Render(chrome.FooterHelpSeparator),
			)
		}
	}

	accent := m.currentAccentColor()
	accentBarStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(m.footerFlashBackgroundColor()).
		Padding(0, 1)

	left := accentBarStyle.Render(status)

	// Add filter pill if current view has active filters.
	if view := m.activeView(); view != nil {
		if fp, ok := view.(route.ActiveFilterProvider); ok {
			if count := fp.ActiveFilterCount(); count > 0 {
				filterLabel := "filters"
				if count == 1 {
					filterLabel = "filter"
				}
				left += " " + theme.FilterBadgeStyle.Render(fmt.Sprintf("%d %s", count, filterLabel))
			}
		}
	}

	// Prepend mode badge when the current view is in a named mode.
	if view := m.activeView(); view != nil {
		if mi, ok := view.(route.ModeIndicator); ok {
			if label, c := mi.ActiveMode(); label != "" {
				badge := lipgloss.NewStyle().
					Bold(true).
					Background(c).
					Foreground(theme.ColorBackground).
					Padding(0, 1).
					Render(label)
				left = badge + " " + left
			}
		}
	}

	right := help
	left, right = fitStatusBarSegments(left, right, m.viewNav.width)
	gap := max(m.viewNav.width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	spacer := lipgloss.NewStyle().
		Width(gap).
		Background(m.footerFlashBackgroundColor()).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
