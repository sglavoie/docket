package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// lensController is the shell's named-lens cluster: the picker overlay, the lens
// a machine-local pointer asked this session to reopen with, and the injection
// of the machine-local write that remembers which lens is live.
//
// The field is named lenses, not lens, because lens is an imported package name
// inside the shell — the same shadowing reason the notes cluster is noteIndex
// and the operation-wiring cluster is operations.
//
// The picker is a cluster of its own rather than part of operations because it
// sits beneath the contextual action flow instead of inside it: it is the
// surface an action sheet is opened *from*, so it stays mounted under the sheet,
// prompt, and confirmation a lens action nests above it, and every cancel path
// inside that stack lands back on the list.
//
// It owns the pointer, not the write. commitActiveLensPointer stays an AppModel
// method, because it commits machine state through the runtime snapshot the
// shell owns and a lens mutation opens no workspace transaction and records no
// undo entry. What lives here is the one Outliner seam that write is injected
// through, so the lens half of wireOutlinerOperationExecutor has a single home.
type lensController struct {
	// picker is the named-lens picker overlay, mounted over the current route.
	picker *lens.NamedLensPickerModel

	// pendingLaunch is the lens the machine-local active-lens pointer named when
	// this session was constructed, held here because the launch reset clears
	// that pointer before any message can read it. It is drained exactly once,
	// after the launch population lands, and forgotten either way.
	pendingLaunch *config.NamedLens
}

// wireOutliner injects the machine-local write that remembers which lens is
// live. It is one of the four seams an Outliner cannot build for itself, and the
// only one the lens cluster owns. The write travels in rather than being held
// here: it needs the runtime snapshot and the machine-commit path that live on
// AppModel, and binding it at wire time is what keeps a directly constructed
// model wired exactly like a launched one.
func (c *lensController) wireOutliner(view *outliner.OutlinerModel, persist func(id string) error) {
	if view == nil {
		return
	}
	view.SetPersistActiveLens(persist)
}

// mounted reports whether the picker is on screen. Every level of it takes typed
// input, so this doubles as the has-active-text-input answer for the lens half
// of the overlay stack.
func (c *lensController) mounted() bool {
	return c != nil && c.picker != nil
}

// open mounts the picker.
func (c *lensController) open(picker *lens.NamedLensPickerModel) {
	c.picker = picker
}

// close dismisses the picker, so it does not outlive the route it was opened
// over. The overlays a lens action nests above it are the operation cluster's
// and are torn down beside this, never by it.
func (c *lensController) close() {
	c.picker = nil
}

// takePendingLaunch hands the launch lens back and forgets it in the same step,
// so a restore that runs twice applies it once. It reports absence rather than
// returning a zero record, because an empty NamedLens is a lens the grammar
// would try to honor.
func (c *lensController) takePendingLaunch() (config.NamedLens, bool) {
	if c == nil || c.pendingLaunch == nil {
		return config.NamedLens{}, false
	}
	pending := *c.pendingLaunch
	c.pendingLaunch = nil
	return pending, true
}
