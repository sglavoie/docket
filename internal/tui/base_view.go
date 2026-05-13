package tui

import (
	"image/color"

	"github.com/sglavoie/tt/internal/config"
)

// ViewBundle groups the config and config manager fields shared by all views.
// Embed this anonymously so that m.config and m.configManager work unchanged.
type ViewBundle struct {
	config        *config.Config
	configManager *config.Manager
}

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

// ToggleSection flips a section's collapsed state.
func ToggleSection[K comparable](collapsed map[K]bool, key K) {
	collapsed[key] = !collapsed[key]
}
