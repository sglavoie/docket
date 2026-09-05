// Package lensops owns the named-lens cluster: the picker overlay's state, the
// six operations a saved projection answers to, and the machine-local commits
// each of them publishes through.
//
// The behavior lives here rather than on the shell because a lens touches
// exactly three things — the Outliner whose axes it writes, the operation
// registry its rows are declared in, and the machine snapshot the definitions
// are committed into. Shell is that port. What stays on the shell is the
// mounting and teardown of the overlays a lens action nests above the picker,
// because those are the shell's own overlay stack and not the lens's.
package lensops

import (
	"errors"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// Shell is what the named-lens cluster needs from the shell that owns it.
type Shell interface {
	// Outliner is the view a saved projection is applied to and captured from.
	Outliner() *outliner.OutlinerModel

	// Registry and Executor are the descriptor authority and the invocation
	// pipeline the six lens operations register themselves with.
	Registry() *operation.OperationRegistry
	Executor() *operation.OperationExecutor

	// RegisterOperation registers one descriptor and its binding.
	RegisterOperation(operation.OperationDescriptor, operation.OperationBinding) error

	// Machine is the cached machine-local snapshot the lens definitions are read
	// from, or nil when no machine state is available. CommitMachine publishes a
	// validated replacement, choosing the durable or in-memory seam for itself.
	Machine() *config.MachineState
	CommitMachine(*config.MachineState) error
}

// Controller is the named-lens cluster's state: the picker overlay, and the
// lens a machine-local pointer asked this session to reopen with.
//
// The picker is a cluster of its own rather than part of the operation wiring
// because it sits beneath the contextual action flow instead of inside it: it
// is the surface an action sheet is opened *from*, so it stays mounted under the
// sheet, prompt, and confirmation a lens action nests above it, and every cancel
// path inside that stack lands back on the list.
//
// The shell travels in per call rather than being held, so the zero value is
// usable and nothing here outlives the shell that asked for it.
type Controller struct {
	// picker is the named-lens picker overlay, mounted over the current route.
	picker *lens.NamedLensPickerModel

	// pendingLaunch is the lens the machine-local active-lens pointer named when
	// this session was constructed, held here because the launch reset clears
	// that pointer before any message can read it. It is drained exactly once,
	// after the launch population lands, and forgotten either way.
	pendingLaunch *config.NamedLens
}

// errRuntimeUnavailable and errMachineUnavailable are the two refusals every
// machine-local read shares, so a caller sees the same sentence whichever side
// of the snapshot is missing.
var (
	errMachineUnavailable = errors.New("machine-local state is unavailable")
)

// SetPendingLaunch records the lens the machine-local pointer named at
// construction. It is written once, by the composition root.
func (c *Controller) SetPendingLaunch(saved *config.NamedLens) {
	if c == nil {
		return
	}
	c.pendingLaunch = saved
}

// TakePendingLaunch hands the launch lens back and forgets it in the same step,
// so a restore that runs twice applies it once. It reports absence rather than
// returning a zero record, because an empty NamedLens is a lens the grammar
// would try to honor.
func (c *Controller) TakePendingLaunch() (config.NamedLens, bool) {
	if c == nil || c.pendingLaunch == nil {
		return config.NamedLens{}, false
	}
	pending := *c.pendingLaunch
	c.pendingLaunch = nil
	return pending, true
}

// Picker is the mounted picker overlay, or nil.
func (c *Controller) Picker() *lens.NamedLensPickerModel {
	if c == nil {
		return nil
	}
	return c.picker
}

// Mounted reports whether the picker is on screen. Every level of it takes
// typed input, so this doubles as the has-active-text-input answer for the lens
// half of the overlay stack.
func (c *Controller) Mounted() bool {
	return c != nil && c.picker != nil
}

// Open mounts the picker.
func (c *Controller) Open(picker *lens.NamedLensPickerModel) {
	c.picker = picker
}

// Close dismisses the picker, so it does not outlive the route it was opened
// over. The overlays a lens action nests above it are the operation cluster's
// and are torn down beside this, never by it.
func (c *Controller) Close() {
	c.picker = nil
}

// WireOutliner injects the machine-local write that remembers which lens is
// live. It is one of the four seams an Outliner cannot build for itself, and
// the only one the lens cluster owns. The write travels in rather than being
// held here: binding it at wire time is what keeps a directly constructed model
// wired exactly like a launched one.
func (c *Controller) WireOutliner(view *outliner.OutlinerModel, persist func(id string) error) {
	if view == nil {
		return
	}
	view.SetPersistActiveLens(persist)
}
