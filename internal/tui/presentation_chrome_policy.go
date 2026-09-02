package tui

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

type presentationChromePolicy struct {
	kind     presentationChromeKind
	guidance presentationGuidanceOwner
}

func (p presentationChromePolicy) workspacePulseEligible() bool {
	return p.kind == presentationChromeFullscreen
}

func (p presentationChromePolicy) valid() bool {
	switch p.kind {
	case presentationChromeFullscreen:
		return p.guidance == presentationGuidanceShell || p.guidance == presentationGuidanceSurface
	case presentationChromeTransient:
		return p.guidance == presentationGuidanceOverlay
	default:
		return false
	}
}

// chromePolicyForPresentation is exhaustive over allPresentations. New
// PresentationIDs must be classified here and in the policy test before they
// can silently inherit workspace chrome.
func chromePolicyForPresentation(view presentation.PresentationID) (presentationChromePolicy, bool) {
	var policy presentationChromePolicy
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
		policy = presentationChromePolicy{
			kind:     presentationChromeFullscreen,
			guidance: presentationGuidanceSurface,
		}
	case presentation.ViewCrossProjectOutliner,
		presentation.ViewTaskComposer:
		policy = presentationChromePolicy{
			kind:     presentationChromeFullscreen,
			guidance: presentationGuidanceShell,
		}
	case presentation.ViewPriorityPicker,
		presentation.ViewParentSelector,
		presentation.ViewTagSelector,
		presentation.ViewOutlineErrors:
		policy = presentationChromePolicy{
			kind:     presentationChromeTransient,
			guidance: presentationGuidanceOverlay,
		}
	default:
		return presentationChromePolicy{}, false
	}
	return policy, policy.valid()
}
