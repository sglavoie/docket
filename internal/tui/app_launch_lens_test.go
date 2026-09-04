package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// launchLensApp pairs a bare AppModel with a populated cross-project Outliner,
// which is the state the drain point actually runs in: the launch reset has
// completed and the population has just been adopted.
func launchLensApp(t *testing.T) *AppModel {
	t.Helper()
	app := newTestAppModel()
	app.crossOutliner = scopeTestOutliner(
		lensScopeProject(lensScopeAlphaID),
		lensScopeProject(lensScopeBetaID),
	)
	return app
}

// The pointer read at construction is applied once and forgotten, so a second
// population load — a refresh, a scope change — cannot reinstall a lens the user
// has since dismissed.
func TestPendingLaunchLensDrainsExactlyOnce(t *testing.T) {
	app := launchLensApp(t)
	lens := scopedWaitingLens(lensTestID(1), "Beta Waiting", lensScopeBetaID)
	app.pendingLaunchLens = &lens

	app.restorePendingLaunchLens()
	if app.pendingLaunchLens != nil {
		t.Fatal("the launch lens was left armed after being drained")
	}
	if app.crossOutliner.scope.projectID != lensScopeBetaID {
		t.Fatalf("scope after the launch drain = %q, want the lens's own %q",
			app.crossOutliner.scope.projectID, lensScopeBetaID)
	}
	if app.crossOutliner.lens.active == nil {
		t.Fatal("the drain installed no lens frame")
	}

	// Whatever the user does next owns the view; a second drain is a no-op.
	app.crossOutliner.clearActiveLensWithoutRestore()
	app.restorePendingLaunchLens()
	if app.crossOutliner.lens.active != nil {
		t.Fatal("a second drain reinstalled a lens the pointer no longer names")
	}
}

// A lens the pointer names but that can no longer be opened fails closed to no
// lens and says nothing: the pointer is a hint the user last expressed on this
// machine, not a record they are owed an error about.
func TestPendingLaunchLensRefusalIsSilent(t *testing.T) {
	app := launchLensApp(t)
	missing := scopedWaitingLens(lensTestID(1), "Ghost Desk", lensScopeAlphaID)
	missing.Scope = "b1c2d3e4-0000-4aaa-8bbb-00000000dead"
	app.pendingLaunchLens = &missing

	if cmd := app.restorePendingLaunchLens(); cmd != nil {
		t.Fatal("a refused launch lens produced a command, so it would surface an error")
	}
	if app.pendingLaunchLens != nil {
		t.Fatal("a refused launch lens stayed armed for the next population load")
	}
	if app.crossOutliner.lens.active != nil {
		t.Fatal("a lens scoped to a project the workspace does not admit was installed anyway")
	}
	if app.crossOutliner.scope.projectID != "" {
		t.Fatalf("a refused lens moved the scope to %q", app.crossOutliner.scope.projectID)
	}
}

// The drain is nil-safe on both halves, because the launch it runs in cannot
// promise either one: a session with no pointer holds no lens, and the arm runs
// before the cross-project Outliner is guaranteed to exist.
func TestPendingLaunchLensDrainToleratesAnAbsentLensOrOutliner(t *testing.T) {
	app := launchLensApp(t)
	if cmd := app.restorePendingLaunchLens(); cmd != nil {
		t.Fatal("draining an unarmed launch lens produced a command")
	}

	lens := scopedWaitingLens(lensTestID(1), "Beta Waiting", lensScopeBetaID)
	app.pendingLaunchLens = &lens
	app.crossOutliner = nil
	if cmd := app.restorePendingLaunchLens(); cmd != nil {
		t.Fatal("draining without an Outliner produced a command")
	}
	if app.pendingLaunchLens != nil {
		t.Fatal("the lens survived a drain that had nowhere to install it")
	}
}

// ActiveNamedLens is what NewAppModel reads, and it validates shape alone: a
// pointer naming a lens another machine deleted resolves to nothing rather than
// failing the load, which is what leaves the refusal to the drain.
func TestActiveNamedLensPointerIsCheckedForShapeAlone(t *testing.T) {
	machine := config.NewMachineState()
	lenses := machine.NamedLenses
	if len(lenses) == 0 {
		t.Fatal("a fresh machine seeds no starter lenses")
	}
	machine.ActiveLensID = "3f4c2a91-7d58-4b0e-9c16-2ae5d81f6b34"
	if err := machine.Validate(); err != nil {
		t.Fatalf("a dangling pointer failed load validation: %v", err)
	}
	if lens := machine.ActiveNamedLens(); lens != nil {
		t.Fatalf("a dangling pointer resolved to %q", lens.Name)
	}

	machine.ActiveLensID = lenses[0].ID
	resolved := machine.ActiveNamedLens()
	if resolved == nil || resolved.ID != lenses[0].ID {
		t.Fatal("a pointer naming a saved lens did not resolve to it")
	}
}
