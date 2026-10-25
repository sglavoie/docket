package route

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/presentation"
)

// allPresentations returns the contiguous renderer inventory. Only the
// exhaustiveness tests below walk it: production code always holds a
// PresentationID it was handed rather than one it enumerated.
func allPresentations() []presentation.PresentationID {
	presentations := make([]presentation.PresentationID, 0, int(presentation.ViewLastMessage))
	for id := presentation.ViewProjectCreator; id <= presentation.ViewLastMessage; id++ {
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

// TestHostGuidanceStandsBeneathNamesExactlyTheTransientPresentations holds the
// predicate the shell footer consults to the classification it is derived from.
// A transient presentation is composited over a host that still fills the
// footer; a fullscreen one replaced that host, so the footer beneath it is its
// own even where its acts are drawn in its content.
func TestHostGuidanceStandsBeneathNamesExactlyTheTransientPresentations(t *testing.T) {
	for _, view := range allPresentations() {
		policy, ok := ChromePolicyForPresentation(view)
		if !ok {
			t.Fatalf("presentation %s has no chrome policy", view)
		}
		want := policy.kind == presentationChromeTransient
		if got := policy.HostGuidanceStandsBeneath(); got != want {
			t.Errorf("%s HostGuidanceStandsBeneath() = %t, want %t", view, got, want)
		}
		// The two questions the footer and the pulse ask are one classification
		// read twice, so a presentation may never answer both the same way.
		if policy.HostGuidanceStandsBeneath() == policy.WorkspacePulseEligible() {
			t.Errorf("%s claims a host beneath it and the workspace pulse at once", view)
		}
	}

	// The zero policy is what an unclassified presentation would carry. It reads
	// as fullscreen, so the reading cannot be mistaken for permission to blank:
	// the shell defaults an unknown presentation to publishing by asking whether
	// the policy resolved at all, never by asking the zero value.
	if (PresentationChromePolicy{}).HostGuidanceStandsBeneath() {
		t.Error("the zero chrome policy claims a host stands beneath it")
	}
}

// TestActiveGuidanceOwnerSuppressionIsAClosedTable states the whole table rather
// than the cases the product reaches today, so an owner added next to the wrong
// branch is a red test rather than a footer that silently keeps speaking under a
// new overlay.
func TestActiveGuidanceOwnerSuppressionIsAClosedTable(t *testing.T) {
	cases := map[ActiveGuidanceOwner]struct {
		suppresses bool
		name       string
	}{
		GuidanceOwnerShell:        {false, "shell"},
		GuidanceOwnerPendingChord: {false, "pending chord"},
		GuidanceOwnerConfirmation: {true, "confirmation"},
		GuidanceOwnerModal:        {true, "modal"},
		GuidanceOwnerPresentation: {true, "presentation"},
	}

	for owner, want := range cases {
		if got := owner.SuppressesShellGuidance(); got != want.suppresses {
			t.Errorf("%s SuppressesShellGuidance() = %t, want %t", want.name, got, want.suppresses)
		}
	}

	// The zero value is the shell, so a resolver that forgot to write an owner
	// publishes rather than blanks: a footer that may be the only instructions on
	// screen is never emptied by omission.
	var unset ActiveGuidanceOwner
	if unset != GuidanceOwnerShell || unset.SuppressesShellGuidance() {
		t.Error("the zero ActiveGuidanceOwner is not the publishing shell")
	}

	// Every owner in the table is distinct, which is what lets the caller switch
	// on one value instead of asking each question again.
	seen := make(map[ActiveGuidanceOwner]bool, len(cases))
	for owner := range cases {
		if seen[owner] {
			t.Fatalf("two owners share the value %d", owner)
		}
		seen[owner] = true
	}
	if len(seen) != 5 {
		t.Fatalf("the ownership vocabulary has %d distinct values; the table states 5", len(seen))
	}
}

func TestPresentationGuidanceReachabilityNamesShellAndSurfaceOwners(t *testing.T) {
	// A shell-owned presentation publishes its keys through
	// ContextualGuidanceProvider and draws no footer of its own, so the frame
	// carries one divider and one set of hints.
	wantShell := map[presentation.PresentationID]bool{
		presentation.ViewCrossProjectOutliner: true,
		presentation.ViewTaskComposer:         true,
		presentation.ViewProjectCreator:       true,
		presentation.ViewProjectEditor:        true,
		presentation.ViewTaskComments:         true,
		presentation.ViewTaskAttachments:      true,
		presentation.ViewTextEditor:           true,
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
