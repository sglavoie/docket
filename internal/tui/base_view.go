package tui

import (
	"image/color"

	"github.com/sglavoie/tt/internal/config"
)

// ViewBundle groups the domain runtime and application-layer coordinators shared
// by secondary views. It deliberately carries no Config facade.
type ViewBundle struct {
	// runtime is the startup-loaded snapshot of the owned configuration domains.
	// Machine-local layout consumers (project-view state, outliner/tree columns,
	// dashboard split/info-panel, cross-outliner state, tab order) read it via
	// runtime.Machine() and persist through the machine-local committer instead of
	// the config.Config facade. A nil runtime (isolated render tests) is tolerated
	// by the config accessors, which default to an empty machine snapshot.
	runtime *config.RuntimeState

	// bulk routes multiselect mutations through the application layer. The shell
	// wires it via SetBulkCoordinator when the view is shown; a zero value (nil
	// mutator) means the view was built in isolation (e.g. a render-only test) and
	// its bulk handlers are not exercised.
	bulk BulkCoordinator

	// taskItem routes single-task outliner mutations through the application
	// layer, parallel to bulk. The shell wires it via SetTaskItemCoordinator; a
	// zero value (nil mutator) means the view was built in isolation. No caller
	// routes through it yet — it is the migration seam for the cross-project
	// single-task raw-save family.
	taskItem TaskItemCoordinator

	// link routes task-link mutations (dependency/related edge removal) through
	// the application layer. The shell wires it via SetLinkCoordinator; a zero
	// value (nil mutator) means the view was built in isolation. The linked-task
	// selector unlink flow routes through it.
	link LinkCoordinator
}

// SetBulkCoordinator injects the application-layer bulk coordinator. It is
// promoted to every view that embeds ViewBundle, so the shell wires all bulk
// views from a single site (showView/replaceView) through the bulkWireable
// interface.
func (b *ViewBundle) SetBulkCoordinator(c BulkCoordinator) { b.bulk = c }

// SetTaskItemCoordinator injects the application-layer single-task coordinator,
// promoted to every view that embeds ViewBundle so the shell wires it from the
// same showView/replaceView site as the bulk coordinator (via taskItemWireable).
func (b *ViewBundle) SetTaskItemCoordinator(c TaskItemCoordinator) { b.taskItem = c }

// SetLinkCoordinator injects the application-layer link coordinator, promoted to
// every view that embeds ViewBundle so the shell wires it from the same
// showView/replaceView site as the other coordinators (via linkWireable).
func (b *ViewBundle) SetLinkCoordinator(c LinkCoordinator) { b.link = c }

// BaseView provides common functionality for TUI views.
// Embed this in view models to get standard dimension and error handling.
type BaseView struct {
	width       int
	height      int
	err         error
	accentColor color.Color
}

// SetSize updates the view dimensions.
func (v *BaseView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

// SetAccentColor stores the view's contextual accent color.
// Called by the app shell when the view is pushed onto the stack.
func (v *BaseView) SetAccentColor(c color.Color) {
	v.accentColor = c
}

// AccentColor returns the view's contextual accent color, or nil if unset.
func (v *BaseView) AccentColor() color.Color {
	return v.accentColor
}

// Width returns the current width.
func (v *BaseView) Width() int {
	return v.width
}

// Height returns the current height.
func (v *BaseView) Height() int {
	return v.height
}

// SetError sets the current error.
func (v *BaseView) SetError(err error) {
	v.err = err
}

// ClearError clears the current error.
func (v *BaseView) ClearError() {
	v.err = nil
}

// HasError returns true if there is an error.
func (v *BaseView) HasError() bool {
	return v.err != nil
}

// Error returns the current error.
func (v *BaseView) Error() error {
	return v.err
}

// ShouldCaptureEsc returns false by default.
func (v *BaseView) ShouldCaptureEsc() bool {
	return false
}

// HasTextInput returns false by default.
func (v *BaseView) HasTextInput() bool {
	return false
}

// IsFullHeight returns false by default.
func (v *BaseView) IsFullHeight() bool {
	return false
}

// ContentWidth returns width minus standard padding (2 on each side).
func (v *BaseView) ContentWidth() int {
	return max(v.width-4, 0)
}

// ContentHeight returns height minus standard padding (1 on each side).
func (v *BaseView) ContentHeight() int {
	return max(v.height-2, 0)
}

// WrapCursor wraps a cursor position within list bounds.
// delta is typically 1 (down) or -1 (up).
func WrapCursor(current, listLen, delta int) int {
	if listLen == 0 {
		return 0
	}
	result := current + delta
	if result < 0 {
		return listLen - 1
	}
	if result >= listLen {
		return 0
	}
	return result
}

// CycleTab advances a tab index by delta with wrapping.
// delta is typically +1 (right) or -1 (left).
func CycleTab(current *int, count, delta int) {
	if count <= 1 {
		return
	}
	*current = ((*current+delta)%count + count) % count
}
