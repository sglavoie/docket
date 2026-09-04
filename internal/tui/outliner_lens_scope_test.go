package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// A lens scope names a project by ID, and NamedLens.Validate holds that ID to
// the canonical UUID spelling, so a scoped fixture's projects have to be minted
// the same way a real catalog mints them.
const (
	lensScopeAlphaID = "b1c2d3e4-0000-4aaa-8bbb-000000000001"
	lensScopeBetaID  = "b1c2d3e4-0000-4aaa-8bbb-000000000002"
)

// lensScopeProject builds one admissible workspace project carrying a single
// waiting task, which is what every lens in this file narrows to.
func lensScopeProject(id string) *models.Project {
	task := primaryRowsTask(id+"-waiting", models.StateWaiting)
	task.Title = "Needle in " + id
	return primaryRowsProject(id, task)
}

// scopedWaitingLens is waitingLens with the scope axis owned.
func scopedWaitingLens(id, name, scope string) config.NamedLens {
	lens := waitingLens(id, name, "")
	lens.Scope = scope
	return lens
}

// A lens that owns the scope axis enters it, and the baseline one Esc returns to
// is the scope the user was in rather than the one the lens installed — which is
// only true because ApplyNamedLens freezes that baseline before the push.
func TestScopedLensEntersItsProjectAndEscRestoresThePreLensScope(t *testing.T) {
	alpha := lensScopeProject(lensScopeAlphaID)
	beta := lensScopeProject(lensScopeBetaID)
	outliner := scopeTestOutliner(alpha, beta)

	lens := scopedWaitingLens(lensTestID(1), "Beta Waiting", lensScopeBetaID)
	if err := lens.Validate(); err != nil {
		t.Fatalf("scoped definition invalid: %v", err)
	}
	if cmd := applyLensOrFatal(t, outliner, lens); cmd != nil {
		t.Fatal("a scoped application must need no population load")
	}
	if outliner.pendingLens != nil {
		t.Fatal("a scoped application armed a load the scope makes unnecessary")
	}
	if outliner.scopedProjectID != lensScopeBetaID {
		t.Fatalf("scope after applying = %q, want the lens's own %q", outliner.scopedProjectID, lensScopeBetaID)
	}
	// The axis was written through pushProjectScope, so the frame one Esc pops to
	// exists exactly as it would after any other scope push.
	if len(outliner.scopeStack) != 1 {
		t.Fatalf("scope stack = %d frames, want the single base frame a push captures", len(outliner.scopeStack))
	}
	if outliner.lensModified() {
		t.Fatal("a lens read as modified in the very scope it named")
	}

	if !listsurface.RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("Esc left the lens live")
	}
	if outliner.scopedProjectID != "" {
		t.Fatalf("restored scope = %q, want the unscoped view the lens replaced", outliner.scopedProjectID)
	}
}

// An unscoped lens declines the axis, so it leaves the scope alone and cannot
// read as diverged from it — until the user moves the scope themselves, which is
// the one carve-out from lensModified's "navigating changes nothing" rule.
func TestUnscopedLensLeavesTheScopeAloneAndMovingItMarksTheLensModified(t *testing.T) {
	alpha := lensScopeProject(lensScopeAlphaID)
	beta := lensScopeProject(lensScopeBetaID)
	outliner := scopeTestOutliner(alpha, beta)
	outliner.pushProjectScope(lensScopeAlphaID)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(2), "Waiting", ""))
	if outliner.scopedProjectID != lensScopeAlphaID {
		t.Fatalf("an unscoped lens moved the scope to %q", outliner.scopedProjectID)
	}
	if outliner.lensModified() {
		t.Fatal("an unowned scope axis reported a divergence from the scope it left alone")
	}

	outliner.pushProjectScope(lensScopeBetaID)
	if !outliner.lensModified() {
		t.Fatal("moving the scope under a live lens must mark it modified")
	}
	if !strings.HasSuffix(outliner.activeLensLabel(), "*") {
		t.Fatalf("lens label = %q, want the modified marker", outliner.activeLensLabel())
	}
}

// The scope is resolved against the base task workspace alone, before any axis
// is written. Archived, parked, and deleted each refuse in their own words and
// none of them widens admission, so a refusal is indistinguishable from never
// having asked.
func TestScopedLensRefusesAProjectTheBaseWorkspaceDoesNotAdmit(t *testing.T) {
	archived := lensScopeProject(lensScopeAlphaID)
	archived.Archived = true
	parked := lensScopeProject(lensScopeBetaID)
	parked.Tags = []string{models.ParkedProjectTag}
	live := lensScopeProject("c1c2d3e4-0000-4aaa-8bbb-000000000003")

	cases := map[string]struct {
		scope string
		want  string
	}{
		"archived": {scope: lensScopeAlphaID, want: "which is archived"},
		"parked":   {scope: lensScopeBetaID, want: "which is parked"},
		"deleted":  {scope: "d1c2d3e4-0000-4aaa-8bbb-000000000004", want: "no longer exists"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			outliner := scopeTestOutliner(archived, parked, live)
			before := outliner.captureLensState().fingerprint()

			cmd, err := outliner.ApplyNamedLens(scopedWaitingLens(lensTestID(3), "Elsewhere", testCase.scope))
			if err == nil {
				t.Fatal("the application accepted a scope the base workspace does not admit")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("refusal = %q, want it to say %q", err, testCase.want)
			}
			if cmd != nil {
				t.Fatal("a refusal started a load")
			}
			if outliner.hasActiveLens() || outliner.scopedProjectID != "" {
				t.Fatal("a refusal wrote an axis")
			}
			if outliner.captureLensState().fingerprint() != before {
				t.Fatal("a refusal moved a lens-owned axis")
			}
		})
	}
}

// A parked focus is an exception to the base workspace rather than a scope the
// base Outliner can be asked for, so saving from inside one records no scope. The
// alternative would mint a definition every later application has to refuse.
func TestCapturingAScopeSkipsAParkedFocusAndTheSingleProjectView(t *testing.T) {
	parked := lensScopeProject(lensScopeBetaID)
	parked.Tags = []string{models.ParkedProjectTag}
	outliner := scopeTestOutliner(lensScopeProject(lensScopeAlphaID), parked)
	outliner.scopedProjectID = lensScopeBetaID
	outliner.parkedFocusProjectID = lensScopeBetaID
	if scope := outliner.captureLensState().scopeProjectID; scope != "" {
		t.Fatalf("captured scope = %q, want none for a parked focus", scope)
	}

	outliner.scopedProjectID = lensScopeAlphaID
	outliner.parkedFocusProjectID = ""
	if scope := outliner.captureLensState().scopeProjectID; scope != lensScopeAlphaID {
		t.Fatalf("captured scope = %q, want the admissible %q", scope, lensScopeAlphaID)
	}

	single := NewOutlinerModel(lensScopeProject(lensScopeAlphaID), nil, nil)
	if scope := single.captureLensState().scopeProjectID; scope != "" {
		t.Fatalf("the single-project Outliner captured a scope %q", scope)
	}
}

// The active-lens pointer is written where a lens becomes live and cleared where
// one stops being live, through the seam the shell injects. It is machine-local
// and is a resume hint, so the commit rides publishLens rather than the ask.
func TestPublishingALensRecordsTheActivePointerAndDismissalClearsIt(t *testing.T) {
	app := lensOperationApp(t)
	app.wireOutlinerOperationExecutor(app.crossOutliner)
	outliner := app.crossOutliner

	lens := waitingLens(lensTestID(4), "Waiting", "")
	applyLensOrFatal(t, outliner, lens)
	if got := app.runtime.Machine().ActiveLensID; got != lens.ID {
		t.Fatalf("active lens pointer = %q, want the published %q", got, lens.ID)
	}

	outliner.restoreLensBaseline()
	if got := app.runtime.Machine().ActiveLensID; got != "" {
		t.Fatalf("restoring the baseline left the pointer at %q", got)
	}

	applyLensOrFatal(t, outliner, lens)
	outliner.resetToGlobalAll()
	if got := app.runtime.Machine().ActiveLensID; got != "" {
		t.Fatalf("the global reset left the pointer at %q", got)
	}
}

// A failed pointer commit is a lost hint, never a lost projection: the lens is
// already live by the time the write runs, so the failure is reported and the
// application stands.
func TestAFailedActiveLensPointerCommitLeavesTheLensApplied(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.persistActiveLens = func(string) error { return errors.New("machine-local state is unavailable") }

	lens := waitingLens(lensTestID(5), "Waiting", "")
	applyLensOrFatal(t, outliner, lens)
	if !outliner.hasActiveLens() {
		t.Fatal("a failed pointer commit abandoned the application it only annotates")
	}
	if !strings.Contains(outliner.StatusMessage, "Could not remember the active lens") {
		t.Fatalf("status = %q, want the degraded pointer warning", outliner.StatusMessage)
	}
}
