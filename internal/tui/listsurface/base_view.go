package listsurface

import (
	"image/color"

	"github.com/sglavoie/tt/internal/tui/pulse"
)

// BaseView provides common functionality for TUI views.
// Embed this in view models to get standard dimension and error handling.
type BaseView struct {
	pulse.WorkspacePulseState
	width       int
	height      int
	Err         error
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
	v.Err = err
}

// ClearError clears the current error.
func (v *BaseView) ClearError() {
	v.Err = nil
}

// HasError returns true if there is an error.
func (v *BaseView) HasError() bool {
	return v.Err != nil
}

// Error returns the current error.
func (v *BaseView) Error() error {
	return v.Err
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
// CycleTab advances a tab index by delta with wrapping.
// delta is typically +1 (right) or -1 (left).
func CycleTab(current *int, count, delta int) {
	if count <= 1 {
		return
	}
	*current = ((*current+delta)%count + count) % count
}
