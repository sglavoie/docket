package listsurface

import (
	"image/color"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
)

// BaseView provides common functionality for TUI views.
// Embed this in view models to get standard dimension and error handling.
type BaseView struct {
	pulse.WorkspacePulseState
	width       int
	height      int
	shellFrame  chrome.ShellFrameMetrics
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

// SetShellFrameMetrics records what the shell's footer costs this frame. It is
// deliberately not SetSize: a message arriving and expiring must not re-anchor a
// scroll window, so the size a view was given stands and only the budget below
// moves. Every embedder gets the method and so satisfies route.ShellFrameAware;
// a view that never reads FrameHeight simply keeps behaving as it did.
func (v *BaseView) SetShellFrameMetrics(metrics chrome.ShellFrameMetrics) {
	v.shellFrame = metrics
}

// ShellFrameMetrics is what was last recorded, for a view composing its own
// budget out of the parts rather than taking FrameHeight whole.
func (v *BaseView) ShellFrameMetrics() chrome.ShellFrameMetrics {
	return v.shellFrame
}

// FrameHeight is the rows this view may actually draw: its height less whatever
// the footer costs beyond the two rows the shell sized it against. A view that
// plans a layout against Height instead will have its last row cropped by the
// shell exactly when the footer has something extra to say, which is exactly
// when the row it loses is the one naming the way out.
func (v *BaseView) FrameHeight() int {
	return max(v.height-v.shellFrame.ExcessRows(), 0)
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
