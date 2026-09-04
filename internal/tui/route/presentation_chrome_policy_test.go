package route

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

// allPresentations returns the contiguous renderer inventory. Only the
// exhaustiveness tests below walk it: production code always holds a
// PresentationID it was handed rather than one it enumerated.
func allPresentations() []presentation.PresentationID {
	presentations := make([]presentation.PresentationID, 0, int(presentation.ViewOutlineErrors))
	for id := presentation.ViewProjectCreator; id <= presentation.ViewOutlineErrors; id++ {
		presentations = append(presentations, id)
	}
	return presentations
}

func TestPresentationChromePolicyClassifiesEveryPresentation(t *testing.T) {
	transient := map[presentation.PresentationID]bool{
		presentation.ViewPriorityPicker: true,
		presentation.ViewParentSelector: true,
		presentation.ViewTagSelector:    true,
		presentation.ViewOutlineErrors:  true,
	}

	for _, view := range allPresentations() {
		policy, ok := ChromePolicyForPresentation(view)
		if !ok {
			t.Fatalf("presentation %s has no chrome policy", view)
		}
		wantPulse := !transient[view]
		if got := policy.WorkspacePulseEligible(); got != wantPulse {
			t.Errorf("%s pulse eligibility = %t, want %t", view, got, wantPulse)
		}
		switch policy.guidance {
		case presentationGuidanceShell, presentationGuidanceSurface, presentationGuidanceOverlay:
		default:
			t.Errorf("%s has no shell, surface, or overlay guidance owner", view)
		}
		if transient[view] && policy.guidance != presentationGuidanceOverlay {
			t.Errorf("%s guidance owner = %v, want overlay", view, policy.guidance)
		}
	}

	if _, ok := ChromePolicyForPresentation(presentation.ViewUnknown); ok {
		t.Fatal("ViewUnknown must not receive an implicit chrome policy")
	}
}

func TestPresentationGuidanceReachabilityNamesShellAndSurfaceOwners(t *testing.T) {
	wantShell := map[presentation.PresentationID]bool{
		presentation.ViewCrossProjectOutliner: true,
		presentation.ViewTaskComposer:         true,
	}
	for _, view := range allPresentations() {
		policy, ok := ChromePolicyForPresentation(view)
		if !ok || policy.kind != presentationChromeFullscreen {
			continue
		}
		want := presentationGuidanceSurface
		if wantShell[view] {
			want = presentationGuidanceShell
		}
		if policy.guidance != want {
			t.Errorf("%s guidance owner = %v, want %v", view, policy.guidance, want)
		}
	}
}
