package route

import "github.com/sglavoie/tt/internal/tui/presentation"

// PresentationChromeKind identifies which layer owns persistent presentation
// chrome. Full-screen presentations own the workspace pulse and their guidance;
// transient selectors and pickers own only their local interaction guidance,
// leaving persistent workspace chrome to the host beneath them.
type PresentationChromeKind uint8

const (
	PresentationChromeFullscreen PresentationChromeKind = iota
	PresentationChromeTransient
)

// PresentationGuidanceOwner identifies the one layer responsible for guidance
// whenever a production presentation is reachable.
type PresentationGuidanceOwner uint8

const (
	_ PresentationGuidanceOwner = iota
	PresentationGuidanceShell
	PresentationGuidanceSurface
	PresentationGuidanceOverlay
)

type PresentationChromePolicy struct {
	kind     PresentationChromeKind
	guidance PresentationGuidanceOwner
}

func (p PresentationChromePolicy) WorkspacePulseEligible() bool {
	return p.kind == PresentationChromeFullscreen
}

func (p PresentationChromePolicy) valid() bool {
	switch p.kind {
	case PresentationChromeFullscreen:
		return p.guidance == PresentationGuidanceShell || p.guidance == PresentationGuidanceSurface
	case PresentationChromeTransient:
		return p.guidance == PresentationGuidanceOverlay
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
		presentation.ViewTaskComments,
		presentation.ViewTextEditor,
		presentation.ViewTaskAttachments,
		presentation.ViewNotes,
		presentation.ViewGlobalNotes,
		presentation.ViewProjectHub:
		policy = PresentationChromePolicy{
			kind:     PresentationChromeFullscreen,
			guidance: PresentationGuidanceSurface,
		}
	case presentation.ViewCrossProjectOutliner,
		presentation.ViewTaskComposer:
		policy = PresentationChromePolicy{
			kind:     PresentationChromeFullscreen,
			guidance: PresentationGuidanceShell,
		}
	case presentation.ViewPriorityPicker,
		presentation.ViewParentSelector,
		presentation.ViewTagSelector,
		presentation.ViewOutlineErrors:
		policy = PresentationChromePolicy{
			kind:     PresentationChromeTransient,
			guidance: PresentationGuidanceOverlay,
		}
	default:
		return PresentationChromePolicy{}, false
	}
	return policy, policy.valid()
}
