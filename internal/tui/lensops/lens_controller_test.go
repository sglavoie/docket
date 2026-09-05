package lensops

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// TestLensControllerDrainsThePendingLaunchExactlyOnce pins the launch order the
// restore depends on: the pointer is handed back once and forgotten in the same
// step, so a restore that runs twice applies the lens once.
func TestLensControllerDrainsThePendingLaunchExactlyOnce(t *testing.T) {
	t.Parallel()

	saved := config.NamedLens{ID: "lens-1", Name: "Deep work"}
	controller := Controller{pendingLaunch: &saved}

	pending, ok := controller.TakePendingLaunch()
	if !ok || pending.ID != "lens-1" {
		t.Fatalf("TakePendingLaunch = %+v, %v; want the stored lens", pending, ok)
	}
	if controller.pendingLaunch != nil {
		t.Fatal("TakePendingLaunch must forget the pointer in the same step")
	}

	if _, ok := controller.TakePendingLaunch(); ok {
		t.Fatal("a drained pointer is absent, so a second restore applies nothing")
	}
}

// TestLensControllerReportsAbsenceRatherThanAZeroLens pins why the drain
// reports presence separately: an empty NamedLens is a lens ApplyNamedLens would
// try to honor, so "no pointer" must not be spelled as a zero record.
func TestLensControllerReportsAbsenceRatherThanAZeroLens(t *testing.T) {
	t.Parallel()

	var absent *Controller
	if _, ok := absent.TakePendingLaunch(); ok {
		t.Fatal("a nil cluster holds no pending launch lens")
	}

	var empty Controller
	if _, ok := empty.TakePendingLaunch(); ok {
		t.Fatal("an unset pointer reports absence, not a zero lens")
	}
}

// TestLensControllerMountsAndDismissesThePicker pins the overlay lifecycle the
// shell reads for HasActiveTextInput and Overlays.Active: the picker is mounted
// or it is not, and a nil cluster is neither.
func TestLensControllerMountsAndDismissesThePicker(t *testing.T) {
	t.Parallel()

	var absent *Controller
	if absent.Mounted() {
		t.Fatal("a nil cluster mounts no picker")
	}

	var controller Controller
	if controller.Mounted() {
		t.Fatal("a fresh cluster mounts no picker")
	}

	controller.Open(&lens.NamedLensPickerModel{})
	if !controller.Mounted() {
		t.Fatal("open mounts the picker")
	}

	controller.Close()
	if controller.Mounted() || controller.picker != nil {
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

	var controller Controller
	controller.WireOutliner(nil, func(string) error { return nil })

	view := outliner.NewOutlinerModel(nil, nil, nil)
	persisted := ""
	controller.WireOutliner(view, func(id string) error {
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
