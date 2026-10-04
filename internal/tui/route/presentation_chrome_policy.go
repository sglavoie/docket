package route

import "github.com/sglavoie/tt/internal/tui/presentation"

// presentationChromeKind identifies which layer owns persistent presentation
// chrome. Full-screen presentations own the workspace pulse and their guidance;
// transient selectors and pickers own only their local interaction guidance,
// leaving persistent workspace chrome to the host beneath them.
type presentationChromeKind uint8

const (
	presentationChromeFullscreen presentationChromeKind = iota
	presentationChromeTransient
)

// presentationGuidanceOwner identifies the one layer responsible for guidance
// whenever a production presentation is reachable.
type presentationGuidanceOwner uint8

const (
	_ presentationGuidanceOwner = iota
	presentationGuidanceShell
	presentationGuidanceSurface
	presentationGuidanceOverlay
)

type PresentationChromePolicy struct {
	kind     presentationChromeKind
	guidance presentationGuidanceOwner
}

func (p PresentationChromePolicy) WorkspacePulseEligible() bool {
	return p.kind == presentationChromeFullscreen
}

// HostGuidanceStandsBeneath reports whether the footer under this presentation
// belongs to a different layer. A transient presentation is composited over a
// host that is still drawn and still filling that footer, so the ranks there
// describe a grammar no press reaches while the overlay stands. A fullscreen
// presentation replaced that host outright, so the footer beneath it is its own
// breadcrumb and its own help hint — the second half of one instruction set
// rather than a competing one — whether it publishes guidance through the shell
// or draws its acts inside its content.
func (p PresentationChromePolicy) HostGuidanceStandsBeneath() bool {
	return p.kind == presentationChromeTransient
}

// ActiveGuidanceOwner names the one layer of a composed frame whose instructions
// are live. It is resolved in the order the input dispatcher visits its layers,
// because the layer that answers the next key press is the only layer entitled
// to describe it.
type ActiveGuidanceOwner uint8

const (
	// GuidanceOwnerShell is the ordinary case: the active surface hands the
	// shell footer its guidance and the footer publishes it.
	GuidanceOwnerShell ActiveGuidanceOwner = iota
	// GuidanceOwnerPendingChord is an unfinished chord's which-key row, which
	// is live input feedback and outranks the surface it was typed over.
	GuidanceOwnerPendingChord
	// GuidanceOwnerConfirmation is a shell-owned confirmation standing over
	// whatever raised it. A confirmation a surface owns and answers for through
	// its own guidance is not this: there the surface is still speaking.
	GuidanceOwnerConfirmation
	// GuidanceOwnerModal is a shell-mounted picker, action sheet, check-in
	// editor, text prompt, lens picker, or conflict browser.
	GuidanceOwnerModal
	// GuidanceOwnerPresentation is a view that has already drawn its own
	// instructions inside its own content.
	GuidanceOwnerPresentation
)

// SuppressesShellGuidance reports whether the shell footer must stop publishing
// the active surface's instructions. It never means the footer loses its rows:
// the frame is measured against that height, so suppression spends the rows on
// blanks and hands them back whole when the owner goes away.
func (o ActiveGuidanceOwner) SuppressesShellGuidance() bool {
	switch o {
	case GuidanceOwnerConfirmation, GuidanceOwnerModal, GuidanceOwnerPresentation:
		return true
	case GuidanceOwnerShell, GuidanceOwnerPendingChord:
		return false
	default:
		return false
	}
}

func (p PresentationChromePolicy) valid() bool {
	switch p.kind {
	case presentationChromeFullscreen:
		return p.guidance == presentationGuidanceShell || p.guidance == presentationGuidanceSurface
	case presentationChromeTransient:
		return p.guidance == presentationGuidanceOverlay
	default:
		return false
	}
}

// ChromePolicyForPresentation is exhaustive over allPresentations. New
// PresentationIDs must be classified here and in the policy test before they
// can silently inherit workspace chrome.
func ChromePolicyForPresentation(view presentation.PresentationID) (PresentationChromePolicy, bool) {
	var policy PresentationChromePolicy
	switch view {
	case presentation.ViewProjectCreator,
		presentation.ViewProjectEditor,
		presentation.ViewHelp,
		presentation.ViewLastMessage,
		presentation.ViewTaskComments,
		presentation.ViewTextEditor,
		presentation.ViewTaskAttachments,
		presentation.ViewNotes,
		presentation.ViewGlobalNotes,
		presentation.ViewProjectHub:
		policy = PresentationChromePolicy{
			kind:     presentationChromeFullscreen,
			guidance: presentationGuidanceSurface,
		}
	case presentation.ViewCrossProjectOutliner,
		presentation.ViewTaskComposer:
		policy = PresentationChromePolicy{
			kind:     presentationChromeFullscreen,
			guidance: presentationGuidanceShell,
		}
	case presentation.ViewPriorityPicker,
		presentation.ViewParentSelector,
		presentation.ViewTagSelector,
		presentation.ViewOutlineErrors:
		policy = PresentationChromePolicy{
			kind:     presentationChromeTransient,
			guidance: presentationGuidanceOverlay,
		}
	default:
		return PresentationChromePolicy{}, false
	}
	return policy, policy.valid()
}
