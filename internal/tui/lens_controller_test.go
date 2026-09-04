package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// TestLensClusterHasOneHome pins the named-lens cluster to its single named
// AppModel field. The picker and the launch pointer are the two pieces of lens
// state the shell holds at all; a second home is how a drained pointer would
// come to be applied twice.
func TestLensClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"lensPicker":        true,
		"pendingLaunchLens": true,
	}
	assertAppModelClusterHasOneHome(t, "lenses", clustered)
}

// TestLensControllerDrainsThePendingLaunchExactlyOnce pins the launch order the
// restore depends on: the pointer is handed back once and forgotten in the same
// step, so a restore that runs twice applies the lens once.
func TestLensControllerDrainsThePendingLaunchExactlyOnce(t *testing.T) {
	t.Parallel()

	saved := config.NamedLens{ID: "lens-1", Name: "Deep work"}
	controller := lensController{pendingLaunch: &saved}

	pending, ok := controller.takePendingLaunch()
	if !ok || pending.ID != "lens-1" {
		t.Fatalf("takePendingLaunch = %+v, %v; want the stored lens", pending, ok)
	}
	if controller.pendingLaunch != nil {
		t.Fatal("takePendingLaunch must forget the pointer in the same step")
	}

	if _, ok := controller.takePendingLaunch(); ok {
		t.Fatal("a drained pointer is absent, so a second restore applies nothing")
	}
}

// TestLensControllerReportsAbsenceRatherThanAZeroLens pins why the drain
// reports presence separately: an empty NamedLens is a lens ApplyNamedLens would
// try to honor, so "no pointer" must not be spelled as a zero record.
func TestLensControllerReportsAbsenceRatherThanAZeroLens(t *testing.T) {
	t.Parallel()

	var absent *lensController
	if _, ok := absent.takePendingLaunch(); ok {
		t.Fatal("a nil cluster holds no pending launch lens")
	}

	var empty lensController
	if _, ok := empty.takePendingLaunch(); ok {
		t.Fatal("an unset pointer reports absence, not a zero lens")
	}
}

// TestLensControllerMountsAndDismissesThePicker pins the overlay lifecycle the
// shell reads for HasActiveTextInput and Overlays.Active: the picker is mounted
// or it is not, and a nil cluster is neither.
func TestLensControllerMountsAndDismissesThePicker(t *testing.T) {
	t.Parallel()

	var absent *lensController
	if absent.mounted() {
		t.Fatal("a nil cluster mounts no picker")
	}

	var controller lensController
	if controller.mounted() {
		t.Fatal("a fresh cluster mounts no picker")
	}

	controller.open(&lens.NamedLensPickerModel{})
	if !controller.mounted() {
		t.Fatal("open mounts the picker")
	}

	controller.close()
	if controller.mounted() || controller.picker != nil {
		t.Fatal("close dismisses the picker, so it does not outlive its route")
	}
}

// TestLensControllerWiresThePersistSeamOntoTheOutliner pins the one seam the
// cluster owns. The machine-local write travels in rather than being held here,
// because it needs the runtime snapshot and the commit path that live on
// AppModel; binding it at wire time is what keeps a directly constructed model
// wired exactly like a launched one.
func TestLensControllerWiresThePersistSeamOntoTheOutliner(t *testing.T) {
	t.Parallel()

	var controller lensController
	controller.wireOutliner(nil, func(string) error { return nil })

	view := outliner.NewOutlinerModel(nil, nil, nil)
	persisted := ""
	controller.wireOutliner(view, func(id string) error {
		persisted = id
		return nil
	})
	if view.Lens.Persist == nil {
		t.Fatal("wireOutliner must install the machine-local write on the Outliner")
	}
	if err := view.Lens.Persist("lens-1"); err != nil {
		t.Fatalf("persist active lens: %v", err)
	}
	if persisted != "lens-1" {
		t.Fatalf("persisted = %q, want the injected write to have received the lens ID", persisted)
	}
}
