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
