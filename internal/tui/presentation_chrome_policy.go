package tui

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
func chromePolicyForPresentation(view PresentationID) (presentationChromePolicy, bool) {
	var policy presentationChromePolicy
	switch view {
	case ViewProjectCreator,
		ViewProjectEditor,
		ViewHelp,
		ViewTaskComments,
		ViewTextEditor,
		ViewTaskAttachments,
		ViewNotes,
		ViewGlobalNotes,
		ViewTaskChecklist,
		ViewTaskForm,
		ViewConflictResolution,
		ViewProjectHub:
		policy = presentationChromePolicy{
			kind:     presentationChromeFullscreen,
			guidance: presentationGuidanceSurface,
		}
	case ViewCrossProjectOutliner,
		ViewTaskComposer:
		policy = presentationChromePolicy{
			kind:     presentationChromeFullscreen,
			guidance: presentationGuidanceShell,
		}
	case ViewTagFilterSelector,
		ViewPriorityPicker,
		ViewParentSelector,
		ViewTagSelector,
		ViewBulkTagSelector,
		ViewProjectSelector:
		policy = presentationChromePolicy{
			kind:     presentationChromeTransient,
			guidance: presentationGuidanceOverlay,
		}
	default:
		return presentationChromePolicy{}, false
	}
	return policy, policy.valid()
}
